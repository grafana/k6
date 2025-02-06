package common

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/cdproto/target"

	"go.k6.io/k6/internal/js/modules/k6/browser/common/js"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6error"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/internal/js/modules/k6/browser/log"

	k6modules "go.k6.io/k6/js/modules"
)

// waitForEventType represents the event types that can be used when working
// with the browserContext.waitForEvent API.
type waitForEventType string

// Cookie represents a browser cookie.
//
// https://datatracker.ietf.org/doc/html/rfc6265.
type Cookie struct {
	Name     string         `js:"name" json:"name"`         // Cookie name.
	Value    string         `js:"value" json:"value"`       // Cookie value.
	Domain   string         `js:"domain" json:"domain"`     // Cookie domain.
	Path     string         `js:"path" json:"path"`         // Cookie path.
	HTTPOnly bool           `js:"httpOnly" json:"httpOnly"` // True if cookie is http-only.
	Secure   bool           `js:"secure" json:"secure"`     // True if cookie is secure.
	SameSite CookieSameSite `js:"sameSite" json:"sameSite"` // Cookie SameSite type.
	URL      string         `js:"url" json:"url,omitempty"` // Cookie URL.
	// Cookie expiration date as the number of seconds since the UNIX epoch.
	Expires int64 `js:"expires" json:"expires"`
}

// CookieSameSite represents the cookie's 'SameSite' status.
//
// https://tools.ietf.org/html/draft-west-first-party-cookies.
type CookieSameSite string

const (
	// CookieSameSiteStrict sets the cookie to be sent only in a first-party
	// context and not be sent along with requests initiated by third party
	// websites.
	CookieSameSiteStrict CookieSameSite = "Strict"

	// CookieSameSiteLax sets the cookie to be sent along with "same-site"
	// requests, and with "cross-site" top-level navigations.
	CookieSameSiteLax CookieSameSite = "Lax"

	// CookieSameSiteNone sets the cookie to be sent in all contexts, i.e
	// potentially insecure third-party requests.
	CookieSameSiteNone CookieSameSite = "None"
)

const (
	// waitForEventTypePage represents the page event which fires when a new
	// page is created.
	waitForEventTypePage = "page"
)

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

	// DownloadsPath is the path where downloads will be stored.
	DownloadsPath string
}

// artifactsDirectory is the prefix for the temporary directory created for downloads.
const artifactsDirectory = "k6browser-artifacts-"

// setDownloadsPath sets the downloads path.
// If the provided path is empty, a temporary directory with
// an artifactsDirectory prefix will be created.
func (b *BrowserContext) setDownloadsPath(path string) error {
	path = strings.TrimSpace(path)
	if path != "" {
		b.DownloadsPath = path
		return nil
	}
	dir, err := os.MkdirTemp(os.TempDir(), artifactsDirectory+"*") //nolint:forbidigo
	if err != nil {
		return fmt.Errorf("creating temporary directory for downloads: %w", err)
	}
	b.DownloadsPath = dir

	return nil
}

// cleanup cleans up the resources associated with the browser context.
func (b *BrowserContext) cleanup() error {
	if err := os.RemoveAll(b.DownloadsPath); err != nil { //nolint:forbidigo
		return fmt.Errorf("removing downloads path: %w", err)
	}
	b.DownloadsPath = ""

	return nil
}

// NewBrowserContext creates a new browser context.
func NewBrowserContext(
	ctx context.Context, browser *Browser, id cdp.BrowserContextID, opts *BrowserContextOptions, logger *log.Logger,
) (*BrowserContext, error) {
	// set the default options if none provided.
	if opts == nil {
		opts = DefaultBrowserContextOptions()
	}
	// Always use the [Browser]'s user agent if it's not set by the user.
	// Setting this forces [FrameSession] to set Chromium's user agent.
	if strings.TrimSpace(opts.UserAgent) == "" {
		opts.UserAgent = browser.UserAgent()
	}

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

	if len(opts.Permissions) > 0 {
		err := b.GrantPermissions(opts.Permissions, GrantPermissionsOptions{})
		if err != nil {
			return nil, err
		}
	}

	if err := b.AddInitScript(js.WebVitalIIFEScript); err != nil {
		return nil, fmt.Errorf("adding web vital script to new browser context: %w", err)
	}
	if err := b.AddInitScript(js.WebVitalInitScript); err != nil {
		return nil, fmt.Errorf("adding web vital init script to new browser context: %w", err)
	}
	if err := b.setDownloadsPath(opts.DownloadsPath); err != nil {
		return nil, fmt.Errorf("setting downloads path: %w", err)
	}

	return &b, nil
}

