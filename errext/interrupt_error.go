package errext

import (
	"errors"

	"go.k6.io/k6/errext/exitcodes"
)

// InterruptError is an error that halts engine execution
type InterruptError struct {
	Reason string
}

var _ interface {
	HasExitCode
	HasAbortReason
} = &InterruptError{}

// Error returns the reason of the interruption.
func (i *InterruptError) Error() string {
	return i.Reason
}

// ExitCode returns the status code used when the k6 process exits.
func (i *InterruptError) ExitCode() exitcodes.ExitCode {
	return exitcodes.ScriptAborted
}

// AbortReason is used to signal that an InterruptError is caused by the
// test.abort() functin in k6/execution.
func (i *InterruptError) AbortReason() AbortReason {
	return AbortedByScriptAbort
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
