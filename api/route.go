package api

import "github.com/dop251/goja"

// Route is the interface of a route for managing request interception.
type Route interface {
	Abort(errorCode string)
	Continue(opts goja.Value)
	Fulfill(opts goja.Value)
	Request() Request
}
