// Package promises provides helpers for working with promises in k6.
package promises

import (
	"github.com/dop251/goja"
	"go.k6.io/k6/js/modules"
)

// New can be used to create promises that will be dispatched to k6's event loop.
//
// Calling the function will create a goja promise and return its `resolve` and `reject` callbacks, wrapped
// in such a way that it will block the k6 JavaScript runtime's event loop from exiting before they are
// called, even if the promise isn't resolved by the time the current script ends executing.
//
// A typical usage would be:
//
//	   func myAsynchronousFunc(vu modules.VU) *(goja.Promise) {
//		    promise, resolve, reject := promises.New(vu)
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
func New(vu modules.VU) (p *goja.Promise, resolve func(result any), reject func(reason any)) {
	p, resolveFunc, rejectFunc := vu.Runtime().NewPromise()
	callback := vu.RegisterCallback()

	resolve = func(result any) {
		callback(func() error {
			resolveFunc(result)
			return nil
		})
	}

	reject = func(reason any) {
		callback(func() error {
			rejectFunc(reason)
			return nil
		})
	}

	return p, resolve, reject
}
