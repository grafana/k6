// Package k6error contains ErrInternal.
package k6error

import (
	"errors"
)

// ErrInternal should be wrapped into an error
// to signal to the mapping layer that the error
// is an internal error and we should abort the test run.
var ErrInternal = errors.New("internal error")
