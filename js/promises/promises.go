// Package promises provides helpers for working with promises in k6.
package promises

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/js/modules"
)

// New can be used to create promises that will be dispatched to k6's event loop.
//
// Calling the function will create a Sobek promise and return its `resolve` and `reject` callbacks, wrapped
// in such a way that it will block the k6 JavaScript runtime's event loop from exiting before they are
// called, even if the promise isn't resolved by the time the current script ends executing.
//
// A typical usage would be:
//
//	   func myAsynchronousFunc(vu modules.VU) *(sobek.Promise) {
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
func New(vu modules.VU) (p *sobek.Promise, resolve func(result any), reject func(reason any)) {
	p, resolveFunc, rejectFunc := vu.Runtime().NewPromise()
	callback := vu.RegisterCallback()

	resolve = func(result any) {
		callback(func() error {
			return resolveFunc(result)
		})
	}

	reject = func(reason any) {
		callback(func() error {
			return rejectFunc(reason)
		})
	}

	return p, resolve, reject
}
