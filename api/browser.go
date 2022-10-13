package api

import "github.com/dop251/goja"

// Browser is the public interface of a CDP browser.
type Browser interface {
	Close()
	Contexts() []BrowserContext
	IsConnected() bool
	NewContext(opts goja.Value) BrowserContext
	NewPage(opts goja.Value) Page
	On(string) *goja.Promise
	UserAgent() string
	Version() string
}