// AddInitScript adds a script that will be initialized on all new pages.
func (b *BrowserContext) AddInitScript(script string) error {
	b.logger.Debugf("BrowserContext:AddInitScript", "bctxid:%v", b.id)

	b.evaluateOnNewDocumentSources = append(b.evaluateOnNewDocumentSources, script)

	for _, p := range b.browser.getPages() {
		if err := p.evaluateOnNewDocument(script); err != nil {
			return fmt.Errorf("adding init script to browser context: %w", err)
		}
	}

	return nil
}

func (b *BrowserContext) applyAllInitScripts(p *Page) error {
	for _, source := range b.evaluateOnNewDocumentSources {
		if err := p.evaluateOnNewDocument(source); err != nil {
			return fmt.Errorf("adding init script to browser context: %w", err)
		}
	}

	return nil
}

// Browser returns the browser instance that this browser context belongs to.
func (b *BrowserContext) Browser() *Browser {
	return b.browser
}

// ClearPermissions clears any permission overrides.
func (b *BrowserContext) ClearPermissions() error {
	b.logger.Debugf("BrowserContext:ClearPermissions", "bctxid:%v", b.id)

	action := cdpbrowser.ResetPermissions().WithBrowserContextID(b.id)
	if err := action.Do(cdp.WithExecutor(b.ctx, b.browser.conn)); err != nil {
		return fmt.Errorf("clearing permissions: %w", err)
	}

	return nil
}

// Close shuts down the browser context.
func (b *BrowserContext) Close() error {
	b.logger.Debugf("BrowserContext:Close", "bctxid:%v", b.id)

	if b.id == "" {
		return fmt.Errorf("default browser context can't be closed")
	}
	if err := b.browser.disposeContext(b.id); err != nil {
		return fmt.Errorf("disposing browser context: %w", err)
	}
	return nil
}

// GrantPermissions enables the specified permissions, all others will be disabled.
func (b *BrowserContext) GrantPermissions(permissions []string, opts GrantPermissionsOptions) error {
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

	perms := make([]cdpbrowser.PermissionType, 0, len(permissions))
	for _, p := range permissions {
		proto, ok := permsToProtocol[p]
		if !ok {
			return fmt.Errorf("%q is an invalid permission", p)
		}
		perms = append(perms, proto)
	}

	action := cdpbrowser.GrantPermissions(perms).WithOrigin(opts.Origin).WithBrowserContextID(b.id)
	if err := action.Do(cdp.WithExecutor(b.ctx, b.browser.conn)); err != nil {
		return fmt.Errorf("granting browser permissions: %w", err)
	}

	return nil
}

// NewPage creates a new page inside this browser context.
func (b *BrowserContext) NewPage() (*Page, error) {
	b.logger.Debugf("BrowserContext:NewPage", "bctxid:%v", b.id)
	_, span := TraceAPICall(b.ctx, "", "browserContext.newPage")
	defer span.End()

	p, err := b.browser.newPageInContext(b.id)
	if err != nil {
		err := fmt.Errorf("creating new page in browser context: %w", err)
		spanRecordError(span, err)
		return nil, err
	}

	b.logger.Debugf("BrowserContext:NewPage:return", "bctxid:%v ptid:%s", b.id, p.targetID)
	return p, nil
}

// Pages returns a list of pages inside this browser context.
func (b *BrowserContext) Pages() []*Page {
	return append([]*Page{}, b.browser.getPages()...)
}

// SetDefaultNavigationTimeout sets the default navigation timeout in milliseconds.
func (b *BrowserContext) SetDefaultNavigationTimeout(timeout int64) {
	b.logger.Debugf("BrowserContext:SetDefaultNavigationTimeout", "bctxid:%v timeout:%d", b.id, timeout)

	b.timeoutSettings.setDefaultNavigationTimeout(time.Duration(timeout) * time.Millisecond)
}

// SetDefaultTimeout sets the default maximum timeout in milliseconds.
func (b *BrowserContext) SetDefaultTimeout(timeout int64) {
	b.logger.Debugf("BrowserContext:SetDefaultTimeout", "bctxid:%v timeout:%d", b.id, timeout)

	b.timeoutSettings.setDefaultTimeout(time.Duration(timeout) * time.Millisecond)
}

