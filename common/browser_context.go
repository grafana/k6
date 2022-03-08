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
	"reflect"
	"time"

	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/cdproto/target"
	"github.com/dop251/goja"
	k6common "go.k6.io/k6/js/common"

	"github.com/grafana/xk6-browser/api"
)

// Ensure BrowserContext implements the EventEmitter and api.BrowserContext interfaces.
var _ EventEmitter = &BrowserContext{}
var _ api.BrowserContext = &BrowserContext{}

// BrowserContext stores context information for a single independent browser session.
// A newly launched browser instance contains a default browser context.
// Any browser context created aside from the default will be considered an "ingognito"
// browser context and will not store any data on disk.
type BrowserContext struct {
	BaseEventEmitter

	ctx             context.Context
	browser         *Browser
	id              cdp.BrowserContextID
	opts            *BrowserContextOptions
	timeoutSettings *TimeoutSettings
	logger          *Logger

	evaluateOnNewDocumentSources []string
}

// NewBrowserContext creates a new browser context.
func NewBrowserContext(ctx context.Context, browser *Browser, id cdp.BrowserContextID, opts *BrowserContextOptions, logger *Logger) *BrowserContext {
	b := BrowserContext{
		BaseEventEmitter: NewBaseEventEmitter(ctx),
		ctx:              ctx,
		browser:          browser,
		id:               id,
		opts:             opts,
		logger:           logger,
		timeoutSettings:  NewTimeoutSettings(nil),
	}
	return &b
}

func (b *BrowserContext) AddCookies(cookies goja.Value) {
	k6Throw(b.ctx, "BrowserContext.addCookies(cookies) has not been implemented yet")
}

// AddInitScript adds a script that will be initialized on all new pages.
func (b *BrowserContext) AddInitScript(script goja.Value, arg goja.Value) {
	b.logger.Debugf("BrowserContext:AddInitScript", "bctxid:%v", b.id)

	rt := k6common.GetRuntime(b.ctx)

	source := ""
	if script != nil && !goja.IsUndefined(script) && !goja.IsNull(script) {
		switch script.ExportType() {
		case reflect.TypeOf(string("")):
			source = script.String()
		case reflect.TypeOf(goja.Object{}):
			opts := script.ToObject(rt)
			for _, k := range opts.Keys() {
				switch k {
				case "content":
					source = opts.Get(k).String()
				}
			}
		default:
			_, isCallable := goja.AssertFunction(script)
			if !isCallable {
				source = fmt.Sprintf("(%s);", script.ToString().String())
			} else {
				source = fmt.Sprintf("(%s)(...args);", script.ToString().String())
			}
		}
	}

	b.evaluateOnNewDocumentSources = append(b.evaluateOnNewDocumentSources, source)

	for _, p := range b.browser.getPages() {
		p.evaluateOnNewDocument(source)
	}
}

// Browser returns the browser instance that this browser context belongs to.
func (b *BrowserContext) Browser() api.Browser {
	return b.browser
}

// ClearCookies clears cookies.
func (b *BrowserContext) ClearCookies() {
	b.logger.Debugf("BrowserContext:ClearCookies", "bctxid:%v", b.id)

	action := storage.ClearCookies().WithBrowserContextID(b.id)
	if err := action.Do(b.ctx); err != nil {
		k6Throw(b.ctx, "unable to clear cookies permissions: %w", err)
	}
}

// ClearPermissions clears any permission overrides.
func (b *BrowserContext) ClearPermissions() {
	b.logger.Debugf("BrowserContext:ClearPermissions", "bctxid:%v", b.id)

	action := cdpbrowser.ResetPermissions().WithBrowserContextID(b.id)
	if err := action.Do(b.ctx); err != nil {
		k6Throw(b.ctx, "unable to clear override permissions: %w", err)
	}
}

// Close shuts down the browser context.
func (b *BrowserContext) Close() {
	b.logger.Debugf("BrowserContext:Close", "bctxid:%v", b.id)

	if b.id == "" {
		k6Throw(b.ctx, "default browser context can't be closed")
	}
	if err := b.browser.disposeContext(b.id); err != nil {
		k6Throw(b.ctx, "cannot dispose browser context: %w", err)
	}
}

func (b *BrowserContext) Cookies() []goja.Object {
	k6Throw(b.ctx, "BrowserContext.cookies() has not been implemented yet")
	return nil
}

func (b *BrowserContext) ExposeBinding(name string, callback goja.Callable, opts goja.Value) {
	k6Throw(b.ctx, "BrowserContext.exposeBinding(name, callback, opts) has not been implemented yet")
}

func (b *BrowserContext) ExposeFunction(name string, callback goja.Callable) {
	k6Throw(b.ctx, "BrowserContext.exposeFunction(name, callback) has not been implemented yet")
}

