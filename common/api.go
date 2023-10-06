package common

import (
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
)

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

// JSHandleAPI is the interface of an in-page JS object.
type JSHandleAPI interface {
	AsElement() *ElementHandle
	Dispose()
	Evaluate(pageFunc goja.Value, args ...goja.Value) any
	EvaluateHandle(pageFunc goja.Value, args ...goja.Value) (JSHandleAPI, error)
	GetProperties() (map[string]JSHandleAPI, error)
	GetProperty(propertyName string) JSHandleAPI
	JSONValue() goja.Value
	ObjectID() cdpruntime.RemoteObjectID
}