// SetGeolocation overrides the geo location of the user.
func (b *BrowserContext) SetGeolocation(g *Geolocation) error {
	b.logger.Debugf("BrowserContext:SetGeolocation", "bctxid:%v", b.id)

	if err := g.Validate(); err != nil {
		return fmt.Errorf("validating geo location: %w", err)
	}

	b.opts.Geolocation = g
	for _, p := range b.browser.getPages() {
		if err := p.updateGeolocation(); err != nil {
			return fmt.Errorf("updating geo location in target ID %s: %w", p.targetID, err)
		}
	}

	return nil
}

// SetHTTPCredentials sets username/password credentials to use for HTTP authentication.
//
// Deprecated: Create a new BrowserContext with httpCredentials instead.
// See for details:
// - https://github.com/microsoft/playwright/issues/2196#issuecomment-627134837
// - https://github.com/microsoft/playwright/pull/2763
func (b *BrowserContext) SetHTTPCredentials(hc Credentials) error {
	b.logger.Warnf("setHTTPCredentials", "setHTTPCredentials is deprecated."+
		" Create a new BrowserContext with httpCredentials instead.")
	b.logger.Debugf("BrowserContext:SetHTTPCredentials", "bctxid:%v", b.id)

	b.opts.HTTPCredentials = hc
	for _, p := range b.browser.getPages() {
		if err := p.updateHTTPCredentials(); err != nil {
			return fmt.Errorf("setting HTTP credentials in target ID %s: %w", p.targetID, err)
		}
	}

	return nil
}

// SetOffline toggles the browser's connectivity on/off.
func (b *BrowserContext) SetOffline(offline bool) error {
	b.logger.Debugf("BrowserContext:SetOffline", "bctxid:%v offline:%t", b.id, offline)

	b.opts.Offline = offline
	for _, p := range b.browser.getPages() {
		if err := p.updateOffline(); err != nil {
			return fmt.Errorf(
				"setting offline status to %t for the browser context ID %s: %w",
				offline, b.id, err,
			)
		}
	}

	return nil
}

// Timeout will return the default timeout or the one set by the user.
func (b *BrowserContext) Timeout() time.Duration {
	return b.timeoutSettings.timeout()
}

// WaitForEvent waits for event.
func (b *BrowserContext) WaitForEvent(event string, f func(p *Page) (bool, error), timeout time.Duration) (any, error) {
	b.logger.Debugf("BrowserContext:WaitForEvent", "bctxid:%v event:%q", b.id, event)

	return b.waitForEvent(waitForEventType(event), f, timeout)
}

func (b *BrowserContext) waitForEvent(
	event waitForEventType,
	predicateFn func(p *Page) (bool, error),
	timeout time.Duration,
) (any, error) {
	if event != waitForEventTypePage {
		return nil, fmt.Errorf("incorrect event %q, %q is the only event supported", event, waitForEventTypePage)
	}

	evCancelCtx, evCancelFn := context.WithCancel(b.ctx)
	defer evCancelFn() // This will remove the event handler once we return from here.

	chEvHandler := make(chan Event)
	ch := make(chan any)
	errCh := make(chan error)

	go b.runWaitForEventHandler(evCancelCtx, chEvHandler, predicateFn, ch, errCh)

	b.on(evCancelCtx, []string{EventBrowserContextPage}, chEvHandler)

	select {
	case <-b.ctx.Done():
		return nil, b.ctx.Err() //nolint:wrapcheck
	case <-time.After(timeout):
		b.logger.Debugf("BrowserContext:WaitForEvent:timeout", "bctxid:%v event:%q", b.id, event)
		return nil, fmt.Errorf("waitForEvent timed out after %v", timeout)
	case evData := <-ch:
		b.logger.Debugf("BrowserContext:WaitForEvent:evData", "bctxid:%v event:%q", b.id, event)
		return evData, nil
	case err := <-errCh:
		b.logger.Debugf("BrowserContext:WaitForEvent:err", "bctxid:%v event:%q, err:%v", b.id, event, err)
		return nil, err
	}
}

