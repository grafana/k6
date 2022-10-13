package api

import (
	"github.com/dop251/goja"
)

// BrowserContext is the public interface of a CDP browser context.
type BrowserContext interface {
	AddCookies(cookies goja.Value) *goja.Promise
	AddInitScript(script goja.Value, arg goja.Value)
	Browser() Browser
	ClearCookies()
	ClearPermissions()
	Close()
	Cookies() *goja.Promise
	ExposeBinding(name string, callback goja.Callable, opts goja.Value) *goja.Promise
	ExposeFunction(name string, callback goja.Callable) *goja.Promise
	GrantPermissions(permissions []string, opts goja.Value)
	NewCDPSession() *goja.Promise
	NewPage() Page
	Pages() []Page
	Route(url goja.Value, handler goja.Callable) *goja.Promise
	SetDefaultNavigationTimeout(timeout int64)
	SetDefaultTimeout(timeout int64)
	SetExtraHTTPHeaders(headers map[string]string) *goja.Promise
	SetGeolocation(geolocation goja.Value)
	// SetHTTPCredentials sets username/password credentials to use for HTTP authentication.
	//
	// Deprecated: Create a new BrowserContext with httpCredentials instead.
	// See for details:
	// - https://github.com/microsoft/playwright/issues/2196#issuecomment-627134837
	// - https://github.com/microsoft/playwright/pull/2763
	SetHTTPCredentials(httpCredentials goja.Value)
	SetOffline(offline bool)
	StorageState(opts goja.Value) *goja.Promise
	Unroute(url goja.Value, handler goja.Callable) *goja.Promise
	WaitForEvent(event string, optsOrPredicate goja.Value) interface{}
}
