package common

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto"
	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/gorilla/websocket"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/internal/js/modules/k6/browser/log"
)

const (
	BrowserStateOpen int64 = iota
	BrowserStateClosed
)

// Browser stores a Browser context.
type Browser struct {
	// These are internal contexts which control the lifecycle of the connection
	// and eventLoop. It is shutdown when browser.close() is called.
	browserCtx      context.Context
	browserCancelFn context.CancelFunc

	vuCtx         context.Context
	vuCtxCancelFn context.CancelFunc

	state int64

	browserProc *BrowserProcess
	browserOpts *BrowserOptions

	// Connection to the browser to talk CDP protocol.
	// A *Connection is saved to this field, see: connect().
	conn connection

	// This mutex is only needed in an edge case where we have multiple
	// instances of k6 connecting to the same chrome instance. In this
	// case when a page is created by the first k6 instance, the second
	// instance of k6 will also receive an onAttachedToTarget event. When
	// this occurs there's a small chance that at the same time a new
	// context is being created by the second k6 instance. So the read
	// occurs in getDefaultBrowserContextOrMatchedID which is called by
	// onAttachedToTarget, and the write in NewContext. This mutex protects
	// the read/write race condition for this one case.
	contextMu      sync.RWMutex
	context        *BrowserContext
	defaultContext *BrowserContext

	// Needed as the targets map will be accessed from multiple Go routines,
	// the main VU/JS go routine and the Go routine listening for CDP messages.
	pagesMu sync.RWMutex
	pages   map[target.ID]*Page

	sessionIDtoTargetIDMu sync.RWMutex
	sessionIDtoTargetID   map[target.SessionID]target.ID

	// Used to display a warning when the browser is reclosed.
	closed bool

	// version caches the browser version information.
	version browserVersion

	// runOnClose is a list of functions to run when the browser is closed.
	runOnClose []func() error

	logger *log.Logger
}

// browserVersion is a struct to hold the browser version information.
type browserVersion struct {
	protocolVersion string
	product         string
	revision        string
	userAgent       string
	jsVersion       string
}

// NewBrowser creates a new browser, connects to it, then returns it.
func NewBrowser(
	ctx context.Context,
	vuCtx context.Context,
	vuCtxCancelFn context.CancelFunc,
	browserProc *BrowserProcess,
	browserOpts *BrowserOptions,
	logger *log.Logger,
) (*Browser, error) {
	b := newBrowser(ctx, vuCtx, vuCtxCancelFn, browserProc, browserOpts, logger)
	if err := b.connect(); err != nil {
		return nil, err
	}

	// cache the browser version information.
	var err error
	if b.version, err = b.fetchVersion(); err != nil {
		return nil, err
	}

	return b, nil
}

// newBrowser returns a ready to use Browser without connecting to an actual browser.
func newBrowser(
	ctx context.Context,
	vuCtx context.Context,
	vuCtxCancelFn context.CancelFunc,
	browserProc *BrowserProcess,
	browserOpts *BrowserOptions,
	logger *log.Logger,
) *Browser {
	// The browser needs its own context to correctly close dependencies such
	// as the connection. It cannot rely on the vuCtx since that would close
	// the connection too early. The connection and subprocess need to be
	// shutdown at around the same time to allow for any last minute CDP
	// cleanup messages to be sent to chromium.
	ctx, cancelFn := context.WithCancel(ctx)

	return &Browser{
		browserCtx:          ctx,
		browserCancelFn:     cancelFn,
		vuCtx:               vuCtx,
		vuCtxCancelFn:       vuCtxCancelFn,
		state:               BrowserStateOpen,
		browserProc:         browserProc,
		browserOpts:         browserOpts,
		pages:               make(map[target.ID]*Page),
		sessionIDtoTargetID: make(map[target.SessionID]target.ID),
		logger:              logger,
	}
}

func (b *Browser) connect() error {
	b.logger.Debugf("Browser:connect", "wsURL:%q", b.browserProc.WsURL())

	// connectionOnAttachedToTarget hooks into the connection to listen
	// for target attachment events. this way, browser can manage the
	// decision of target attachments. so that we can stop connection
	// from doing unnecessary work.
	//
	// We need the connection to shutdown when browser.Close is called.
	// This is why we're using the internal context.
	var err error
	b.conn, err = NewConnection(
		b.browserCtx,
		b.browserProc.WsURL(),
		b.logger,
		b.connectionOnAttachedToTarget,
	)
	if err != nil {
		return fmt.Errorf("connecting to browser DevTools URL: %w", err)
	}

	// We don't need to lock this because `connect()` is called only in NewBrowser
	b.defaultContext, err = NewBrowserContext(b.vuCtx, b, "", DefaultBrowserContextOptions(), b.logger)
	if err != nil {
		return fmt.Errorf("browser connect: %w", err)
	}
	b.runOnClose = append(b.runOnClose, b.defaultContext.cleanup)

	return b.initEvents()
}

