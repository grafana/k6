package tracing

import "github.com/dop251/goja"

// isNullish checks if the given value is nullish, i.e. nil, undefined or null.
//
// This helper function emulates the behavior of Javascript's nullish coalescing
// operator (??).
func isNullish(value goja.Value) bool {
	return value == nil || goja.IsUndefined(value) || goja.IsNull(value)
}
