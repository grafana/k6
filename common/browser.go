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
func NewBrowser(ctx context.Context, cancelFn context.CancelFunc, browserProc *BrowserProcess, launchOpts *LaunchOptions, logger *Logger) (*Browser, error) {
	b := Browser{
		BaseEventEmitter:    NewBaseEventEmitter(ctx),
		ctx:                 ctx,
		cancelFn:            cancelFn,
		state:               int64(BrowserStateOpen),
		browserProc:         browserProc,
		launchOpts:          launchOpts,
		contexts:            make(map[cdp.BrowserContextID]*BrowserContext),
		pages:               make(map[target.ID]*Page),
		sessionIDtoTargetID: make(map[target.SessionID]target.ID),
		logger:              logger,
	}
	if err := b.connect(); err != nil {
		return nil, err
	}
	return &b, nil
}

func (b *Browser) connect() error {
	b.logger.Debugf("Browser:connect", "wsURL:%q", b.browserProc.WsURL())
	var err error
	b.conn, err = NewConnection(b.ctx, b.browserProc.WsURL(), b.logger)
	if err != nil {
		return fmt.Errorf("unable to connect to browser WS URL: %w", err)
	}

	b.connMu.Lock()
	b.connected = true
	b.connMu.Unlock()

	// We don't need to lock this because `connect()` is called only in NewBrowser
	b.defaultContext = NewBrowserContext(b.ctx, b.conn, b, "", NewBrowserContextOptions(), b.logger)

	return b.initEvents()
}

