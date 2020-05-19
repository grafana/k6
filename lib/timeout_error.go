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
	case stageSetup:
		hint = "You can increase the time limit via the setupTimeout option"
	case stageTeardown:
		hint = "You can increase the time limit via the teardownTimeout option"
	}
	return hint
}
