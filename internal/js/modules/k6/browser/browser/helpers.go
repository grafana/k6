package browser

import (
	"context"
	"errors"
	"strings"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6error"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
)

func panicIfFatalError(ctx context.Context, err error) {
	if errors.Is(err, k6error.ErrFatal) {
		k6ext.Abort(ctx, err.Error())
	}
}

// mergeWith merges the Sobek value with the existing Go value.
func mergeWith[T any](rt *sobek.Runtime, src T, v sobek.Value) error {
	if !sobekValueExists(v) {
		return nil
	}
	return rt.ExportTo(v, &src) //nolint:wrapcheck
}

// exportTo exports the Sobek value to a Go value.
// It returns the zero value of T if obj does not exist in the Sobek runtime.
// It's caller's responsibility to check for nilness.
func exportTo[T any](rt *sobek.Runtime, obj sobek.Value) (T, error) {
	var t T
	if !sobekValueExists(obj) {
		return t, nil
	}
	err := rt.ExportTo(obj, &t)
	return t, err //nolint:wrapcheck
}

// exportArg exports the value and returns it.
// It returns nil if the value is undefined or null.
func exportArg(gv sobek.Value) any {
	if !sobekValueExists(gv) {
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

// sobekValueExists returns true if a given value is not nil and exists
// (defined and not null) in the sobek runtime.
func sobekValueExists(v sobek.Value) bool {
	return v != nil && !sobek.IsUndefined(v) && !sobek.IsNull(v)
}

// sobekEmptyString returns true if a given value is not nil or an empty string.
func sobekEmptyString(v sobek.Value) bool {
	return !sobekValueExists(v) || strings.TrimSpace(v.String()) == ""
}
