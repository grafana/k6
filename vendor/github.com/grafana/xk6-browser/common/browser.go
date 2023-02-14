package common

import (
	"context"
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
var _ EventEmitter = &Browser{}
var _ api.Browser = &Browser{}

const (
	BrowserStateOpen int64 = iota
	BrowserStateClosed
)

// Browser stores a Browser context.
type Browser struct {
	BaseEventEmitter

	ctx      context.Context
	cancelFn context.CancelFunc

	state int64

	browserProc *BrowserProcess
	launchOpts  *LaunchOptions

	// Connection to the browser to talk CDP protocol.
	// A *Connection is saved to this field, see: connect().
	conn connection

	contextsMu     sync.RWMutex
	contexts       map[cdp.BrowserContextID]*BrowserContext
	defaultContext *BrowserContext

	// Cancel function to stop event listening
	evCancelFn context.CancelFunc

	// Needed as the targets map will be accessed from multiple Go routines,
	// the main VU/JS go routine and the Go routine listening for CDP messages.
	pagesMu sync.RWMutex
	pages   map[target.ID]*Page

	sessionIDtoTargetIDMu sync.RWMutex
	sessionIDtoTargetID   map[target.SessionID]target.ID

	vu k6modules.VU

	logger *log.Logger
}

// NewBrowser creates a new browser, connects to it, then returns it.
func NewBrowser(
	ctx context.Context,
	cancel context.CancelFunc,
	browserProc *BrowserProcess,
	launchOpts *LaunchOptions,
	logger *log.Logger,
) (*Browser, error) {
	b := newBrowser(ctx, cancel, browserProc, launchOpts, logger)
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
	launchOpts *LaunchOptions,
	logger *log.Logger,
) *Browser {
	return &Browser{
		BaseEventEmitter:    NewBaseEventEmitter(ctx),
		ctx:                 ctx,
		cancelFn:            cancelFn,
		state:               int64(BrowserStateOpen),
		browserProc:         browserProc,
		launchOpts:          launchOpts,
		contexts:            make(map[cdp.BrowserContextID]*BrowserContext),
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
	b.defaultContext = NewBrowserContext(b.ctx, b, "", NewBrowserContextOptions(), b.logger)

	return b.initEvents()
}

func (b *Browser) disposeContext(id cdp.BrowserContextID) error {
	b.logger.Debugf("Browser:disposeContext", "bctxid:%v", id)

	action := target.DisposeBrowserContext(id)
	if err := action.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
		return fmt.Errorf("disposing browser context ID %s: %w", id, err)
	}

	b.contextsMu.Lock()
	defer b.contextsMu.Unlock()
	delete(b.contexts, id)

	return nil
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

func (b *Browser) onAttachedToTarget(ev *target.EventAttachedToTarget) {
	evti := ev.TargetInfo

	b.contextsMu.RLock()
	browserCtx := b.defaultContext
	bctx, ok := b.contexts[evti.BrowserContextID]
	if ok {
		browserCtx = bctx
	}
	b.contextsMu.RUnlock()

	b.logger.Debugf("Browser:onAttachedToTarget", "sid:%v tid:%v bctxid:%v bctx nil:%t",
		ev.SessionID, evti.TargetID, evti.BrowserContextID, browserCtx == nil)

	// We're not interested in the top-level browser target, other targets or DevTools targets right now.
	isDevTools := strings.HasPrefix(evti.URL, "devtools://devtools")
	if evti.Type == "browser" || evti.Type == "other" || isDevTools {
		b.logger.Debugf("Browser:onAttachedToTarget:return", "sid:%v tid:%v (devtools)", ev.SessionID, evti.TargetID)
		return
	}

	session := b.conn.getSession(ev.SessionID)
	if session == nil {
		b.logger.Warnf("Browser:onAttachedToTarget",
			"session closed before attachToTarget is handled. sid:%v tid:%v",
			ev.SessionID, evti.TargetID)
		return // ignore
	}

	switch evti.Type {
	case "background_page":
		p, err := NewPage(b.ctx, session, browserCtx, evti.TargetID, nil, false, b.logger)
		if err != nil {
			isRunning := atomic.LoadInt64(&b.state) == BrowserStateOpen && b.IsConnected() // b.conn.isConnected()
			if _, ok := err.(*websocket.CloseError); !ok && !isRunning {
				// If we're no longer connected to browser, then ignore WebSocket errors
				b.logger.Debugf("Browser:onAttachedToTarget:background_page:return", "sid:%v tid:%v websocket err:%v",
					ev.SessionID, evti.TargetID, err)
				return
			}
			select {
			case <-b.ctx.Done():
				b.logger.Debugf("Browser:onAttachedToTarget:background_page:return:<-ctx.Done",
					"sid:%v tid:%v err:%v",
					ev.SessionID, evti.TargetID, b.ctx.Err())
				return // ignore
			default:
				k6ext.Panic(b.ctx, "creating a new background page: %w", err)
			}
		}

		b.pagesMu.Lock()
		b.logger.Debugf("Browser:onAttachedToTarget:background_page:addTid", "sid:%v tid:%v", ev.SessionID, evti.TargetID)
		b.pages[evti.TargetID] = p
		b.pagesMu.Unlock()

		b.sessionIDtoTargetIDMu.Lock()
		b.logger.Debugf("Browser:onAttachedToTarget:background_page:addSid", "sid:%v tid:%v", ev.SessionID, evti.TargetID)
		b.sessionIDtoTargetID[ev.SessionID] = evti.TargetID
		b.sessionIDtoTargetIDMu.Unlock()
	case "page":
		// Opener is nil for the initial page
		var opener *Page
		b.pagesMu.RLock()
		if t, ok := b.pages[evti.OpenerID]; ok {
			opener = t
		}
		b.pagesMu.RUnlock()

		b.logger.Debugf("Browser:onAttachedToTarget:page", "sid:%v tid:%v opener nil:%t", ev.SessionID, evti.TargetID, opener == nil)

		p, err := NewPage(b.ctx, session, browserCtx, evti.TargetID, opener, true, b.logger)
		if err != nil {
			isRunning := atomic.LoadInt64(&b.state) == BrowserStateOpen && b.IsConnected() // b.conn.isConnected()
			if _, ok := err.(*websocket.CloseError); !ok && !isRunning {
				// If we're no longer connected to browser, then ignore WebSocket errors
				b.logger.Debugf("Browser:onAttachedToTarget:page:return", "sid:%v tid:%v websocket err:", ev.SessionID, evti.TargetID)
				return
			}
			select {
			case <-b.ctx.Done():
				b.logger.Debugf("Browser:onAttachedToTarget:page:return:<-ctx.Done",
					"sid:%v tid:%v err:%v",
					ev.SessionID, evti.TargetID, b.ctx.Err())
				return // ignore
			default:
				k6ext.Panic(b.ctx, "creating a new page: %w", err)
			}
		}

		b.pagesMu.Lock()
		b.logger.Debugf("Browser:onAttachedToTarget:page:addTarget", "sid:%v tid:%v", ev.SessionID, evti.TargetID)
		b.pages[evti.TargetID] = p
		b.pagesMu.Unlock()

		b.sessionIDtoTargetIDMu.Lock()
		b.logger.Debugf("Browser:onAttachedToTarget:page:sidToTid", "sid:%v tid:%v", ev.SessionID, evti.TargetID)
		b.sessionIDtoTargetID[ev.SessionID] = evti.TargetID
		b.sessionIDtoTargetIDMu.Unlock()

		browserCtx.emit(EventBrowserContextPage, p)
	default:
		b.logger.Warnf(
			"Browser:onAttachedToTarget", "sid:%v tid:%v bctxid:%v bctx nil:%t, unknown target type: %q",
			ev.SessionID, evti.TargetID, evti.BrowserContextID, browserCtx == nil, evti.Type)
	}
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
	b.contextsMu.RLock()
	browserCtx, ok := b.contexts[id]
	b.contextsMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("missing browser context: %s", id)
	}

	ctx, cancel := context.WithTimeout(b.ctx, b.launchOpts.Timeout)
	defer cancel()

	// buffer of one is for sending the target ID whether an event handler
	// exists or not.
	targetID := make(chan target.ID, 1)

	waitForPage, removeEventHandler := createWaitForEventHandler(
		ctx,
		browserCtx, // browser context will emit the following event:
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
	action := target.CreateTarget("about:blank").WithBrowserContextID(id)
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
			Timeout: b.launchOpts.Timeout,
		}
		b.logger.Debugf("Browser:newPageInContext:<-ctx.Done", "tid:%v bctxid:%v err:%v", tid, id, err)
	}
	return page, err
}