// runWaitForEventHandler can work with a nil predicateFn. If predicateFn is
// nil it will return the response straight away.
func (b *BrowserContext) runWaitForEventHandler(
	ctx context.Context,
	chEvHandler chan Event, predicateFn func(p *Page) (bool, error),
	out chan<- any, errOut chan<- error,
) {
	b.logger.Debugf("BrowserContext:runWaitForEventHandler:go():starts", "bctxid:%v", b.id)
	defer b.logger.Debugf("BrowserContext:runWaitForEventHandler:go():returns", "bctxid:%v", b.id)

	defer func() {
		close(out)
		close(errOut)
	}()

	for {
		select {
		case <-ctx.Done():
			b.logger.Debugf("BrowserContext:runWaitForEventHandler:go():ctx:done", "bctxid:%v", b.id)
			return
		case ev := <-chEvHandler:
			if ev.typ != EventBrowserContextPage {
				continue
			}

			b.logger.Debugf("BrowserContext:runWaitForEventHandler:go():EventBrowserContextPage", "bctxid:%v", b.id)
			p, ok := ev.data.(*Page)
			if !ok {
				errOut <- fmt.Errorf("on create page event failed to return a page: %w", k6error.ErrFatal)
				return
			}

			if predicateFn == nil {
				b.logger.Debugf("BrowserContext:runWaitForEventHandler:go():EventBrowserContextPage:return", "bctxid:%v", b.id)
				out <- p
				return
			}

			retVal, err := predicateFn(p)
			if err != nil {
				errOut <- fmt.Errorf("predicate function failed: %w", err)
				return
			}

			if retVal {
				b.logger.Debugf(
					"BrowserContext:runWaitForEventHandler:go():EventBrowserContextPage:predicateFn:return",
					"bctxid:%v", b.id,
				)
				out <- p
				return
			}
		}
	}
}

func (b *BrowserContext) getSession(id target.SessionID) *Session {
	return b.browser.conn.getSession(id)
}

// AddCookies adds cookies into this browser context.
// All pages within this context will have these cookies installed.
func (b *BrowserContext) AddCookies(cookies []*Cookie) error {
	b.logger.Debugf("BrowserContext:AddCookies", "bctxid:%v", b.id)

	// skip work if no cookies provided.
	if len(cookies) == 0 {
		return fmt.Errorf("no cookies provided")
	}

	cookiesToSet := make([]*network.CookieParam, 0, len(cookies))
	for _, c := range cookies {
		if c.Name == "" {
			return fmt.Errorf("cookie name must be set: %#v", c)
		}
		if c.Value == "" {
			return fmt.Errorf("cookie value must be set: %#v", c)
		}
		// if URL is not set, both Domain and Path must be provided
		if c.URL == "" && (c.Domain == "" || c.Path == "") {
			const msg = "if cookie URL is not provided, both domain and path must be specified: %#v"
			return fmt.Errorf(msg, c)
		}
		// calculate the cookie expiration date, session cookie if not set.
		var ts *cdp.TimeSinceEpoch
		if c.Expires > 0 {
			t := cdp.TimeSinceEpoch(time.Unix(c.Expires, 0))
			ts = &t
		}
		cookiesToSet = append(cookiesToSet, &network.CookieParam{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			URL:      c.URL,
			Expires:  ts,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
			SameSite: network.CookieSameSite(c.SameSite),
		})
	}

	setCookies := storage.
		SetCookies(cookiesToSet).
		WithBrowserContextID(b.id)
	if err := setCookies.Do(cdp.WithExecutor(b.ctx, b.browser.conn)); err != nil {
		return fmt.Errorf("cannot set cookies: %w", err)
	}

	return nil
}

// ClearCookies clears cookies.
func (b *BrowserContext) ClearCookies() error {
	b.logger.Debugf("BrowserContext:ClearCookies", "bctxid:%v", b.id)

	clearCookies := storage.
		ClearCookies().
		WithBrowserContextID(b.id)
	if err := clearCookies.Do(cdp.WithExecutor(b.ctx, b.browser.conn)); err != nil {
		return fmt.Errorf("clearing cookies: %w", err)
	}
	return nil
}

