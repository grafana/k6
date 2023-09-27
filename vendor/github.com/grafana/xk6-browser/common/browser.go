package common

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/k6ext"
	"github.com/grafana/xk6-browser/log"

	k6modules "go.k6.io/k6/js/modules"

	"github.com/chromedp/cdproto"
	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/dop251/goja"
	"github.com/gorilla/websocket"
)

// Ensure Browser implements the EventEmitter and Browser interfaces.
var (
	_ api.Browser = &Browser{}
)

const (
	BrowserStateOpen int64 = iota
	BrowserStateClosed
)

// Browser stores a Browser context.
type Browser struct {
	ctx      context.Context
	cancelFn context.CancelFunc

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

	// Cancel function to stop event listening
	evCancelFn context.CancelFunc

	// Needed as the targets map will be accessed from multiple Go routines,
	// the main VU/JS go routine and the Go routine listening for CDP messages.
	pagesMu sync.RWMutex
	pages   map[target.ID]*Page

	sessionIDtoTargetIDMu sync.RWMutex
	sessionIDtoTargetID   map[target.SessionID]target.ID

	// Used to display a warning when the browser is reclosed.
	closed bool

	vu k6modules.VU

	logger *log.Logger
}

// NewBrowser creates a new browser, connects to it, then returns it.
func NewBrowser(
	ctx context.Context,
	cancel context.CancelFunc,
	browserProc *BrowserProcess,
	browserOpts *BrowserOptions,
	logger *log.Logger,
) (*Browser, error) {
	b := newBrowser(ctx, cancel, browserProc, browserOpts, logger)
	if err := b.connect(); err != nil {
		return nil, err
	}
	return b, nil
}

// newBrowser returns a ready to use Browser without connecting to an actual browser.
func newBrowser(
	ctx context.Context,
	cancelFn context.CancelFunc,
	browserProc *BrowserProcess,
	browserOpts *BrowserOptions,
	logger *log.Logger,
) *Browser {
	return &Browser{
		ctx:                 ctx,
		cancelFn:            cancelFn,
		state:               int64(BrowserStateOpen),
		browserProc:         browserProc,
		browserOpts:         browserOpts,
		pages:               make(map[target.ID]*Page),
		sessionIDtoTargetID: make(map[target.SessionID]target.ID),
		vu:                  k6ext.GetVU(ctx),
		logger:              logger,
	}
}

func (b *Browser) connect() error {
	b.logger.Debugf("Browser:connect", "wsURL:%q", b.browserProc.WsURL())
	conn, err := NewConnection(b.ctx, b.browserProc.WsURL(), b.logger)
	if err != nil {
		return fmt.Errorf("connecting to browser DevTools URL: %w", err)
	}

	b.conn = conn

	// We don't need to lock this because `connect()` is called only in NewBrowser
	b.defaultContext, err = NewBrowserContext(b.ctx, b, "", NewBrowserContextOptions(), b.logger)
	if err != nil {
		return fmt.Errorf("browser connect: %w", err)
	}

	return b.initEvents()
}

