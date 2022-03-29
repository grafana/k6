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

// TODO: move to some other package - types? models?

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
	RunStatusAbortedLimit       RunStatus = 9
)
