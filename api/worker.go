package api

import "github.com/dop251/goja"

// WorkerAPI is the interface of a web worker.
type WorkerAPI interface {
	Evaluate(pageFunc goja.Value, args ...goja.Value) any
	EvaluateHandle(pageFunc goja.Value, args ...goja.Value) (JSHandleAPI, error)
	URL() string
}
