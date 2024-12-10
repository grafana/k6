package errext

import (
	"errors"

	"go.k6.io/k6/errext/exitcodes"
)

// ExitCode is the code with which the application should exit if this error
// bubbles up to the top of the scope. Values should be between 0 and 125:
// https://unix.stackexchange.com/questions/418784/what-is-the-min-and-max-values-of-exit-codes-in-linux

// HasExitCode is a wrapper around an error with an attached exit code.
type HasExitCode interface {
	error
	ExitCode() exitcodes.ExitCode
}

// WithExitCodeIfNone can attach an exit code to the given error, if it doesn't
// have one already. It won't do anything if the error already had an exit code
// attached. Similarly, if there is no error (i.e. the given error is nil), it
// also won't do anything.
func WithExitCodeIfNone(err error, exitCode exitcodes.ExitCode) error {
	if err == nil {
		// No error, do nothing
		return nil
	}
	var ecerr HasExitCode
	if errors.As(err, &ecerr) {
		// The given error already has an exit code, do nothing
		return err
	}
	return withExitCodeError{err, exitCode}
}

type withExitCodeError struct {
	error
	exitCode exitcodes.ExitCode
}

func (wh withExitCodeError) Unwrap() error {
	return wh.error
}

func (wh withExitCodeError) ExitCode() exitcodes.ExitCode {
	return wh.exitCode
}

var _ HasExitCode = withExitCodeError{}
