package js

import (
	"fmt"
	"time"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib/consts"
)

// timeoutError is used when some operation times out.
type timeoutError struct {
	place string
	d     time.Duration
}

var _ interface {
	errext.HasExitCode
	errext.HasHint
	errext.HasAbortReason
} = timeoutError{}

// newTimeoutError returns a new timeout error, reporting that a timeout has
// happened at the given place and given duration.
func newTimeoutError(place string, d time.Duration) timeoutError {
	return timeoutError{place: place, d: d}
}

// String returns the timeout error in human readable format.
func (t timeoutError) Error() string {
	return fmt.Sprintf("%s() execution timed out after %.f seconds", t.place, t.d.Seconds())
}

// Hint potentially returns a hint message for fixing the error.
func (t timeoutError) Hint() string {
	hint := ""

	switch t.place {
	case consts.SetupFn:
		hint = "You can increase the time limit via the setupTimeout option"
	case consts.TeardownFn:
		hint = "You can increase the time limit via the teardownTimeout option"
	}
	return hint
}

func (t timeoutError) AbortReason() errext.AbortReason {
	return errext.AbortedByTimeout
}

// ExitCode returns the corresponding exit code value to the place.
func (t timeoutError) ExitCode() exitcodes.ExitCode {
	// TODO: add handleSummary()
	switch t.place {
	case consts.SetupFn:
		return exitcodes.SetupTimeout
	case consts.TeardownFn:
		return exitcodes.TeardownTimeout
	default:
		return exitcodes.GenericTimeout
	}
}
