/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
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

package common

import (
	"errors"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
)

// InterruptError is an error that halts engine execution
type InterruptError struct {
	Reason string
}

var _ errext.HasExitCode = &InterruptError{}

// Error returns the reason of the interruption.
func (i *InterruptError) Error() string {
	return i.Reason
}

// ExitCode returns the status code used when the k6 process exits.
func (i *InterruptError) ExitCode() errext.ExitCode {
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
