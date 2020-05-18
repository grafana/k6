package lib

import (
	"fmt"
	"time"

	"github.com/loadimpact/k6/internal"
)

// TimeoutError is used when somethings timeouts
type TimeoutError struct {
	place string
	d     time.Duration
}

// NewTimeoutError returns a new TimeoutError reporting that timeout has happened
// at the given place and given duration.
func NewTimeoutError(place string, d time.Duration) TimeoutError {
	return TimeoutError{place: place, d: d}
}

// String returns timeout error in human readable format.
func (t TimeoutError) String() string {
	return fmt.Sprintf("%s execution timed out after %.f seconds", t.place, t.d.Seconds())
}

// Error implements error interface.
func (t TimeoutError) Error() string {
	return t.String()
}

// Place returns the place where timeout occurred.
func (t TimeoutError) Place() string {
	return t.place
}

// Hint returns a hint message for logging with given stage.
func (t TimeoutError) Hint() string {
	hint := ""

	switch t.place {
	case internal.StageSetup:
		hint = "You can increase the time limit via the setupTimeout option"
	case internal.StageTeardown:
		hint = "You can increase the time limit via the teardownTimeout option"
	}
	return hint
}
