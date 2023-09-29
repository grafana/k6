package api

import "github.com/dop251/goja"

// BrowserAPI is the public interface of a CDP browser.
type BrowserAPI interface {
	Close()
	Context() BrowserContextAPI
	IsConnected() bool
	NewContext(opts goja.Value) (BrowserContextAPI, error)
	NewPage(opts goja.Value) (Page, error)
	On(string) (bool, error)
	UserAgent() string
	Version() string
}
