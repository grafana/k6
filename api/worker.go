package api

import "github.com/dop251/goja"

// Worker is the interface of a web worker.
type Worker interface {
	Evaluate(pageFunc goja.Value, args ...goja.Value) any
	EvaluateHandle(pageFunc goja.Value, args ...goja.Value) (JSHandle, error)
	URL() string
}