// Cookies returns all cookies.
// Some of them can be added with the AddCookies method and some of them are
// automatically taken from the browser context when it is created. And some of
// them are set by the page, i.e., using the Set-Cookie HTTP header or via
// JavaScript like document.cookie.
func (b *BrowserContext) Cookies(urls ...string) ([]*Cookie, error) {
	b.logger.Debugf("BrowserContext:Cookies", "bctxid:%v", b.id)

	// get cookies from this browser context.
	getCookies := storage.
		GetCookies().
		WithBrowserContextID(b.id)
	networkCookies, err := getCookies.Do(
		cdp.WithExecutor(b.ctx, b.browser.conn),
	)
	if err != nil {
		return nil, fmt.Errorf("retrieving cookies: %w", err)
	}
	// return if no cookies found so we don't have to needlessly convert them.
	// users can still work with cookies using the empty slice.
	// like this: cookies.length === 0.
	if len(networkCookies) == 0 {
		return nil, nil
	}

	// convert the received CDP cookies to the browser API format.
	cookies := make([]*Cookie, len(networkCookies))
	for i, c := range networkCookies {
		cookies[i] = &Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  int64(c.Expires),
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
			SameSite: CookieSameSite(c.SameSite),
		}
	}
	// filter cookies by the provided URLs.
	cookies, err = filterCookies(cookies, urls...)
	if err != nil {
		return nil, fmt.Errorf("filtering cookies: %w", err)
	}
	if len(cookies) == 0 {
		return nil, nil
	}

	return cookies, nil
}

// filterCookies filters the given cookies based on URLs.
// If an error occurs while parsing the cookie URLs, the error is returned.
func filterCookies(cookies []*Cookie, urls ...string) ([]*Cookie, error) {
	if len(urls) == 0 || len(cookies) == 0 {
		return cookies, nil
	}

	purls, err := parseURLs(urls...)
	if err != nil {
		return nil, fmt.Errorf("parsing urls: %w", err)
	}

	// the following algorithm is like a sorting algorithm,
	// but instead of sorting, it filters the cookies slice
	// in place, without allocating a new slice. this is
	// done to avoid unnecessary allocations and copying
	// of data.
	//
	// n is used to remember the last cookie that should be
	// kept in the cookies slice. all cookies before n should
	// be kept, all cookies after n should be removed. it
	// constantly shifts cookies to be kept to the left in the
	// slice, overwriting cookies that should be removed.
	//
	// if a cookie should not be kept, it will be overwritten
	// by the next cookie that should be kept. if no cookies
	// should be kept, a nil slice is returned. otherwise,
	// the slice is truncated to the last cookie that should
	// be kept.

	var n int

	for _, c := range cookies {
		var keep bool

		for _, uri := range purls {
			if shouldKeepCookie(c, uri) {
				keep = true
				break
			}
		}
		if !keep {
			continue
		}
		cookies[n] = c
		n++
	}
	// if no cookies should be kept, return nil instead of
	// an empty slice to conform with the API error behavior.
	// also makes tests concise.
	if n == 0 {
		return nil, nil
	}

	// remove all cookies after the last cookie that should be kept.
	return cookies[:n], nil
}

// shouldKeepCookie determines whether a cookie should be kept,
// based on its compatibility with a specific URL.
// Returns true if the cookie should be kept, false otherwise.
func shouldKeepCookie(c *Cookie, uri *url.URL) bool {
	// Ensure consistent domain formatting for easier comparison.
	// A leading dot means the cookie is valid across subdomains.
	// For example, if the domain is example.com, then adding a
	// dot turns it into .example.com, making the cookie valid
	// for sub.example.com, another.example.com, etc.
	domain := c.Domain
	if !strings.HasPrefix(domain, ".") {
		domain = "." + domain
	}
	// Confirm that the cookie's domain is a suffix of the URL's
	// hostname, emulating how a browser would scope cookies to
	// specific domains.
	if !strings.HasSuffix(domain, "."+uri.Hostname()) {
		return false
	}
	// Follow RFC 6265 for cookies: an empty or missing path should
	// be treated as "/".
	//
	// See: https://datatracker.ietf.org/doc/html/rfc6265#section-5.1.4
	path := c.Path
	if path == "" {
		path = "/"
	}
	// Ensure that the cookie applies to the specific path of the
	// URL, emulating how a browser would scope cookies to specific
	// paths within a domain.
	if !strings.HasPrefix(path, uri.Path) {
		return false
	}
	// Emulate browser behavior: Don't include secure cookies when
	// the scheme is not HTTPS, unless it's localhost.
	if uri.Scheme != "https" && uri.Hostname() != "localhost" && c.Secure {
		return false
	}

	// Keep the cookie.
	return true
}

// parseURLs parses the given URLs.
// If an error occurs while parsing a URL, the error is returned.
func parseURLs(urls ...string) ([]*url.URL, error) {
	purls := make([]*url.URL, len(urls))
	for i, u := range urls {
		uri, err := url.ParseRequestURI(
			strings.TrimSpace(u),
		)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", u, err)
		}
		purls[i] = uri
	}

	return purls, nil
}
