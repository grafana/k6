// Package common contains small helpers for interacting with the Sobek runtime.
package common

import "github.com/grafana/sobek"

// Throw raises err as a JavaScript exception.
//
// Existing exceptions are preserved. Other Go errors are converted with
// NewGoError so Sobek can attach the JavaScript stack at the call site.
func Throw(rt *sobek.Runtime, err error) {
	if exception, ok := err.(*sobek.Exception); ok { //nolint:errorlint // preserve JS exceptions
		panic(exception)
	}
	panic(rt.NewGoError(err))
}
