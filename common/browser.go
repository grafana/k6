/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package common

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto"
	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/dop251/goja"
	"github.com/gorilla/websocket"
	"github.com/grafana/xk6-browser/api"
	k6lib "go.k6.io/k6/lib"
)

// Ensure Browser implements the EventEmitter and Browser interfaces
var _ EventEmitter = &Browser{}
var _ api.Browser = &Browser{}

const (
	BrowserStateOpen int64 = iota
	BrowserStateClosing
	BrowserStateClosed
)

// Browser stores a Browser context
type Browser struct {
	BaseEventEmitter

	ctx      context.Context
	cancelFn context.CancelFunc

	state int64

	browserProc *BrowserProcess
	launchOpts  *LaunchOptions

	// Connection to browser to talk CDP protocol
	conn      *Connection
	connMu    sync.RWMutex
	connected bool

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

	logger *Logger
}

// NewBrowser creates a new browser
func NewBrowser(ctx context.Context, cancelFn context.CancelFunc, browserProc *BrowserProcess, launchOpts *LaunchOptions) (*Browser, error) {
	state := k6lib.GetState(ctx)
	reCategoryFilter, _ := regexp.Compile(launchOpts.LogCategoryFilter)
	b := Browser{
		BaseEventEmitter:      NewBaseEventEmitter(ctx),
		ctx:                   ctx,
		cancelFn:              cancelFn,
		state:                 int64(BrowserStateOpen),
		browserProc:           browserProc,
		conn:                  nil,
		connected:             false,
		launchOpts:            launchOpts,
		contexts:              make(map[cdp.BrowserContextID]*BrowserContext),
		defaultContext:        nil,
		pagesMu:               sync.RWMutex{},
		pages:                 make(map[target.ID]*Page),
		sessionIDtoTargetIDMu: sync.RWMutex{},
		sessionIDtoTargetID:   make(map[target.SessionID]target.ID),
		logger:                NewLogger(ctx, state.Logger, launchOpts.Debug, reCategoryFilter),
	}
	if err := b.connect(); err != nil {
		return nil, err
	}
	return &b, nil
}

func (b *Browser) connect() error {
	b.logger.Infof("Browser:connect", "wsurl=%v", b.browserProc.WsURL())
	var err error
	b.conn, err = NewConnection(b.ctx, b.browserProc.WsURL(), b.logger)
	if err != nil {
		return fmt.Errorf("unable to connect to browser WS URL: %w", err)
	}

	b.connMu.Lock()
	defer b.connMu.Unlock()
	b.connected = true
	b.defaultContext = NewBrowserContext(b.ctx, b.conn, b, "", NewBrowserContextOptions(), b.logger)
	return b.initEvents()
}

func (b *Browser) disposeContext(id cdp.BrowserContextID) error {
	b.logger.Infof("Browser:disposeContext", "bctxid=%v", id)
	action := target.DisposeBrowserContext(id)
	if err := action.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
		return fmt.Errorf("unable to dispose browser context %T: %w", action, err)
	}
	b.contextsMu.Lock()
	defer b.contextsMu.Unlock()
	delete(b.contexts, id)
	return nil
}

func (b *Browser) getPages() []*Page {
	b.pagesMu.RLock()
	defer b.pagesMu.RUnlock()
	pages := make([]*Page, len(b.pages))
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
		for {
			select {
			case <-cancelCtx.Done():
				return
			case event := <-chHandler:
				if ev, ok := event.data.(*target.EventAttachedToTarget); ok {
					b.logger.Infof("Browser:initEvents:onAttachedToTarget", "sid=%v, tid=%v", ev.SessionID, ev.TargetInfo.TargetID)
					go b.onAttachedToTarget(ev)
				} else if ev, ok := event.data.(*target.EventDetachedFromTarget); ok {
					b.logger.Infof("Browser:initEvents:onDetachedFromTarget", "sid=%v", ev.SessionID)
					go b.onDetachedFromTarget(ev)
				} else if event.typ == EventConnectionClose {
					b.logger.Infof("Browser:initEvents:EventConnectionClose", "sid=%v", ev.SessionID)

					b.connMu.Lock()
					b.connected = false
					b.connMu.Unlock()
					b.browserProc.didLoseConnection()
					b.cancelFn()
				}
			}
		}
	}()

	action := target.SetAutoAttach(true, true).WithFlatten(true)
	if err := action.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
		return fmt.Errorf("unable to execute %T: %w", action, err)
	}

	// Target.setAutoAttach has a bug where it does not wait for new Targets being attached.
	// However making a dummy call afterwards fixes this.
	// This can be removed after https://chromium-review.googlesource.com/c/chromium/src/+/2885888 lands in stable.
	action2 := target.GetTargetInfo()
	if _, err := action2.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
		return fmt.Errorf("unable to execute %T: %w", action, err)
	}

	return nil
}

