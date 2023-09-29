package api

import "github.com/dop251/goja"

// RouteAPI is the interface of a route for managing request interception.
type RouteAPI interface {
	Abort(errorCode string)
	Continue(opts goja.Value)
	Fulfill(opts goja.Value)
	Request() RequestAPI
}
