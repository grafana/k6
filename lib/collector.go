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

import (
	"context"

	"github.com/loadimpact/k6/stats"
)

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

// A Collector abstracts the process of funneling samples to an external storage backend,
// such as an InfluxDB instance.
type Collector interface {
	// Init is called between the collector's creation and the call to Run().
	// You should do any lengthy setup here rather than in New.
	Init() error

	// Run is called in a goroutine and starts the collector. Should commit samples to the backend
	// at regular intervals and when the context is terminated.
	Run(ctx context.Context)

	// Collect receives a set of samples. This method is never called concurrently, and only while
	// the context for Run() is valid, but should defer as much work as possible to Run().
	Collect(samples []stats.SampleContainer)

	// Optionally return a link that is shown to the user.
	Link() string

	// Return the required system sample tags for the specific collector
	GetRequiredSystemTags() stats.SystemTagSet

	// Set run status
	SetRunStatus(status RunStatus)
}
