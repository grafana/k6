package k6ext

import (
	"context"

	"github.com/dop251/goja"
)

// eventLoopDirective determines whether the event
// loop should be aborted if the promise is rejected.
type eventLoopDirective int

const (
	continueEventLoop eventLoopDirective = iota + 1
	abortEventLoop
)

// PromisifiedFunc is a type of the function to run as a promise.
type PromisifiedFunc func() (result interface{}, reason error)

// Promise runs fn in a goroutine and returns a new goja.Promise.
//   - If fn returns a nil error, resolves the promise with the
//     first result value fn returns.
//   - Otherwise, rejects the promise with the error fn returns.
func Promise(ctx context.Context, fn PromisifiedFunc) *goja.Promise {
	return promise(ctx, fn, continueEventLoop)
}

// AbortingPromise is like Promise, but it aborts the event loop if an error occurs.
func AbortingPromise(ctx context.Context, fn PromisifiedFunc) *goja.Promise {
	return promise(ctx, fn, abortEventLoop)
}

func promise(ctx context.Context, fn PromisifiedFunc, d eventLoopDirective) *goja.Promise {
	var (
		vu                 = GetVU(ctx)
		cb                 = vu.RegisterCallback()
		p, resolve, reject = vu.Runtime().NewPromise()
	)
	go func() {
		v, err := fn()
		cb(func() error {
			if err != nil {
				reject(err)
			} else {
				resolve(v)
			}
			if d == continueEventLoop {
				err = nil
			}
			return err
		})
	}()

	return p
}
