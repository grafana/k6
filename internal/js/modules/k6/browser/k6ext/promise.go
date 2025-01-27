package k6ext

import (
	"context"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/promises"
)

// PromisifiedFunc is a type of the function to run as a promise.
type PromisifiedFunc func() (result any, reason error)

// Promise runs fn in a goroutine and returns a new sobek.Promise.
//   - If fn returns a nil error, resolves the promise with the
//     first result value fn returns.
//   - Otherwise, rejects the promise with the error fn returns.
func Promise(ctx context.Context, fn PromisifiedFunc) *sobek.Promise {
	return promise(ctx, fn)
}

func promise(ctx context.Context, fn PromisifiedFunc) *sobek.Promise {
	p, resolve, reject := promises.New(GetVU(ctx))
	go func() {
		v, err := fn()
		if err != nil {
			reject(err)
			return
		}
		resolve(v)
	}()

	return p
}
