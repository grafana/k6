package browser

import (
	"context"
	"errors"
	"strings"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6error"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/promises"
)

func panicIfFatalError(ctx context.Context, err error) {
	if errors.Is(err, k6error.ErrFatal) {
		k6ext.Abortf(ctx, err.Error())
	}
}

// mergeWith merges the Sobek value with the existing Go value.
func mergeWith[T any](rt *sobek.Runtime, src T, v sobek.Value) error {
	if common.IsNullish(v) {
		return nil
	}
	return rt.ExportTo(v, &src) //nolint:wrapcheck
}

// exportTo exports the Sobek value to a Go value.
// It returns the zero value of T if obj does not exist in the Sobek runtime.
// It's caller's responsibility to check for nilness.
func exportTo[T any](rt *sobek.Runtime, obj sobek.Value) (T, error) {
	var t T
	if common.IsNullish(obj) {
		return t, nil
	}
	err := rt.ExportTo(obj, &t)
	return t, err //nolint:wrapcheck
}

// exportArg exports the value and returns it.
// It returns nil if the value is undefined or null.
func exportArg(gv sobek.Value) any {
	if common.IsNullish(gv) {
		return nil
	}
	return gv.Export()
}

// exportArgs returns a slice of exported sobek values.
func exportArgs(gargs []sobek.Value) []any {
	args := make([]any, 0, len(gargs))
	for _, garg := range gargs {
		// leaves a nil garg in the array since users might want to
		// pass undefined or null as an argument to a function
		args = append(args, exportArg(garg))
	}
	return args
}

// sobekEmptyString returns true if a given value is not nil or an empty string.
func sobekEmptyString(v sobek.Value) bool {
	return common.IsNullish(v) || strings.TrimSpace(v.String()) == ""
}

// promise runs fn in a goroutine and returns a new sobek.Promise.
//   - If fn returns a nil error, resolves the promise with the
//     first result value fn returns.
//   - Otherwise, rejects the promise with the error fn returns.
func promise(vu moduleVU, fn func() (result any, reason error)) *sobek.Promise {
	p, resolve, reject := promises.New(vu)
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