func (b *Browser) disposeContext(id cdp.BrowserContextID) error {
	b.logger.Debugf("Browser:disposeContext", "bctxid:%v", id)

	action := target.DisposeBrowserContext(id)
	if err := action.Do(cdp.WithExecutor(b.vuCtx, b.conn)); err != nil {
		return fmt.Errorf("disposing browser context ID %s: %w", id, err)
	}
	b.context = nil

	return nil
}

// getDefaultBrowserContextOrMatchedID returns the BrowserContext for the given browser context ID.
// If the browser context is not found, the default BrowserContext is returned.
func (b *Browser) getDefaultBrowserContextOrMatchedID(id cdp.BrowserContextID) *BrowserContext {
	b.contextMu.RLock()
	defer b.contextMu.RUnlock()

	if b.context == nil || b.context.id != id {
		return b.defaultContext
	}

	return b.context
}

func (b *Browser) getPages() []*Page {
	b.pagesMu.RLock()
	defer b.pagesMu.RUnlock()
	pages := make([]*Page, 0, len(b.pages))
	for _, p := range b.pages {
		pages = append(pages, p)
	}
	return pages
}

func (b *Browser) initEvents() error {
	chHandler := make(chan Event)

	// Using the internal context here. Using vuCtx would close the connection/subprocess
	// and therefore shutdown chromium when the iteration ends which isn't what we
	// want to happen. Chromium should only be closed by the k6 event system.
	b.conn.on(b.browserCtx, []string{
		cdproto.EventTargetAttachedToTarget,
		cdproto.EventTargetDetachedFromTarget,
		EventConnectionClose,
	}, chHandler)

	go func() {
		defer func() {
			b.browserProc.didLoseConnection()
			// Closing the vuCtx incase it hasn't already been closed. Very likely
			// already closed since the vuCtx is controlled by the k6 iteration,
			// whereas the initContext is controlled by the k6 event system when
			// browser.close() is called. k6 iteration ends before the event system.
			if b.vuCtxCancelFn != nil {
				b.vuCtxCancelFn()
			}
		}()
		for {
			select {
			case <-b.browserCtx.Done():
				return
			case event := <-chHandler:
				if ev, ok := event.data.(*target.EventAttachedToTarget); ok {
					b.logger.Debugf("Browser:initEvents:onAttachedToTarget", "sid:%v tid:%v", ev.SessionID, ev.TargetInfo.TargetID)
					if err := b.onAttachedToTarget(ev); err != nil {
						k6ext.Panic(b.vuCtx, "browser is attaching to target: %w", err)
					}
				} else if ev, ok := event.data.(*target.EventDetachedFromTarget); ok {
					b.logger.Debugf("Browser:initEvents:onDetachedFromTarget", "sid:%v", ev.SessionID)
					b.onDetachedFromTarget(ev)
				} else if event.typ == EventConnectionClose {
					b.logger.Debugf("Browser:initEvents:EventConnectionClose", "")
					return
				}
			}
		}
	}()

	action := target.SetAutoAttach(true, true).WithFlatten(true)
	if err := action.Do(cdp.WithExecutor(b.vuCtx, b.conn)); err != nil {
		return fmt.Errorf("internal error while auto-attaching to browser pages: %w", err)
	}

	// Target.setAutoAttach has a bug where it does not wait for new Targets being attached.
	// However making a dummy call afterwards fixes this.
	// This can be removed after https://chromium-review.googlesource.com/c/chromium/src/+/2885888 lands in stable.
	action2 := target.GetTargetInfo()
	if _, err := action2.Do(cdp.WithExecutor(b.vuCtx, b.conn)); err != nil {
		return fmt.Errorf("internal error while getting browser target info: %w", err)
	}

	return nil
}