// Close shuts down the browser.
func (b *Browser) Close() {
	defer func() {
		if err := b.browserProc.userDataDir.Cleanup(); err != nil {
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

	// Send the Browser.close CDP command, which triggers the browser process to
	// exit.
	action := cdpbrowser.Close()
	if err := action.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
		if _, ok := err.(*websocket.CloseError); !ok {
			k6ext.Panic(b.ctx, "closing the browser: %v", err)
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

// Contexts returns list of browser contexts.
func (b *Browser) Contexts() []api.BrowserContext {
	b.contextsMu.RLock()
	defer b.contextsMu.RUnlock()

	contexts := make([]api.BrowserContext, 0, len(b.contexts))
	for _, b := range b.contexts {
		contexts = append(contexts, b)
	}

	return contexts
}

// IsConnected returns whether the WebSocket connection to the browser process
// is active or not.
func (b *Browser) IsConnected() bool {
	return b.browserProc.isConnected()
}

// NewContext creates a new incognito-like browser context.
func (b *Browser) NewContext(opts goja.Value) api.BrowserContext {
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

	b.contextsMu.Lock()
	defer b.contextsMu.Unlock()
	browserCtx := NewBrowserContext(b.ctx, b, browserContextID, browserCtxOpts, b.logger)
	b.contexts[browserContextID] = browserCtx

	return browserCtx
}

// NewPage creates a new tab in the browser window.
func (b *Browser) NewPage(opts goja.Value) api.Page {
	browserCtx := b.NewContext(opts)
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
