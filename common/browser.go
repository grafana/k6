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
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"golang.org/x/net/context"
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
	connected bool

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
	state := lib.GetState(ctx)
	reCategoryFilter, _ := regexp.Compile(launchOpts.LogCategoryFilter)
	b := Browser{
		BaseEventEmitter:      NewBaseEventEmitter(),
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
	var err error
	b.conn, err = NewConnection(b.ctx, b.browserProc.WsURL(), b.logger)
	if err != nil {
		return fmt.Errorf("unable to connect to browser WS URL: %w", err)
	}

	b.connected = true
	b.defaultContext = NewBrowserContext(b.ctx, b.conn, b, "", NewBrowserContextOptions(), b.logger)
	return b.initEvents()
}

func (b *Browser) disposeContext(id cdp.BrowserContextID) error {
	action := target.DisposeBrowserContext(id)
	if err := action.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
		return fmt.Errorf("unable to dispose browser context %T: %v", action, err)
	}
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
					go b.onAttachedToTarget(ev)
				} else if ev, ok := event.data.(*target.EventDetachedFromTarget); ok {
					go b.onDetachedFromTarget(ev)
				} else if event.typ == EventConnectionClose {
					b.connected = false
					b.browserProc.didLooseConnection()
					b.cancelFn()
				}
			}
		}
	}()

	action := target.SetAutoAttach(true, true).WithFlatten(true)
	if err := action.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
		return fmt.Errorf("unable to execute %T: %v", action, err)
	}

	// Target.setAutoAttach has a bug where it does not wait for new Targets being attached.
	// However making a dummy call afterwards fixes this.
	// This can be removed after https://chromium-review.googlesource.com/c/chromium/src/+/2885888 lands in stable.
	action2 := target.GetTargetInfo()
	if _, err := action2.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
		return fmt.Errorf("unable to execute %T: %v", action, err)
	}

	return nil
}

func (b *Browser) onAttachedToTarget(ev *target.EventAttachedToTarget) {
	var browserCtx *BrowserContext = b.defaultContext
	if b, ok := b.contexts[ev.TargetInfo.BrowserContextID]; ok {
		browserCtx = b
	}

	// We're not interested in the top-level browser target, other targets or DevTools targets right now.
	isDevTools := strings.HasPrefix(ev.TargetInfo.URL, "devtools://devtools")
	if ev.TargetInfo.Type == "browser" || ev.TargetInfo.Type == "other" || isDevTools {
		return
	}

	if ev.TargetInfo.Type == "background_page" {
		p, err := NewPage(b.ctx, b.conn.getSession(ev.SessionID), browserCtx, ev.TargetInfo.TargetID, nil, false)
		if err != nil {
			isRunning := b.state == BrowserStateOpen && b.IsConnected() //b.conn.isConnected()
			if _, ok := err.(*websocket.CloseError); !ok && !isRunning {
				// If we're no longer connected to browser, then ignore WebSocket errors
				return
			}
			rt := common.GetRuntime(b.ctx)
			common.Throw(rt, err)
		}
		b.pagesMu.Lock()
		b.pages[ev.TargetInfo.TargetID] = p
		b.pagesMu.Unlock()
		b.sessionIDtoTargetIDMu.Lock()
		b.sessionIDtoTargetID[ev.SessionID] = ev.TargetInfo.TargetID
		b.sessionIDtoTargetIDMu.Unlock()
	} else if ev.TargetInfo.Type == "page" {
		var opener *Page = nil
		if t, ok := b.pages[ev.TargetInfo.OpenerID]; ok {
			opener = t
		}
		p, err := NewPage(b.ctx, b.conn.getSession(ev.SessionID), browserCtx, ev.TargetInfo.TargetID, opener, true)
		if err != nil {
			isRunning := b.state == BrowserStateOpen && b.IsConnected() //b.conn.isConnected()
			if _, ok := err.(*websocket.CloseError); !ok && !isRunning {
				// If we're no longer connected to browser, then ignore WebSocket errors
				return
			}
			rt := common.GetRuntime(b.ctx)
			common.Throw(rt, err)
		}
		b.pagesMu.Lock()
		b.pages[ev.TargetInfo.TargetID] = p
		b.pagesMu.Unlock()
		b.sessionIDtoTargetIDMu.Lock()
		b.sessionIDtoTargetID[ev.SessionID] = ev.TargetInfo.TargetID
		b.sessionIDtoTargetIDMu.Unlock()
		browserCtx.emit(EventBrowserContextPage, p)
	}
}

