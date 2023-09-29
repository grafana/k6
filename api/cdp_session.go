package api

import "github.com/dop251/goja"

// CDPSessionAPI is the interface of a raw CDP session.
type CDPSessionAPI interface {
	Detach()
	Send(method string, params goja.Value) goja.Value
}