// GrantPermissions enables the specified permissions, all others will be disabled.
func (b *BrowserContext) GrantPermissions(permissions []string, opts goja.Value) {
	b.logger.Debugf("BrowserContext:GrantPermissions", "bctxid:%v", b.id)

	permsToProtocol := map[string]cdpbrowser.PermissionType{
		"geolocation":          cdpbrowser.PermissionTypeGeolocation,
		"midi":                 cdpbrowser.PermissionTypeMidi,
		"midi-sysex":           cdpbrowser.PermissionTypeMidiSysex,
		"notifications":        cdpbrowser.PermissionTypeNotifications,
		"camera":               cdpbrowser.PermissionTypeVideoCapture,
		"microphone":           cdpbrowser.PermissionTypeAudioCapture,
		"background-sync":      cdpbrowser.PermissionTypeBackgroundSync,
		"ambient-light-sensor": cdpbrowser.PermissionTypeSensors,
		"accelerometer":        cdpbrowser.PermissionTypeSensors,
		"gyroscope":            cdpbrowser.PermissionTypeSensors,
		"magnetometer":         cdpbrowser.PermissionTypeSensors,
		"accessibility-events": cdpbrowser.PermissionTypeAccessibilityEvents,
		"clipboard-read":       cdpbrowser.PermissionTypeClipboardReadWrite,
		"clipboard-write":      cdpbrowser.PermissionTypeClipboardSanitizedWrite,
		"payment-handler":      cdpbrowser.PermissionTypePaymentHandler,
	}
	origin := ""

	rt := k6common.GetRuntime(b.ctx)
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "origin":
				origin = opts.Get(k).String()
			}
		}
	}

	var perms []cdpbrowser.PermissionType
	for _, p := range permissions {
		perms = append(perms, permsToProtocol[p])
	}

	action := cdpbrowser.GrantPermissions(perms).WithOrigin(origin).WithBrowserContextID(b.id)
	if err := action.Do(b.ctx); err != nil {
		k6Throw(b.ctx, "unable to override permissions: %w", err)
	}
}

// NewCDPSession returns a new CDP session attached to this target.
func (b *BrowserContext) NewCDPSession() api.CDPSession {
	k6Throw(b.ctx, "BrowserContext.newCDPSession() has not been implemented yet")
	return nil
}

// NewPage creates a new page inside this browser context.
func (b *BrowserContext) NewPage() api.Page {
	b.logger.Debugf("BrowserContext:NewPage", "bctxid:%v", b.id)

	p, err := b.browser.newPageInContext(b.id)
	if err != nil {
		k6Throw(b.ctx, "newPageInContext: %w", err)
	}

	var (
		bctxid cdp.BrowserContextID
		ptid   target.ID
	)
	if b != nil {
		bctxid = b.id
	}
	if p != nil {
		ptid = p.targetID
	}
	b.logger.Debugf("BrowserContext:NewPage:return", "bctxid:%v ptid:%s", bctxid, ptid)

	return p
}

// Pages returns a list of pages inside this browser context.
func (b *BrowserContext) Pages() []api.Page {
	pages := make([]api.Page, 1)
	for _, p := range b.browser.getPages() {
		pages = append(pages, p)
	}
	return pages
}

func (b *BrowserContext) Route(url goja.Value, handler goja.Callable) {
	k6Throw(b.ctx, "BrowserContext.route(url, handler) has not been implemented yet")
}

// SetDefaultNavigationTimeout sets the default navigation timeout in milliseconds.
func (b *BrowserContext) SetDefaultNavigationTimeout(timeout int64) {
	b.logger.Debugf("BrowserContext:SetDefaultNavigationTimeout", "bctxid:%v timeout:%d", b.id, timeout)

	b.timeoutSettings.setDefaultNavigationTimeout(timeout)
}

// SetDefaultTimeout sets the default maximum timeout in milliseconds.
func (b *BrowserContext) SetDefaultTimeout(timeout int64) {
	b.logger.Debugf("BrowserContext:SetDefaultTimeout", "bctxid:%v timeout:%d", b.id, timeout)

	b.timeoutSettings.setDefaultTimeout(timeout)
}

func (b *BrowserContext) SetExtraHTTPHeaders(headers map[string]string) {
	k6Throw(b.ctx, "BrowserContext.setExtraHTTPHeaders(headers) has not been implemented yet")
}

// SetGeolocation overrides the geo location of the user.
func (b *BrowserContext) SetGeolocation(geolocation goja.Value) {
	b.logger.Debugf("BrowserContext:SetGeolocation", "bctxid:%v", b.id)

	g := NewGeolocation()
	if err := g.Parse(b.ctx, geolocation); err != nil {
		k6Throw(b.ctx, "cannot parse geo location: %v", err)
	}

	b.opts.Geolocation = g
	for _, p := range b.browser.getPages() {
		if err := p.updateGeolocation(); err != nil {
			k6Throw(b.ctx, "cannot update geo location in target (%s): %w", p.targetID, err)
		}
	}
}