func (b *Browser) onDetachedFromTarget(ev *target.EventDetachedFromTarget) {
	b.sessionIDtoTargetIDMu.RLock()
	targetID, ok := b.sessionIDtoTargetID[ev.SessionID]
	b.sessionIDtoTargetIDMu.RUnlock()
	if !ok {
		// We don't track targets of type "browser", "other" and "devtools", so ignore if we don't recognize target.
		return
	}

	if t, ok := b.pages[targetID]; ok {
		b.pagesMu.Lock()
		delete(b.pages, targetID)
		b.pagesMu.Unlock()
		t.didClose()
	}
}

func (b *Browser) newPageInContext(id cdp.BrowserContextID) (*Page, error) {
	browserCtx, ok := b.contexts[id]
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
			return data.(*Page).targetID == targetID
		},
	)
	defer evCancelFn() // Remove event handler
	errCh := make(chan error)
	func() {
		action := target.CreateTarget("about:blank").WithBrowserContextID(id)
		mu.Lock()
		defer mu.Unlock()
		if targetID, err = action.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
			errCh <- fmt.Errorf("unable to execute %T: %v", action, err)
		}
	}()
	select {
	case <-b.ctx.Done():
	case <-time.After(b.launchOpts.Timeout):
	case <-ch:
	case err := <-errCh:
		return nil, err
	}
	b.pagesMu.RLock()
	defer b.pagesMu.RUnlock()
	return b.pages[targetID], nil
}

// Close shuts down the browser
func (b *Browser) Close() {
	if !atomic.CompareAndSwapInt64(&b.state, b.state, BrowserStateClosing) {
		// If we're already in a closing state then no need to continue.
		return
	}
	b.browserProc.GracefulClose()
	defer b.browserProc.Terminate()

	action := cdpbrowser.Close()
	if err := action.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
		if _, ok := err.(*websocket.CloseError); !ok {
			rt := common.GetRuntime(b.ctx)
			common.Throw(rt, fmt.Errorf("unable to execute %T: %v", action, err))
		}
	}

	atomic.CompareAndSwapInt64(&b.state, b.state, BrowserStateClosed)
}

// Contexts returns list of browser contexts
func (b *Browser) Contexts() []api.BrowserContext {
	contexts := make([]api.BrowserContext, 0, len(b.contexts))
	for _, b := range b.contexts {
		contexts = append(contexts, b)
	}
	return contexts
}

func (b *Browser) IsConnected() bool {
	return b.connected
}

// NewContext creates a new incognito-like browser context
func (b *Browser) NewContext(opts goja.Value) api.BrowserContext {
	rt := common.GetRuntime(b.ctx)

	action := target.CreateBrowserContext().WithDisposeOnDetach(true)
	browserContextID, err := action.Do(cdp.WithExecutor(b.ctx, b.conn))
	if err != nil {
		common.Throw(rt, fmt.Errorf("unable to execute %T: %w", action, err))
	}

	browserCtxOpts := NewBrowserContextOptions()
	err = browserCtxOpts.Parse(b.ctx, opts)
	if err != nil {
		common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}

	browserCtx := NewBrowserContext(b.ctx, b.conn, b, browserContextID, browserCtxOpts, b.logger)
	b.contexts[browserContextID] = browserCtx

	return browserCtx
}

// NewPage creates a new tab in the browser window
func (b *Browser) NewPage(opts goja.Value) api.Page {
	browserCtx := b.NewContext(opts)
	return browserCtx.NewPage()
}

// UserAgent returns the controlled browser's user agent string
func (b *Browser) UserAgent() string {
	rt := common.GetRuntime(b.ctx)
	var userAgent string
	var err error

	action := cdpbrowser.GetVersion()
	if _, _, _, userAgent, _, err = action.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
		common.Throw(rt, fmt.Errorf("unable to get browser user agent: %w", err))
	}

	return userAgent
}

// Version returns the controlled browser's version
func (b *Browser) Version() string {
	rt := common.GetRuntime(b.ctx)
	var product string
	var err error

	action := cdpbrowser.GetVersion()
	if _, product, _, _, _, err = action.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
		common.Throw(rt, fmt.Errorf("unable to get browser version: %w", err))
	}

	i := strings.Index(product, "/")
	if i == -1 {
		return product
	}
	return product[i+1:]
}
