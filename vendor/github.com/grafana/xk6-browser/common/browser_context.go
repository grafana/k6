package common

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/k6ext"
	"github.com/grafana/xk6-browser/log"

	k6modules "go.k6.io/k6/js/modules"

	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/cdproto/target"
	"github.com/dop251/goja"
)

// Ensure BrowserContext implements the EventEmitter and api.BrowserContext interfaces.
var _ EventEmitter = &BrowserContext{}
var _ api.BrowserContext = &BrowserContext{}

// BrowserContext stores context information for a single independent browser session.
// A newly launched browser instance contains a default browser context.
// Any browser context created aside from the default will be considered an "incognito"
// browser context and will not store any data on disk.
type BrowserContext struct {
	BaseEventEmitter

	ctx             context.Context
	browser         *Browser
	id              cdp.BrowserContextID
	opts            *BrowserContextOptions
	timeoutSettings *TimeoutSettings
	logger          *log.Logger
	vu              k6modules.VU

	evaluateOnNewDocumentSources []string
}

// NewBrowserContext creates a new browser context.
func NewBrowserContext(
	ctx context.Context, browser *Browser, id cdp.BrowserContextID, opts *BrowserContextOptions, logger *log.Logger,
) *BrowserContext {
	b := BrowserContext{
		BaseEventEmitter: NewBaseEventEmitter(ctx),
		ctx:              ctx,
		browser:          browser,
		id:               id,
		opts:             opts,
		logger:           logger,
		vu:               k6ext.GetVU(ctx),
		timeoutSettings:  NewTimeoutSettings(nil),
	}

	if opts != nil && len(opts.Permissions) > 0 {
		b.GrantPermissions(opts.Permissions, nil)
	}

	return &b
}

// AddCookies is not implemented.
func (b *BrowserContext) AddCookies(cookies goja.Value) {
	k6ext.Panic(b.ctx, "BrowserContext.addCookies(cookies) has not been implemented yet")
}

// AddInitScript adds a script that will be initialized on all new pages.
func (b *BrowserContext) AddInitScript(script goja.Value, arg goja.Value) {
	b.logger.Debugf("BrowserContext:AddInitScript", "bctxid:%v", b.id)

	rt := b.vu.Runtime()

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
		k6ext.Panic(b.ctx, "clearing cookies: %w", err)
	}
}

// ClearPermissions clears any permission overrides.
func (b *BrowserContext) ClearPermissions() {
	b.logger.Debugf("BrowserContext:ClearPermissions", "bctxid:%v", b.id)

	action := cdpbrowser.ResetPermissions().WithBrowserContextID(b.id)
	if err := action.Do(b.ctx); err != nil {
		k6ext.Panic(b.ctx, "clearing permissions: %w", err)
	}
}

// Close shuts down the browser context.
func (b *BrowserContext) Close() {
	b.logger.Debugf("BrowserContext:Close", "bctxid:%v", b.id)

	if b.id == "" {
		k6ext.Panic(b.ctx, "default browser context can't be closed")
	}
	if err := b.browser.disposeContext(b.id); err != nil {
		k6ext.Panic(b.ctx, "disposing browser context: %w", err)
	}
}

// Cookies is not implemented.
func (b *BrowserContext) Cookies() []any {
	k6ext.Panic(b.ctx, "BrowserContext.cookies() has not been implemented yet")
	return nil
}

// ExposeBinding is not implemented.
func (b *BrowserContext) ExposeBinding(name string, callback goja.Callable, opts goja.Value) {
	k6ext.Panic(b.ctx, "BrowserContext.exposeBinding(name, callback, opts) has not been implemented yet")
}

// ExposeFunction is not implemented.
func (b *BrowserContext) ExposeFunction(name string, callback goja.Callable) {
	k6ext.Panic(b.ctx, "BrowserContext.exposeFunction(name, callback) has not been implemented yet")
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

	rt := b.vu.Runtime()
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			if k == "origin" {
				origin = opts.Get(k).String()
				break
			}
		}
	}

	perms := make([]cdpbrowser.PermissionType, 0, len(permissions))
	for _, p := range permissions {
		perms = append(perms, permsToProtocol[p])
	}

	action := cdpbrowser.GrantPermissions(perms).WithOrigin(origin).WithBrowserContextID(b.id)
	if err := action.Do(cdp.WithExecutor(b.ctx, b.browser.conn)); err != nil {
		k6ext.Panic(b.ctx, "internal error while granting browser permissions: %w", err)
	}
}

// NewCDPSession returns a new CDP session attached to this target.
func (b *BrowserContext) NewCDPSession() api.CDPSession {
	k6ext.Panic(b.ctx, "BrowserContext.newCDPSession() has not been implemented yet")
	return nil
}

