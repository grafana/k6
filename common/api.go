package common

import (
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
)

// JSHandleAPI is the interface of an in-page JS object.
//
// TODO: Find a way to move this to a concrete type. It's too difficult to
// do that right now because of the tests and the way we're using the
// JSHandleAPI interface.
type JSHandleAPI interface {
	AsElement() *ElementHandle
	Dispose()
	Evaluate(pageFunc goja.Value, args ...goja.Value) any
	EvaluateHandle(pageFunc goja.Value, args ...goja.Value) (JSHandleAPI, error)
	GetProperties() (map[string]JSHandleAPI, error)
	GetProperty(propertyName string) JSHandleAPI
	JSONValue() goja.Value
	ObjectID() cdpruntime.RemoteObjectID
}