// connectionOnAttachedToTarget is called when Connection receives an attachedToTarget
// event. Returning false will stop the event from being processed by the connection.
func (b *Browser) connectionOnAttachedToTarget(eva *target.EventAttachedToTarget) bool {
	// This allows to attach targets to the same browser context as the current
	// one, and to the default browser context.
	//
	// We don't want to hold the lock for the entire function
	// (connectionOnAttachedToTarget) run duration, because we want to avoid
	// possible lock contention issues with the browser context being closed while
	// we're waiting for it. So, we do the lock management in a function with its
	// own defer.
	isAllowedBrowserContext := func() bool {
		b.contextMu.RLock()
		defer b.contextMu.RUnlock()
		return b.context == nil || b.context.id == eva.TargetInfo.BrowserContextID
	}

	return isAllowedBrowserContext()
}

// onAttachedToTarget is called when a new page is attached to the browser.
func (b *Browser) onAttachedToTarget(ev *target.EventAttachedToTarget) error {
	b.logger.Debugf("Browser:onAttachedToTarget", "sid:%v tid:%v bctxid:%v",
		ev.SessionID, ev.TargetInfo.TargetID, ev.TargetInfo.BrowserContextID)

	var (
		targetPage = ev.TargetInfo
		browserCtx = b.getDefaultBrowserContextOrMatchedID(targetPage.BrowserContextID)
	)

	if !b.isAttachedPageValid(ev, browserCtx) {
		return nil // Ignore this page.
	}
	session := b.conn.getSession(ev.SessionID)
	if session == nil {
		b.logger.Debugf("Browser:onAttachedToTarget",
			"session closed before attachToTarget is handled. sid:%v tid:%v",
			ev.SessionID, targetPage.TargetID)
		return nil // ignore
	}

	var (
		isPage = targetPage.Type == "page"
		opener *Page
	)
	// Opener is nil for the initial page.
	if isPage {
		b.pagesMu.RLock()
		if t, ok := b.pages[targetPage.OpenerID]; ok {
			opener = t
		}
		b.pagesMu.RUnlock()
	}
	p, err := NewPage(b.vuCtx, session, browserCtx, targetPage.TargetID, opener, isPage, b.logger)
	if err != nil && b.isPageAttachmentErrorIgnorable(ev, session, err) {
		return nil // Ignore this page.
	}
	if err != nil {
		return fmt.Errorf("creating a new %s: %w", targetPage.Type, err)
	}

	b.attachNewPage(p, ev) // Register the page as an active page.

	// Emit the page event only for pages, not for background pages.
	// Background pages are created by extensions.
	if isPage {
		browserCtx.emit(EventBrowserContextPage, p)
	}

	return nil
}

// attachNewPage registers the page as an active page and attaches the sessionID with the targetID.
func (b *Browser) attachNewPage(p *Page, ev *target.EventAttachedToTarget) {
	targetPage := ev.TargetInfo

	// Register the page as an active page.
	b.logger.Debugf("Browser:attachNewPage:addTarget", "sid:%v tid:%v pageType:%s",
		ev.SessionID, targetPage.TargetID, targetPage.Type)
	b.pagesMu.Lock()
	b.pages[targetPage.TargetID] = p
	b.pagesMu.Unlock()

	// Attach the sessionID with the targetID so we can communicate with the
	// page later.
	b.logger.Debugf("Browser:attachNewPage:addSession", "sid:%v tid:%v pageType:%s",
		ev.SessionID, targetPage.TargetID, targetPage.Type)
	b.sessionIDtoTargetIDMu.Lock()
	b.sessionIDtoTargetID[ev.SessionID] = targetPage.TargetID
	b.sessionIDtoTargetIDMu.Unlock()
}

// isAttachedPageValid returns true if the attached page is valid and should be
// added to the browser's pages. It returns false if the attached page is not
// valid and should be ignored.
func (b *Browser) isAttachedPageValid(ev *target.EventAttachedToTarget, browserCtx *BrowserContext) bool {
	targetPage := ev.TargetInfo

	// We're not interested in the top-level browser target, other targets or DevTools targets right now.
	isDevTools := strings.HasPrefix(targetPage.URL, "devtools://devtools")
	if targetPage.Type == "browser" || targetPage.Type == "other" || isDevTools {
		b.logger.Debugf("Browser:isAttachedPageValid:return", "sid:%v tid:%v (devtools)", ev.SessionID, targetPage.TargetID)
		return false
	}
	pageType := targetPage.Type
	if pageType != "page" && pageType != "background_page" {
		b.logger.Warnf(
			"Browser:isAttachedPageValid", "sid:%v tid:%v bctxid:%v bctx nil:%t, unknown target type: %q",
			ev.SessionID, targetPage.TargetID, targetPage.BrowserContextID, browserCtx == nil, targetPage.Type)
		return false
	}
	// If the target is not in the same browser context as the current one, ignore it.
	if browserCtx.id != targetPage.BrowserContextID {
		b.logger.Debugf(
			"Browser:isAttachedPageValid", "incorrect browser context sid:%v tid:%v bctxid:%v target bctxid:%v",
			ev.SessionID, targetPage.TargetID, targetPage.BrowserContextID, browserCtx.id,
		)
		return false
	}

	return true
}