// NewPage creates a new page inside this browser context.
func (b *BrowserContext) NewPage() api.Page {
	b.logger.Debugf("BrowserContext:NewPage", "bctxid:%v", b.id)

	p, err := b.browser.newPageInContext(b.id)
	if err != nil {
		k6ext.Panic(b.ctx, "newPageInContext: %w", err)
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

// Route is not implemented.
func (b *BrowserContext) Route(url goja.Value, handler goja.Callable) {
	k6ext.Panic(b.ctx, "BrowserContext.route(url, handler) has not been implemented yet")
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

// SetExtraHTTPHeaders is not implemented.
func (b *BrowserContext) SetExtraHTTPHeaders(headers map[string]string) {
	k6ext.Panic(b.ctx, "BrowserContext.setExtraHTTPHeaders(headers) has not been implemented yet")
}

// SetGeolocation overrides the geo location of the user.
func (b *BrowserContext) SetGeolocation(geolocation goja.Value) {
	b.logger.Debugf("BrowserContext:SetGeolocation", "bctxid:%v", b.id)

	g := NewGeolocation()
	if err := g.Parse(b.ctx, geolocation); err != nil {
		k6ext.Panic(b.ctx, "parsing geo location: %v", err)
	}

	b.opts.Geolocation = g
	for _, p := range b.browser.getPages() {
		if err := p.updateGeolocation(); err != nil {
			k6ext.Panic(b.ctx, "updating geo location in target ID %s: %w", p.targetID, err)
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
		k6ext.Panic(b.ctx, "setting HTTP credentials: %w", err)
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

// StorageState is not implemented.
func (b *BrowserContext) StorageState(opts goja.Value) {
	k6ext.Panic(b.ctx, "BrowserContext.storageState(opts) has not been implemented yet")
}

// Unroute is not implemented.
func (b *BrowserContext) Unroute(url goja.Value, handler goja.Callable) {
	k6ext.Panic(b.ctx, "BrowserContext.unroute(url, handler) has not been implemented yet")
}

// WaitForEvent waits for event.
func (b *BrowserContext) WaitForEvent(event string, optsOrPredicate goja.Value) any {
	// TODO: This public API needs Promise support (as return value) to be useful in JS!
	b.logger.Debugf("BrowserContext:WaitForEvent", "bctxid:%v event:%q", b.id, event)

	var (
		isCallable  bool
		predicateFn goja.Callable
		// TODO: Find out whether * time.Second is necessary.
		timeout = b.browser.launchOpts.Timeout * time.Second //nolint:durationcheck
	)
	if gojaValueExists(optsOrPredicate) {
		switch optsOrPredicate.ExportType() {
		case reflect.TypeOf(goja.Object{}):
			opts := optsOrPredicate.ToObject(b.vu.Runtime())
			for _, k := range opts.Keys() {
				switch k {
				case "predicate":
					predicateFn, isCallable = goja.AssertFunction(opts.Get(k))
					if !isCallable {
						k6ext.Panic(b.ctx, "predicate function is not callable")
					}
				case "timeout":
					timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
				}
			}
		default:
			predicateFn, isCallable = goja.AssertFunction(optsOrPredicate)
			if !isCallable {
				k6ext.Panic(b.ctx, "predicate function is not callable")
			}
		}
	}

	return b.waitForEvent(event, predicateFn, timeout)
}

func (b *BrowserContext) waitForEvent(event string, predicateFn goja.Callable, timeout time.Duration) any {
	evCancelCtx, evCancelFn := context.WithCancel(b.ctx)
	chEvHandler := make(chan Event)
	ch := make(chan any)

	go b.runWaitForEventHandler(evCancelCtx, evCancelFn, chEvHandler, ch, predicateFn)

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

func (b *BrowserContext) runWaitForEventHandler(
	ctx context.Context, evCancelFn func(), chEvHandler chan Event, out chan any, predicateFn goja.Callable,
) {
	b.logger.Debugf("BrowserContext:WaitForEvent:go():starts", "bctxid:%v", b.id)
	defer b.logger.Debugf("BrowserContext:WaitForEvent:go():returns", "bctxid:%v", b.id)
	for {
		select {
		case <-ctx.Done():
			b.logger.Debugf("BrowserContext:WaitForEvent:go():ctx:done", "bctxid:%v", b.id)
			return
		case ev := <-chEvHandler:
			if ev.typ == EventBrowserContextClose {
				b.logger.Debugf("BrowserContext:WaitForEvent:go():EventBrowserContextClose:return", "bctxid:%v", b.id)
				out <- nil
				close(out)

				// We wait for one matching event only,
				// then remove event handler by cancelling context and stopping goroutine.
				evCancelFn()
				return
			}
			if ev.typ == EventBrowserContextPage {
				b.logger.Debugf("BrowserContext:WaitForEvent:go():EventBrowserContextPage", "bctxid:%v", b.id)
				p, _ := ev.data.(*Page)
				if retVal, err := predicateFn(b.vu.Runtime().ToValue(p)); err == nil && retVal.ToBoolean() {
					b.logger.Debugf("BrowserContext:WaitForEvent:go():EventBrowserContextPage:return", "bctxid:%v", b.id)
					out <- p
					close(out)

					// We wait for one matching event only,
					// then remove event handler by cancelling context and stopping goroutine.
					evCancelFn()
					return
				}
			}
		}
	}
}

func (b *BrowserContext) getSession(id target.SessionID) *Session {
	return b.browser.conn.getSession(id)
}
