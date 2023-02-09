package api

import "github.com/dop251/goja"

// CDPSession is the interface of a raw CDP session.
type CDPSession interface {
	Detach()
	Send(method string, params goja.Value) goja.Value
}
