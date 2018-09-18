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
	"time"

	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	log "github.com/sirupsen/logrus"
	null "gopkg.in/guregu/null.v3"
)

// An Executor is in charge of scheduling VUs created by a wrapped Runner, but decouples how you
// control a swarm of VUs from the details of how or even where they're scheduled.
//
// The core/local executor schedules VUs on the local machine, but the same interface may be
// implemented to control a test running on a cluster or in the cloud.
type Executor interface {
	// Run the Executor, funneling generated samples through the out channel.
	Run(ctx context.Context, engineOut chan<- stats.SampleContainer) error
	// Is the executor currently running?
	IsRunning() bool

	// Returns the wrapped runner. May return nil if not applicable, eg. if we're remote
	// controlling a test running on another machine.
	GetRunner() Runner

	// Get and set the logger. This is propagated to the Runner.
	GetLogger() *log.Logger
	SetLogger(l *log.Logger)

	// Get and set the list of stages.
	GetStages() []Stage
	SetStages(s []Stage)

	// Get iterations executed so far, get and set how many to end the test after.
	GetIterations() int64
	GetEndIterations() null.Int
	SetEndIterations(i null.Int)

	// Get time elapsed so far, accounting for pauses, get and set at what point to end the test.
	GetTime() time.Duration
	GetEndTime() types.NullDuration
	SetEndTime(t types.NullDuration)

	// Check whether the test is paused, or pause it. A paused won't start any new iterations (but
	// will allow currently in progress ones to finish), and will not increment the value returned
	// by GetTime().
	IsPaused() bool
	SetPaused(paused bool)

	// Get and set the number of currently active VUs.
	// It is an error to try to set this higher than MaxVUs.
	GetVUs() int64
	SetVUs(vus int64) error

	// Get and set the number of allocated, available VUs.
	// Please note that initialising new VUs is a very expensive operation, and doing it during a
	// running test may skew metrics; if you're not sure how many you will need, it's generally
	// speaking better to preallocate too many than too few.
	GetVUsMax() int64
	SetVUsMax(max int64) error

	// Set whether or not to run setup/teardown phases. Default is to run all of them.
	SetRunSetup(r bool)
	SetRunTeardown(r bool)
}
