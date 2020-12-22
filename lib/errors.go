/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package lib

import (
	"fmt"
	"time"

	"github.com/loadimpact/k6/lib/consts"
)

// Exception represents an internal JS error or unexpected panic in VU code.
type Exception struct {
	Err              interface{}
	StackGo, StackJS string
}

// Error implements the error interface.
func (exc Exception) Error() string {
	out := fmt.Sprintf("%s", exc.Err)
	if exc.StackGo != "" {
		out += fmt.Sprintf("\n%s", exc.StackGo)
	}
	if exc.StackJS != "" {
		if exc.StackGo != "" {
			out += "\nJavaScript stack:"
		}
		out += fmt.Sprintf("\n%s", exc.StackJS)
	}
	return out
}

// IterationInterruptedError is used when an iteration is interrupted due to
// one of the following:
// - normal JS exceptions, throw(), etc., with cause: error
// - k6.fail() called in script, with cause: fail
// - the iteration exceeded the gracefulStop or gracefulRampDown duration
//   and was interrupted by the executor, with cause: duration
// - a signal is received, e.g. via Ctrl+C, with cause: signal
type IterationInterruptedError struct {
	cause, msg string
}

// NewIterationInterruptedError returns a new error.
func NewIterationInterruptedError(cause, msg string) IterationInterruptedError {
	return IterationInterruptedError{cause: cause, msg: msg}
}

// Cause returns the cause of the interruption.
func (e IterationInterruptedError) Cause() string {
	return e.cause
}

// String returns the error in human readable format.
func (e IterationInterruptedError) String() string {
	return fmt.Sprintf("%s: %s", e.cause, e.msg)
}

// Error implements the error interface.
func (e IterationInterruptedError) Error() string {
	return e.String()
}

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
	return fmt.Sprintf("%s() execution timed out after %.f seconds", t.place, t.d.Seconds())
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
	case consts.SetupFn:
		hint = "You can increase the time limit via the setupTimeout option"
	case consts.TeardownFn:
		hint = "You can increase the time limit via the teardownTimeout option"
	}
	return hint
}
