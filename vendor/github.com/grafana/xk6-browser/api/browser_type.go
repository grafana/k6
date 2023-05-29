package api

import (
	"github.com/dop251/goja"
)

// BrowserType is the public interface of a CDP browser client.
type BrowserType interface {
	Connect(wsEndpoint string) (Browser, error)
	ExecutablePath() string
	Launch() (_ Browser, browserProcessID int, _ error)
	LaunchPersistentContext(userDataDir string, opts goja.Value) Browser
	Name() string
}
