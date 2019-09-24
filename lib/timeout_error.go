package lib

import (
	"fmt"
	"time"
)

//nolint:gochecknoglobals
// Keep stages in sync with js/runner.go
// We set it here to prevent import cycle.
var (
	stageSetup    = "setup"
	stageTeardown = "teardown"
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
	s := t.place + " execution timed out"
	if t.d.Seconds() > 0 {
		s += fmt.Sprintf(" after %.f seconds", t.d.Seconds())
	}
	return s
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
	case stageSetup:
		hint = "You can increase the time limit via the setupTimeout option"
	case stageTeardown:
		hint = "You can increase the time limit via the teardownTimeout option"
	}
	return hint
}