// isPageAttachmentErrorIgnorable returns true if the error is ignorable.
func (b *Browser) isPageAttachmentErrorIgnorable(ev *target.EventAttachedToTarget, session *Session, err error) bool {
	targetPage := ev.TargetInfo

	// If we're no longer connected to browser, then ignore WebSocket errors.
	// This can happen when the browser is closed while the page is being attached.
	var (
		isRunning = atomic.LoadInt64(&b.state) == BrowserStateOpen && b.IsConnected() // b.conn.isConnected()
		wsErr     *websocket.CloseError
	)
	if !errors.As(err, &wsErr) && !isRunning {
		// If we're no longer connected to browser, then ignore WebSocket errors
		b.logger.Debugf("Browser:isPageAttachmentErrorIgnorable:return",
			"sid:%v tid:%v pageType:%s websocket err:%v",
			ev.SessionID, targetPage.TargetID, targetPage.Type, err)
		return true
	}
	// No need to register the page if the test run is over.
	select {
	case <-b.vuCtx.Done():
		b.logger.Debugf("Browser:isPageAttachmentErrorIgnorable:return:<-ctx.Done",
			"sid:%v tid:%v pageType:%s err:%v",
			ev.SessionID, targetPage.TargetID, targetPage.Type, b.vuCtx.Err())
		return true
	default:
	}
	// Another VU or instance closed the page, and the session is closed.
	// This can happen if the page is closed before the attachedToTarget
	// event is handled.
	if session.Closed() {
		b.logger.Debugf("Browser:isPageAttachmentErrorIgnorable:return:session.Done",
			"session closed: sid:%v tid:%v pageType:%s err:%v",
			ev.SessionID, targetPage.TargetID, targetPage.Type, err)
		return true
	}

	return false // cannot ignore
}

// onDetachedFromTarget event can be issued multiple times per target if multiple
// sessions have been attached to it. So we'll remove the page only once.
func (b *Browser) onDetachedFromTarget(ev *target.EventDetachedFromTarget) {
	b.sessionIDtoTargetIDMu.RLock()
	targetID, ok := b.sessionIDtoTargetID[ev.SessionID]

	b.logger.Debugf("Browser:onDetachedFromTarget", "sid:%v tid:%v", ev.SessionID, targetID)
	defer b.logger.Debugf("Browser:onDetachedFromTarget:return", "sid:%v tid:%v", ev.SessionID, targetID)

	b.sessionIDtoTargetIDMu.RUnlock()
	if !ok {
		// We don't track targets of type "browser", "other" and "devtools",
		// so ignore if we don't recognize target.
		return
	}

	b.pagesMu.Lock()
	defer b.pagesMu.Unlock()
	if t, ok := b.pages[targetID]; ok {
		b.logger.Debugf("Browser:onDetachedFromTarget:deletePage", "sid:%v tid:%v", ev.SessionID, targetID)

		delete(b.pages, targetID)
		t.didClose()
	}
}

