package api

import (
	"github.com/dop251/goja"
)

// BrowserContext is the public interface of a CDP browser context.
type BrowserContext interface {
	AddCookies(cookies []*Cookie) error
	AddInitScript(script goja.Value, arg goja.Value) error
	Browser() Browser
	ClearCookies() error
	ClearPermissions()
	Close()
	Cookies(urls ...string) ([]*Cookie, error)
	ExposeBinding(name string, callback goja.Callable, opts goja.Value)
	ExposeFunction(name string, callback goja.Callable)
	GrantPermissions(permissions []string, opts goja.Value)
	NewCDPSession() CDPSession
	NewPage() (Page, error)
	Pages() []Page
	Route(url goja.Value, handler goja.Callable)
	SetDefaultNavigationTimeout(timeout int64)
	SetDefaultTimeout(timeout int64)
	SetExtraHTTPHeaders(headers map[string]string) error
	SetGeolocation(geolocation goja.Value)
	// SetHTTPCredentials sets username/password credentials to use for HTTP authentication.
	//
	// Deprecated: Create a new BrowserContext with httpCredentials instead.
	// See for details:
	// - https://github.com/microsoft/playwright/issues/2196#issuecomment-627134837
	// - https://github.com/microsoft/playwright/pull/2763
	SetHTTPCredentials(httpCredentials goja.Value)
	SetOffline(offline bool)
	StorageState(opts goja.Value)
	Unroute(url goja.Value, handler goja.Callable)
	WaitForEvent(event string, optsOrPredicate goja.Value) any
}

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
