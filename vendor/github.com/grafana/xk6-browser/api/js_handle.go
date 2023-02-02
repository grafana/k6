package api

import (
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
)

// JSHandle is the interface of an in-page JS object.
type JSHandle interface {
	AsElement() ElementHandle
	Dispose()
	Evaluate(pageFunc goja.Value, args ...goja.Value) any
	EvaluateHandle(pageFunc goja.Value, args ...goja.Value) JSHandle
	GetProperties() map[string]JSHandle
	GetProperty(propertyName string) JSHandle
	JSONValue() goja.Value
	ObjectID() cdpruntime.RemoteObjectID
}
