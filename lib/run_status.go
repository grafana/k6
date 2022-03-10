/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

import "errors"

// TODO: move to some other package - execution?

// RunStatus values can be used by k6 to denote how a script run ends
// and by the cloud executor and collector so that k6 knows the current
// status of a particular script run.
type RunStatus int

// Possible run status values; iota isn't used intentionally
const (
	RunStatusCreated            RunStatus = -2
	RunStatusValidated          RunStatus = -1
	RunStatusQueued             RunStatus = 0
	RunStatusInitializing       RunStatus = 1
	RunStatusRunning            RunStatus = 2
	RunStatusFinished           RunStatus = 3
	RunStatusTimedOut           RunStatus = 4
	RunStatusAbortedUser        RunStatus = 5
	RunStatusAbortedSystem      RunStatus = 6
	RunStatusAbortedScriptError RunStatus = 7
	RunStatusAbortedThreshold   RunStatus = 8
)

// HasRunStatus is a wrapper around an error with an attached run status.
type HasRunStatus interface {
	error
	RunStatus() RunStatus
}

// WithRunStatusIfNone can attach a run code to the given error, if it doesn't
// have one already. It won't do anything if the error already had a run status
// attached. Similarly, if there is no error (i.e. the given error is nil), it
// also won't do anything.
func WithRunStatusIfNone(err error, runStatus RunStatus) error {
	if err == nil {
		// No error, do nothing
		return nil
	}
	var ecerr HasRunStatus
	if errors.As(err, &ecerr) {
		// The given error already has a run status, do nothing
		return err
	}
	return withRunStatus{err, runStatus}
}

type withRunStatus struct {
	error
	runStatus RunStatus
}

func (wh withRunStatus) Unwrap() error {
	return wh.error
}

func (wh withRunStatus) RunStatus() RunStatus {
	return wh.runStatus
}

var _ HasRunStatus = withRunStatus{}
