// Package k6error contains ErrFatal.
package k6error

import (
	"errors"
)

// ErrFatal should be wrapped into an error
// to signal to the mapping layer that the error
// is a fatal error and we should abort the whole
// test run, not just the current iteration. It
// should be used in cases where if the iteration
// ran again then there's a 100% chance that it
// will end up running into the same error.
var ErrFatal = errors.New("fatal error")
