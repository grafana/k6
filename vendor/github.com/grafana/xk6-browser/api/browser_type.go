package api

import (
	"github.com/dop251/goja"
)

// BrowserType is the public interface of a CDP browser client.
type BrowserType interface {
	Connect(wsEndpoint string, opts goja.Value) Browser
	ExecutablePath() string
	Launch(opts goja.Value) (_ Browser, browserProcessID int)
	LaunchPersistentContext(userDataDir string, opts goja.Value) Browser
	Name() string
}
