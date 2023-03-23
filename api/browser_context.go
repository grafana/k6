package api

import (
	"github.com/dop251/goja"
)

// BrowserContext is the public interface of a CDP browser context.
type BrowserContext interface {
	AddCookies(cookies goja.Value)
	AddInitScript(script goja.Value, arg goja.Value) error
	Browser() Browser
	ClearCookies()
	ClearPermissions()
	Close()
	Cookies() ([]any, error) // TODO: make it []Cookie later on
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