func (b *Browser) onAttachedToTarget(ev *target.EventAttachedToTarget) {
	b.contextsMu.RLock()
	var browserCtx *BrowserContext = b.defaultContext
	bctx, ok := b.contexts[ev.TargetInfo.BrowserContextID]
	if ok {
		browserCtx = bctx
	}
	b.logger.Infof("Browser:onAttachedToTarget", "id=%v, btx nil=%t", ev.TargetInfo.BrowserContextID, bctx == nil)
	b.contextsMu.RUnlock()

	// We're not interested in the top-level browser target, other targets or DevTools targets right now.
	isDevTools := strings.HasPrefix(ev.TargetInfo.URL, "devtools://devtools")
	if ev.TargetInfo.Type == "browser" || ev.TargetInfo.Type == "other" || isDevTools {
		b.logger.Infof("Browser:onAttachedToTarget", "returns: devtools")
		return
	}

	if ev.TargetInfo.Type == "background_page" {
		p, err := NewPage(b.ctx, b.conn.getSession(ev.SessionID), browserCtx, ev.TargetInfo.TargetID, nil, false)
		if err != nil {
			isRunning := atomic.LoadInt64(&b.state) == BrowserStateOpen && b.IsConnected() //b.conn.isConnected()
			if _, ok := err.(*websocket.CloseError); !ok && !isRunning {
				// If we're no longer connected to browser, then ignore WebSocket errors
				b.logger.Infof("Browser:onAttachedToTarget:background_page", "returns: websocket error")
				return
			}
			k6Throw(b.ctx, "cannot create NewPage for background_page event: %w", err)
		}
		b.pagesMu.Lock()
		b.logger.Infof("Browser:onAttachedToTarget:background_page", "adding target id:%v", ev.TargetInfo.TargetID)
		b.pages[ev.TargetInfo.TargetID] = p
		b.pagesMu.Unlock()
		b.sessionIDtoTargetIDMu.Lock()
		b.logger.Infof("Browser:onAttachedToTarget:background_page", "adding session id:%v", ev.TargetInfo.TargetID)
		b.sessionIDtoTargetID[ev.SessionID] = ev.TargetInfo.TargetID
		b.sessionIDtoTargetIDMu.Unlock()
	} else if ev.TargetInfo.Type == "page" {
		var opener *Page = nil
		b.pagesMu.RLock()
		if t, ok := b.pages[ev.TargetInfo.OpenerID]; ok {
			opener = t
		}
		b.logger.Infof("Browser:onAttachedToTarget:page", "opener nil:%t", opener == nil)
		b.pagesMu.RUnlock()
		p, err := NewPage(b.ctx, b.conn.getSession(ev.SessionID), browserCtx, ev.TargetInfo.TargetID, opener, true)
		if err != nil {
			isRunning := atomic.LoadInt64(&b.state) == BrowserStateOpen && b.IsConnected() //b.conn.isConnected()
			if _, ok := err.(*websocket.CloseError); !ok && !isRunning {
				// If we're no longer connected to browser, then ignore WebSocket errors
				b.logger.Infof("Browser:onAttachedToTarget:page", "returns: websocket error")
				return
			}
			k6Throw(b.ctx, "cannot create NewPage for page event: %w", err)
		}
		b.pagesMu.Lock()
		b.logger.Infof("Browser:onAttachedToTarget:page", "adding page as a target id:%v", ev.TargetInfo.TargetID)
		b.pages[ev.TargetInfo.TargetID] = p
		b.pagesMu.Unlock()
		b.sessionIDtoTargetIDMu.Lock()
		b.logger.Infof("Browser:onAttachedToTarget:page", "changing session id (%v) to target id (%v)", ev.SessionID, ev.TargetInfo.TargetID)
		b.sessionIDtoTargetID[ev.SessionID] = ev.TargetInfo.TargetID
		b.sessionIDtoTargetIDMu.Unlock()
		browserCtx.emit(EventBrowserContextPage, p)
	}
}

func (b *Browser) onDetachedFromTarget(ev *target.EventDetachedFromTarget) {
	b.sessionIDtoTargetIDMu.RLock()
	targetID, ok := b.sessionIDtoTargetID[ev.SessionID]
	b.logger.Infof("Browser:onDetachedFromTarget", "sid=%v, tid=%v", ev.SessionID, targetID)
	b.sessionIDtoTargetIDMu.RUnlock()
	if !ok {
		b.logger.Infof("Browser:onDetachedFromTarget", "returns")
		// We don't track targets of type "browser", "other" and "devtools", so ignore if we don't recognize target.
		return
	}

	b.pagesMu.Lock()
	defer b.pagesMu.Unlock()
	if t, ok := b.pages[targetID]; ok {
		b.logger.Infof("Browser:onDetachedFromTarget", "deletes")
		delete(b.pages, targetID)
		t.didClose()
		b.logger.Infof("Browser:onDetachedFromTarget", "deleted")
	}
}

