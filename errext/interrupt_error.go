package errext

import (
	"errors"

	"go.k6.io/k6/errext/exitcodes"
)

// InterruptError is an error that halts engine execution
type InterruptError struct {
	Reason string
}

var _ HasExitCode = &InterruptError{}

// Error returns the reason of the interruption.
func (i *InterruptError) Error() string {
	return i.Reason
}

// ExitCode returns the status code used when the k6 process exits.
func (i *InterruptError) ExitCode() exitcodes.ExitCode {
	return exitcodes.ScriptAborted
}

// AbortTest is the reason emitted when a test script calls test.abort()
const AbortTest = "test aborted"

// IsInterruptError returns true if err is *InterruptError.
func IsInterruptError(err error) bool {
	if err == nil {
		return false
	}
	var intErr *InterruptError
	return errors.As(err, &intErr)
}
