package common

import (
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
)

// JSHandleAPI is the interface of an in-page JS object.
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
