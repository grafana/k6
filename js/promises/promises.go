// Package promises provides helpers for working with promises in k6.
package promises

import (
	"github.com/dop251/goja"
	"go.k6.io/k6/js/modules"
)

// MakeHandledPromise can be used to create promises that will be dispatched to k6's event loop.
//
// Calling the function will create a goja promise and return its `resolve` and `reject` callbacks, wrapped
// in such a way that it will block the k6 JS runtime's event loop from exiting before they are
// called, even if the promise isn't resolved by the time the current script ends executing.
//
// A typical usage would be:
//
//	   func myAsynchronousFunc(vu modules.VU) *(goja.Promise) {
//		    promise, resolve, reject := promises.MakeHandledPromise(vu)
//		    go func() {
//		        v, err := someAsyncFunc()
//				   if err != nil {
//		            reject(err)
//		            return
//		        }
//
//		        resolve(v)
//		    }()
//		    return promise
//		  }
func MakeHandledPromise(vu modules.VU) (promise *goja.Promise, resolve func(result interface{}), reject func(reason interface{})) {
	runtime := vu.Runtime()
	promise, resolve, reject = runtime.NewPromise()
	callback := vu.RegisterCallback()

	return promise, func(i interface{}) {
			callback(func() error {
				resolve(i)
				return nil
			})
		}, func(i interface{}) {
			callback(func() error {
				reject(i)
				return nil
			})
		}
}