func (b *Browser) newPageInContext(id cdp.BrowserContextID) (*Page, error) {
	b.contextsMu.RLock()
	browserCtx, ok := b.contexts[id]
	b.contextsMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no browser context with ID %s exists", id)
	}

	var (
		mu       sync.RWMutex // protects targetID
		targetID target.ID

		err error
	)
	ch, evCancelFn := createWaitForEventHandler(
		b.ctx, browserCtx, []string{EventBrowserContextPage},
		func(data interface{}) bool {
			mu.RLock()
			defer mu.RUnlock()
			b.logger.Infof("Browser:newPageInContext", "createWaitForEventHandler page targetID: %v, targetID: %v", data.(*Page).targetID, targetID)
			return data.(*Page).targetID == targetID
		},
	)
	defer evCancelFn() // Remove event handler
	errCh := make(chan error)
	func() {
		b.logger.Infof("Browser:newPageInContext", "CreateTarget(blank) target id: %v", id)
		action := target.CreateTarget("about:blank").WithBrowserContextID(id)
		mu.Lock()
		defer mu.Unlock()
		if targetID, err = action.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
			errCh <- fmt.Errorf("unable to execute %T: %w", action, err)
		}
	}()
	select {
	case <-b.ctx.Done():
		b.logger.Infof("Browser:newPageInContext", "ctx done")
	case <-time.After(b.launchOpts.Timeout):
		b.logger.Infof("Browser:newPageInContext", "timed out")
	case c := <-ch:
		b.logger.Infof("Browser:newPageInContext", "ch: %v", c)
	case err := <-errCh:
		b.logger.Infof("Browser:newPageInContext", "err: %v", err)
		return nil, err
	}
	b.pagesMu.RLock()
	defer b.pagesMu.RUnlock()
	return b.pages[targetID], nil
}

// Close shuts down the browser
func (b *Browser) Close() {
	b.logger.Infof("Browser:Close", "")
	if !atomic.CompareAndSwapInt64(&b.state, b.state, BrowserStateClosing) {
		// If we're already in a closing state then no need to continue.
		b.logger.Infof("Browser:Close", "already in a closing state")
		return
	}
	b.logger.Infof("Browser:Close", "graceful terminate")
	b.browserProc.GracefulClose()
	defer func() {
		b.logger.Infof("Browser:Close", "browserProc terminate")
		b.browserProc.Terminate()
	}()

	action := cdpbrowser.Close()
	if err := action.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
		if _, ok := err.(*websocket.CloseError); !ok {
			k6Throw(b.ctx, "unable to execute %T: %v", action, err)
		}
	}
	atomic.CompareAndSwapInt64(&b.state, b.state, BrowserStateClosed)
}

// Contexts returns list of browser contexts
func (b *Browser) Contexts() []api.BrowserContext {
	b.contextsMu.RLock()
	defer b.contextsMu.RUnlock()
	contexts := make([]api.BrowserContext, 0, len(b.contexts))
	for _, b := range b.contexts {
		contexts = append(contexts, b)
	}
	return contexts
}

func (b *Browser) IsConnected() bool {
	b.connMu.RLock()
	defer b.connMu.RUnlock()
	return b.connected
}

// NewContext creates a new incognito-like browser context
func (b *Browser) NewContext(opts goja.Value) api.BrowserContext {
	action := target.CreateBrowserContext().WithDisposeOnDetach(true)
	browserContextID, err := action.Do(cdp.WithExecutor(b.ctx, b.conn))
	b.logger.Infof("Browser:NewContext", "browserContextID: %v", browserContextID)
	if err != nil {
		k6Throw(b.ctx, "unable to execute %T: %w", action, err)
	}

	browserCtxOpts := NewBrowserContextOptions()
	if err := browserCtxOpts.Parse(b.ctx, opts); err != nil {
		k6Throw(b.ctx, "failed parsing options: %w", err)
	}

	b.contextsMu.Lock()
	defer b.contextsMu.Unlock()
	b.logger.Infof("Browser:NewContext", "NewBrowserContext: %v", browserContextID)
	browserCtx := NewBrowserContext(b.ctx, b.conn, b, browserContextID, browserCtxOpts, b.logger)
	b.contexts[browserContextID] = browserCtx

	return browserCtx
}

// NewPage creates a new tab in the browser window
func (b *Browser) NewPage(opts goja.Value) api.Page {
	browserCtx := b.NewContext(opts)
	b.logger.Infof("Browser:NewContext", "NewPage")
	return browserCtx.NewPage()
}

// UserAgent returns the controlled browser's user agent string
func (b *Browser) UserAgent() string {
	action := cdpbrowser.GetVersion()
	_, _, _, ua, _, err := action.Do(cdp.WithExecutor(b.ctx, b.conn))
	if err != nil {
		k6Throw(b.ctx, "unable to get browser user agent: %w", err)
	}
	return ua
}

// Version returns the controlled browser's version
func (b *Browser) Version() string {
	action := cdpbrowser.GetVersion()
	_, product, _, _, _, err := action.Do(cdp.WithExecutor(b.ctx, b.conn))
	if err != nil {
		k6Throw(b.ctx, "unable to get browser version: %w", err)
	}
	i := strings.Index(product, "/")
	if i == -1 {
		return product
	}
	return product[i+1:]
}