func (b *Browser) disposeContext(id cdp.BrowserContextID) error {
	b.logger.Debugf("Browser:disposeContext", "bctxid:%v", id)

	action := target.DisposeBrowserContext(id)
	if err := action.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
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
	var cancelCtx context.Context
	cancelCtx, b.evCancelFn = context.WithCancel(b.ctx)
	chHandler := make(chan Event)

	b.conn.on(cancelCtx, []string{
		cdproto.EventTargetAttachedToTarget,
		cdproto.EventTargetDetachedFromTarget,
		EventConnectionClose,
	}, chHandler)

	go func() {
		defer func() {
			b.logger.Debugf("Browser:initEvents:defer", "ctx err: %v", cancelCtx.Err())
			b.browserProc.didLoseConnection()
			if b.cancelFn != nil {
				b.cancelFn()
			}
		}()
		for {
			select {
			case <-cancelCtx.Done():
				return
			case event := <-chHandler:
				if ev, ok := event.data.(*target.EventAttachedToTarget); ok {
					b.logger.Debugf("Browser:initEvents:onAttachedToTarget", "sid:%v tid:%v", ev.SessionID, ev.TargetInfo.TargetID)
					b.onAttachedToTarget(ev)
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
	if err := action.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
		return fmt.Errorf("internal error while auto-attaching to browser pages: %w", err)
	}

	// Target.setAutoAttach has a bug where it does not wait for new Targets being attached.
	// However making a dummy call afterwards fixes this.
	// This can be removed after https://chromium-review.googlesource.com/c/chromium/src/+/2885888 lands in stable.
	action2 := target.GetTargetInfo()
	if _, err := action2.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
		return fmt.Errorf("internal error while getting browser target info: %w", err)
	}

	return nil
}

// onAttachedToTarget is called when a new page is attached to the browser.
func (b *Browser) onAttachedToTarget(ev *target.EventAttachedToTarget) {
	b.logger.Debugf("Browser:onAttachedToTarget", "sid:%v tid:%v bctxid:%v",
		ev.SessionID, ev.TargetInfo.TargetID, ev.TargetInfo.BrowserContextID)

	var (
		targetPage = ev.TargetInfo
		browserCtx = b.getDefaultBrowserContextOrMatchedID(targetPage.BrowserContextID)
	)

	if !b.isAttachedPageValid(ev, browserCtx) {
		return // Ignore this page.
	}
	session := b.conn.getSession(ev.SessionID)
	if session == nil {
		b.logger.Debugf("Browser:onAttachedToTarget",
			"session closed before attachToTarget is handled. sid:%v tid:%v",
			ev.SessionID, targetPage.TargetID)
		return // ignore
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
	p, err := NewPage(b.ctx, session, browserCtx, targetPage.TargetID, opener, isPage, b.logger)
	if err != nil && b.isPageAttachmentErrorIgnorable(ev, session, err) {
		return // Ignore this page.
	}
	if err != nil {
		k6ext.Panic(b.ctx, "creating a new %s: %w", targetPage.Type, err)
	}
	b.attachNewPage(p, ev) // Register the page as an active page.
	// Emit the page event only for pages, not for background pages.
	// Background pages are created by extensions.
	if isPage {
		browserCtx.emit(EventBrowserContextPage, p)
	}
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
	case <-b.ctx.Done():
		b.logger.Debugf("Browser:isPageAttachmentErrorIgnorable:return:<-ctx.Done",
			"sid:%v tid:%v pageType:%s err:%v",
			ev.SessionID, targetPage.TargetID, targetPage.Type, b.ctx.Err())
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

	ctx, cancel := context.WithTimeout(b.ctx, b.browserOpts.Timeout)
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
				"tid:%v ptid:%v bctxid:%v", tid, e.(*Page).targetID, id)

			// we are only interested in the new page.
			return e.(*Page).targetID == tid
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
		err = &k6ext.UserFriendlyError{
			Err:     ctx.Err(),
			Timeout: b.browserOpts.Timeout,
		}
		b.logger.Debugf("Browser:newPageInContext:<-ctx.Done", "tid:%v bctxid:%v err:%v", tid, id, err)
	}
	return page, err
}

// Close shuts down the browser.
func (b *Browser) Close() {
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
		err := cdpbrowser.Close().Do(cdp.WithExecutor(b.ctx, b.conn))
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

// Context returns the current browser context or nil.
func (b *Browser) Context() api.BrowserContext {
	return b.context
}

// IsConnected returns whether the WebSocket connection to the browser process
// is active or not.
func (b *Browser) IsConnected() bool {
	return b.browserProc.isConnected()
}

// NewContext creates a new incognito-like browser context.
func (b *Browser) NewContext(opts goja.Value) (api.BrowserContext, error) {
	if b.context != nil {
		return nil, errors.New("existing browser context must be closed before creating a new one")
	}

	action := target.CreateBrowserContext().WithDisposeOnDetach(true)
	browserContextID, err := action.Do(cdp.WithExecutor(b.ctx, b.conn))
	b.logger.Debugf("Browser:NewContext", "bctxid:%v", browserContextID)
	if err != nil {
		k6ext.Panic(b.ctx, "creating browser context ID %s: %w", browserContextID, err)
	}

	browserCtxOpts := NewBrowserContextOptions()
	if err := browserCtxOpts.Parse(b.ctx, opts); err != nil {
		k6ext.Panic(b.ctx, "parsing newContext options: %w", err)
	}

	browserCtx, err := NewBrowserContext(b.ctx, b, browserContextID, browserCtxOpts, b.logger)
	if err != nil {
		return nil, fmt.Errorf("new context: %w", err)
	}

	b.contextMu.Lock()
	defer b.contextMu.Unlock()
	b.context = browserCtx

	return browserCtx, nil
}

// NewPage creates a new tab in the browser window.
func (b *Browser) NewPage(opts goja.Value) (api.Page, error) {
	browserCtx, err := b.NewContext(opts)
	if err != nil {
		return nil, fmt.Errorf("new page: %w", err)
	}

	return browserCtx.NewPage()
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
	case <-b.ctx.Done():
		return false, fmt.Errorf("browser.on promise rejected: %w", b.ctx.Err())
	}
}

// UserAgent returns the controlled browser's user agent string.
func (b *Browser) UserAgent() string {
	action := cdpbrowser.GetVersion()
	_, _, _, ua, _, err := action.Do(cdp.WithExecutor(b.ctx, b.conn))
	if err != nil {
		k6ext.Panic(b.ctx, "getting browser user agent: %w", err)
	}
	return ua
}

// Version returns the controlled browser's version.
func (b *Browser) Version() string {
	action := cdpbrowser.GetVersion()
	_, product, _, _, _, err := action.Do(cdp.WithExecutor(b.ctx, b.conn))
	if err != nil {
		k6ext.Panic(b.ctx, "getting browser version: %w", err)
	}
	i := strings.Index(product, "/")
	if i == -1 {
		return product
	}
	return product[i+1:]
}

// WsURL returns the Websocket URL that the browser is listening on for CDP clients.
func (b *Browser) WsURL() string {
	return b.browserProc.WsURL()
}