func (b *Browser) newPageInContext(id cdp.BrowserContextID) (*Page, error) {
	if b.context == nil || b.context.id != id {
		return nil, fmt.Errorf("missing browser context %s, current context is %s", id, b.context.id)
	}

	ctx, cancel := context.WithTimeout(b.vuCtx, b.browserOpts.Timeout)
	defer cancel()

	// buffer of one is for sending the target ID whether an event handler
	// exists or not.
	targetID := make(chan target.ID, 1)

	waitForPage, removeEventHandler := createWaitForEventHandler(
		ctx,
		b.context, // browser context will emit the following event:
		[]string{EventBrowserContextPage},
		func(e any) bool {
			tid := <-targetID

			b.logger.Debugf("Browser:newPageInContext:createWaitForEventHandler",
				"tid:%v ptid:%v bctxid:%v", tid, e.(*Page).targetID, id) //nolint:forcetypeassert

			// we are only interested in the new page.
			return e.(*Page).targetID == tid //nolint:forcetypeassert
		},
	)
	defer removeEventHandler()

	// create a new page.
	action := target.CreateTarget(BlankPage).WithBrowserContextID(id)
	tid, err := action.Do(cdp.WithExecutor(ctx, b.conn))
	if err != nil {
		return nil, fmt.Errorf("creating a new blank page: %w", err)
	}
	// let the event handler know about the new page.
	targetID <- tid
	var page *Page
	select {
	case <-waitForPage:
		b.logger.Debugf("Browser:newPageInContext:<-waitForPage", "tid:%v bctxid:%v", tid, id)
		b.pagesMu.RLock()
		page = b.pages[tid]
		b.pagesMu.RUnlock()
	case <-ctx.Done():
		b.logger.Debugf("Browser:newPageInContext:<-ctx.Done", "tid:%v bctxid:%v err:%v", tid, id, ctx.Err())
	}

	if err = ctx.Err(); err != nil {
		err = &k6ext.UserFriendlyError{
			Err:     ctx.Err(),
			Timeout: b.browserOpts.Timeout,
		}
	}

	if err == nil && page == nil {
		err = &k6ext.UserFriendlyError{
			Err: errors.New("can't fetch the page for unknown reason"),
		}
	}

	return page, err
}

// Close shuts down the browser.
func (b *Browser) Close() {
	// This will help with some cleanup in the connection and event loop above in
	// initEvents().
	defer b.browserCancelFn()

	if b.closed {
		b.logger.Warnf(
			"Browser:Close",
			"Please call browser.close only once, and do not use the browser after calling close.",
		)
		return
	}
	b.closed = true

	defer func() {
		if err := b.browserProc.Cleanup(); err != nil {
			b.logger.Errorf("Browser:Close", "cleaning up the user data directory: %v", err)
		}
	}()
	defer func() {
		for _, fn := range b.runOnClose {
			if err := fn(); err != nil {
				b.logger.Errorf("Browser:Close", "running cleanup function: %v", err)
			}
		}
	}()

	b.logger.Debugf("Browser:Close", "")
	atomic.CompareAndSwapInt64(&b.state, b.state, BrowserStateClosed)

	// Signal to the connection and the process that we're gracefully closing.
	// We ignore any IO errors reading from the WS connection, because the below
	// CDP Browser.close command ends the connection unexpectedly, which causes
	// `websocket.ReadMessage()` to return `close 1006 (abnormal closure):
	// unexpected EOF`.
	b.conn.IgnoreIOErrors()
	b.browserProc.GracefulClose()

	// If the browser is not being executed remotely, send the Browser.close CDP
	// command, which triggers the browser process to exit.
	if !b.browserOpts.isRemoteBrowser {
		var closeErr *websocket.CloseError
		// Using the internal context with a timeout of 10 seconds here since
		// 1. vu context will very likely be closed;
		// 2. there's a chance that the process has died but the connection still
		//    thinks it's open.
		toCtx, toCancelCtx := context.WithTimeout(b.browserCtx, time.Second*10)
		defer toCancelCtx()

		err := cdpbrowser.Close().Do(cdp.WithExecutor(toCtx, b.conn))
		if err != nil && !errors.As(err, &closeErr) {
			b.logger.Errorf("Browser:Close", "closing the browser: %v", err)
		}
	}

	// Wait for all outstanding events (e.g. Target.detachedFromTarget) to be
	// processed, and for the process to exit gracefully. Otherwise kill it
	// forcefully after the timeout.
	timeout := time.Second
	select {
	case <-b.browserProc.processDone:
	case <-time.After(timeout):
		b.logger.Debugf("Browser:Close", "killing browser process with PID %d after %s", b.browserProc.Pid(), timeout)
		b.browserProc.Terminate()
	}

	// This is unintuitive, since the process exited, so the connection would've
	// been closed as well. The reason we still call conn.Close() here is to
	// close all sessions and emit the EventConnectionClose event, which will
	// trigger the cancellation of the main browser context. We don't call it
	// before the process is done to avoid disconnecting too early, since we
	// expect some CDP events to arrive after Browser.close, and we can't know
	// for sure when that has finished. This will error writing to the socket,
	// but we ignore it.
	b.conn.Close()
}

