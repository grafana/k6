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

var (
	_ errext.HasExitCode = timeoutError{}
	_ errext.HasHint     = timeoutError{}
)

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

// ExitCode returns the coresponding exit code value to the place.
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
