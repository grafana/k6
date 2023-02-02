package api

import (
	"github.com/dop251/goja"
)

// BrowserType is the public interface of a CDP browser client.
type BrowserType interface {
	Connect(opts goja.Value)
	ExecutablePath() string
	Launch(opts goja.Value) Browser
	LaunchPersistentContext(userDataDir string, opts goja.Value) Browser
	Name() string
}