// CloseContext is a short-cut function to close the current browser's context.
// If there is no active browser context, it returns an error.
func (b *Browser) CloseContext() error {
	if b.context == nil {
		return errors.New("cannot close context as none is active in browser")
	}
	return b.context.Close()
}

// Context returns the current browser context or nil.
func (b *Browser) Context() *BrowserContext {
	return b.context
}

// IsConnected returns whether the WebSocket connection to the browser process
// is active or not.
func (b *Browser) IsConnected() bool {
	return b.browserProc.isConnected()
}

// NewContext creates a new incognito-like browser context.
func (b *Browser) NewContext(opts *BrowserContextOptions) (*BrowserContext, error) {
	_, span := TraceAPICall(b.vuCtx, "", "browser.newContext")
	defer span.End()

	if b.context != nil {
		err := errors.New("existing browser context must be closed before creating a new one")
		spanRecordError(span, err)
		return nil, err
	}

	action := target.CreateBrowserContext().WithDisposeOnDetach(true)
	browserContextID, err := action.Do(cdp.WithExecutor(b.vuCtx, b.conn))
	b.logger.Debugf("Browser:NewContext", "bctxid:%v", browserContextID)
	if err != nil {
		err := fmt.Errorf("creating browser context ID %s: %w", browserContextID, err)
		spanRecordError(span, err)
		return nil, err
	}

	browserCtx, err := NewBrowserContext(b.vuCtx, b, browserContextID, opts, b.logger)
	if err != nil {
		err := fmt.Errorf("new context: %w", err)
		spanRecordError(span, err)
		return nil, err
	}
	b.runOnClose = append(b.runOnClose, browserCtx.cleanup)

	b.contextMu.Lock()
	defer b.contextMu.Unlock()
	b.context = browserCtx

	return browserCtx, nil
}

// NewPage creates a new tab in the browser window.
func (b *Browser) NewPage(opts *BrowserContextOptions) (*Page, error) {
	_, span := TraceAPICall(b.vuCtx, "", "browser.newPage")
	defer span.End()

	browserCtx, err := b.NewContext(opts)
	if err != nil {
		err := fmt.Errorf("new page: %w", err)
		spanRecordError(span, err)
		return nil, err
	}

	page, err := browserCtx.NewPage()
	if err != nil {
		spanRecordError(span, err)
		return nil, err
	}

	return page, nil
}

// On returns a Promise that is resolved when the browser process is disconnected.
// The only accepted event value is "disconnected".
func (b *Browser) On(event string) (bool, error) {
	if event != EventBrowserDisconnected {
		return false, fmt.Errorf("unknown browser event: %q, must be %q", event, EventBrowserDisconnected)
	}

	select {
	case <-b.browserProc.lostConnection:
		return true, nil
	case <-b.vuCtx.Done():
		return false, fmt.Errorf("browser.on promise rejected: %w", b.vuCtx.Err())
	}
}

// UserAgent returns the controlled browser's user agent string.
func (b *Browser) UserAgent() string {
	return b.version.userAgent
}

// Version returns the controlled browser's version.
func (b *Browser) Version() string {
	product := b.version.product
	i := strings.Index(product, "/")
	if i == -1 {
		return product
	}
	return product[i+1:]
}

// fetchVersion returns the browser version information.
func (b *Browser) fetchVersion() (browserVersion, error) {
	var (
		bv  browserVersion
		err error
	)
	bv.protocolVersion, bv.product, bv.revision, bv.userAgent, bv.jsVersion, err = cdpbrowser.
		GetVersion().
		Do(cdp.WithExecutor(b.vuCtx, b.conn))
	if err != nil {
		return browserVersion{}, fmt.Errorf("getting browser version information: %w", err)
	}

	// Adjust the user agent to remove the headless part.
	//
	// Including Headless might cause issues with some websites that treat headless
	// browsers differently. Later on, [BrowserContext] will set the user agent to
	// this user agent if not set by the user. This will force [FrameSession] to
	// set the user agent to the browser's user agent.
	//
	// Doing this here provides a consistent user agent across all browser contexts.
	// Also, it makes it consistent to query the user agent from the browser.
	if b.browserOpts.Headless {
		bv.userAgent = strings.ReplaceAll(bv.userAgent, "Headless", "")
	}

	return bv, nil
}

// WsURL returns the Websocket URL that the browser is listening on for CDP clients.
func (b *Browser) WsURL() string {
	return b.browserProc.WsURL()
}