// SetHTTPCredentials sets username/password credentials to use for HTTP authentication.
//
// Deprecated: Create a new BrowserContext with httpCredentials instead.
// See for details:
// - https://github.com/microsoft/playwright/issues/2196#issuecomment-627134837
// - https://github.com/microsoft/playwright/pull/2763
func (b *BrowserContext) SetHTTPCredentials(httpCredentials goja.Value) {
	b.logger.Warnf("setHTTPCredentials", "setHTTPCredentials is deprecated."+
		" Create a new BrowserContext with httpCredentials instead.")
	b.logger.Debugf("BrowserContext:SetHTTPCredentials", "bctxid:%v", b.id)

	c := NewCredentials()
	if err := c.Parse(b.ctx, httpCredentials); err != nil {
		k6Throw(b.ctx, "cannot set HTTP credentials: %w", err)
	}

	b.opts.HttpCredentials = c
	for _, p := range b.browser.getPages() {
		p.updateHttpCredentials()
	}
}

// SetOffline toggles the browser's connectivity on/off.
func (b *BrowserContext) SetOffline(offline bool) {
	b.logger.Debugf("BrowserContext:SetOffline", "bctxid:%v offline:%t", b.id, offline)

	b.opts.Offline = offline
	for _, p := range b.browser.getPages() {
		p.updateOffline()
	}
}

func (b *BrowserContext) StorageState(opts goja.Value) {
	k6Throw(b.ctx, "BrowserContext.storageState(opts) has not been implemented yet")
}

func (b *BrowserContext) Unroute(url goja.Value, handler goja.Callable) {
	k6Throw(b.ctx, "BrowserContext.unroute(url, handler) has not been implemented yet")
}

func (b *BrowserContext) WaitForEvent(event string, optsOrPredicate goja.Value) interface{} {
	// TODO: This public API needs Promise support (as return value) to be useful in JS!
	b.logger.Debugf("BrowserContext:WaitForEvent", "bctxid:%v event:%q", b.id, event)

	rt := k6common.GetRuntime(b.ctx)

	var isCallable bool
	var predicateFn goja.Callable = nil
	timeout := time.Duration(b.browser.launchOpts.Timeout * time.Second)

	if optsOrPredicate != nil && !goja.IsUndefined(optsOrPredicate) && !goja.IsNull(optsOrPredicate) {
		switch optsOrPredicate.ExportType() {
		case reflect.TypeOf(goja.Object{}):
			opts := optsOrPredicate.ToObject(rt)
			for _, k := range opts.Keys() {
				switch k {
				case "predicate":
					predicateFn, isCallable = goja.AssertFunction(opts.Get(k))
					if !isCallable {
						k6Throw(b.ctx, "expected callable predicate")
					}
				case "timeout":
					timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
				}
			}
		default:
			predicateFn, isCallable = goja.AssertFunction(optsOrPredicate)
			if !isCallable {
				k6Throw(b.ctx, "expected callable predicate")
			}
		}
	}

	evCancelCtx, evCancelFn := context.WithCancel(b.ctx)
	chEvHandler := make(chan Event)
	ch := make(chan interface{})

	go func() {
		b.logger.Debugf("BrowserContext:WaitForEvent:go():starts", "bctxid:%v", b.id)
		defer b.logger.Debugf("BrowserContext:WaitForEvent:go():returns", "bctxid:%v", b.id)
		for {
			select {
			case <-evCancelCtx.Done():
				b.logger.Debugf("BrowserContext:WaitForEvent:go():evCancelCtx:done", "bctxid:%v", b.id)
				return
			case ev := <-chEvHandler:
				if ev.typ == EventBrowserContextClose {
					b.logger.Debugf("BrowserContext:WaitForEvent:go():EventBrowserContextClose:return", "bctxid:%v", b.id)
					ch <- nil
					close(ch)

					// We wait for one matching event only,
					// then remove event handler by cancelling context and stopping goroutine.
					evCancelFn()
					return
				}
				if ev.typ == EventBrowserContextPage {
					b.logger.Debugf("BrowserContext:WaitForEvent:go():EventBrowserContextPage", "bctxid:%v", b.id)
					p := ev.data.(*Page)
					exported := k6common.Bind(rt, p, &b.ctx)
					if retVal, err := predicateFn(rt.ToValue(exported)); err == nil && retVal.ToBoolean() {
						b.logger.Debugf("BrowserContext:WaitForEvent:go():EventBrowserContextPage:return", "bctxid:%v", b.id)
						ch <- p
						close(ch)

						// We wait for one matching event only,
						// then remove event handler by cancelling context and stopping goroutine.
						evCancelFn()
						return
					}
				}
			}
		}
	}()

	b.on(evCancelCtx, []string{EventBrowserContextPage}, chEvHandler)
	defer evCancelFn() // Remove event handler

	select {
	case <-b.ctx.Done():
		b.logger.Debugf("BrowserContext:WaitForEvent:ctx.Done", "bctxid:%v event:%q", b.id, event)
	case <-time.After(timeout):
		b.logger.Debugf("BrowserContext:WaitForEvent:timeout", "bctxid:%v event:%q", b.id, event)
	case evData := <-ch:
		b.logger.Debugf("BrowserContext:WaitForEvent:evData", "bctxid:%v event:%q", b.id, event)
		return evData
	}
	b.logger.Debugf("BrowserContext:WaitForEvent:return nil", "bctxid:%v event:%q", b.id, event)
	return nil
}

func (b *BrowserContext) getSession(id target.SessionID) *Session {
	return b.browser.conn.getSession(id)
}