func (b *Browser) disposeContext(id cdp.BrowserContextID) error {
	b.logger.Debugf("Browser:disposeContext", "bctxid:%v", id)

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
		for {
			select {
			case <-cancelCtx.Done():
				return
			case event := <-chHandler:
				if ev, ok := event.data.(*target.EventAttachedToTarget); ok {
					b.logger.Debugf("Browser:initEvents:onAttachedToTarget", "sid:%v tid:%v", ev.SessionID, ev.TargetInfo.TargetID)
					go b.onAttachedToTarget(ev)
				} else if ev, ok := event.data.(*target.EventDetachedFromTarget); ok {
					b.logger.Debugf("Browser:initEvents:onDetachedFromTarget", "sid:%v", ev.SessionID)
					go b.onDetachedFromTarget(ev)
				} else if event.typ == EventConnectionClose {
					b.logger.Debugf("Browser:initEvents:EventConnectionClose", "")

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

	switch evti.Type {
	case "background_page":
		p, err := NewPage(b.ctx, b.conn.getSession(ev.SessionID), browserCtx, evti.TargetID, nil, false, b.logger)
		if err != nil {
			isRunning := atomic.LoadInt64(&b.state) == BrowserStateOpen && b.IsConnected() //b.conn.isConnected()
			if _, ok := err.(*websocket.CloseError); !ok && !isRunning {
				// If we're no longer connected to browser, then ignore WebSocket errors
				b.logger.Debugf("Browser:onAttachedToTarget:background_page:return", "sid:%v tid:%v websocket err:%v",
					ev.SessionID, evti.TargetID, err)
				return
			}
			k6Throw(b.ctx, "cannot create NewPage for background_page event: %w", err)
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
		var opener *Page = nil
		b.pagesMu.RLock()
		if t, ok := b.pages[evti.OpenerID]; ok {
			opener = t
		}
		b.logger.Debugf("Browser:onAttachedToTarget:page", "sid:%v tid:%v opener nil:%t", ev.SessionID, evti.TargetID, opener == nil)
		b.pagesMu.RUnlock()

		p, err := NewPage(b.ctx, b.conn.getSession(ev.SessionID), browserCtx, evti.TargetID, opener, true, b.logger)
		if err != nil {
			isRunning := atomic.LoadInt64(&b.state) == BrowserStateOpen && b.IsConnected() //b.conn.isConnected()
			if _, ok := err.(*websocket.CloseError); !ok && !isRunning {
				// If we're no longer connected to browser, then ignore WebSocket errors
				b.logger.Debugf("Browser:onAttachedToTarget:page:return", "sid:%v tid:%v websocket err:", ev.SessionID, evti.TargetID)
				return
			}
			k6Throw(b.ctx, "cannot create NewPage for page event: %w", err)
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

func (b *Browser) onDetachedFromTarget(ev *target.EventDetachedFromTarget) {
	b.sessionIDtoTargetIDMu.RLock()
	targetID, ok := b.sessionIDtoTargetID[ev.SessionID]

	b.logger.Debugf("Browser:onDetachedFromTarget", "sid:%v tid:%v", ev.SessionID, targetID)
	defer b.logger.Debugf("Browser:onDetachedFromTarget:return", "sid:%v tid:%v", ev.SessionID, targetID)

	b.sessionIDtoTargetIDMu.RUnlock()
	if !ok {
		// We don't track targets of type "browser", "other" and "devtools", so ignore if we don't recognize target.
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
		return nil, fmt.Errorf("no browser context with ID %s exists", id)
	}

	var (
		mu            sync.RWMutex // protects targetID
		targetID      target.ID
		localTargetID target.ID // sync free access for logging

		err error
	)
	ch, evCancelFn := createWaitForEventHandler(
		b.ctx, browserCtx, []string{EventBrowserContextPage},
		func(data interface{}) bool {
			mu.RLock()
			defer mu.RUnlock()
			b.logger.Debugf("Browser:newPageInContext:createWaitForEventHandler", "tid:%v bctxid:%v", targetID, id)
			return data.(*Page).targetID == targetID
		},
	)
	defer evCancelFn() // Remove event handler
	errCh := make(chan error)
	func() {
		action := target.CreateTarget("about:blank").WithBrowserContextID(id)
		mu.Lock()
		defer mu.Unlock()
		localTargetID = targetID
		b.logger.Debugf("Browser:newPageInContext:CreateTargetBlank", "tid:%v bctxid:%v", localTargetID, id)
		if targetID, err = action.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
			errCh <- fmt.Errorf("unable to execute %T: %w", action, err)
		}
	}()
	select {
	case <-b.ctx.Done():
		b.logger.Debugf("Browser:newPageInContext:<-b.ctx.Done", "tid:%v bctxid:%v", localTargetID, id)
	case <-time.After(b.launchOpts.Timeout):
		b.logger.Debugf("Browser:newPageInContext:timeout", "tid:%v bctxid:%v timeout:%s", localTargetID, id, b.launchOpts.Timeout)
	case c := <-ch:
		b.logger.Debugf("Browser:newPageInContext:<-ch", "tid:%v bctxid:%v, c:%v", localTargetID, id, c)
	case err := <-errCh:
		b.logger.Debugf("Browser:newPageInContext:<-errCh", "tid:%v bctxid:%v, err:%v", localTargetID, id, err)
		return nil, err
	}
	b.pagesMu.RLock()
	defer b.pagesMu.RUnlock()
	return b.pages[targetID], nil
}

// Close shuts down the browser
func (b *Browser) Close() {
	b.logger.Debugf("Browser:Close", "")
	if !atomic.CompareAndSwapInt64(&b.state, b.state, BrowserStateClosing) {
		// If we're already in a closing state then no need to continue.
		b.logger.Debugf("Browser:Close", "already in a closing state")
		return
	}

	atomic.CompareAndSwapInt64(&b.state, b.state, BrowserStateClosed)

	action := cdpbrowser.Close()
	if err := action.Do(cdp.WithExecutor(b.ctx, b.conn)); err != nil {
		if _, ok := err.(*websocket.CloseError); !ok {
			k6Throw(b.ctx, "unable to execute %T: %v", action, err)
		}
	}

	// terminate the browser process early on, then tell the CDP
	// afterwards. this will take a little bit of time, and CDP
	// will stop emitting events.
	b.browserProc.GracefulClose()
	b.browserProc.Terminate()
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
	b.logger.Debugf("Browser:NewContext", "bctxid:%v", browserContextID)
	if err != nil {
		k6Throw(b.ctx, "cannot create browser context (%s): %w", browserContextID, err)
	}

	browserCtxOpts := NewBrowserContextOptions()
	if err := browserCtxOpts.Parse(b.ctx, opts); err != nil {
		k6Throw(b.ctx, "failed parsing options: %w", err)
	}

	b.contextsMu.Lock()
	defer b.contextsMu.Unlock()
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
