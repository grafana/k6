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

package errors

// InterruptError is an error that halts engine execution
type InterruptError struct {
	Reason string
}

func (i InterruptError) Error() string {
	return i.Reason
}

// AbortTest is a reason emitted when a test script calls abortTest() without arguments
const AbortTest = "abortTest() was called in a script"

// IsInterruptError returns true if err is *InterruptError
func IsInterruptError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*InterruptError)
	return ok
}
