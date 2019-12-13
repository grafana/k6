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

// A Runner is a factory for VUs. It should precompute as much as possible upon
// creation (parse ASTs, load files into memory, etc.), so that spawning VUs
// becomes as fast as possible. The Runner doesn't actually *do* anything in
// itself, the ExecutionScheduler is responsible for wrapping and scheduling
// these VUs for execution.
//
// TODO: Rename this to something more obvious? This name made sense a very long
// time ago.
type Runner interface {
	// Creates an Archive of the runner. There should be a corresponding NewFromArchive() function
	// that will restore the runner from the archive.
	MakeArchive() *Archive

	// Spawns a new VU. It's fine to make this function rather heavy, if it means a performance
	// improvement at runtime. Remember, this is called once per VU and normally only at the start
	// of a test - RunOnce() may be called hundreds of thousands of times, and must be fast.
	//TODO: pass context.Context, so initialization can be killed properly...
	NewVU(out chan<- stats.SampleContainer) (VU, error)

	// Runs pre-test setup, if applicable.
	Setup(ctx context.Context, out chan<- stats.SampleContainer) error

	// Returns json representation of the setup data if setup() is specified and run, nil otherwise
	GetSetupData() []byte

	// Saves the externally supplied setup data as json in the runner
	SetSetupData([]byte)

	// Runs post-test teardown, if applicable.
	Teardown(ctx context.Context, out chan<- stats.SampleContainer) error

	// Returns the default (root) Group.
	GetDefaultGroup() *Group

	// Get and set options. The initial value will be whatever the script specifies (for JS,
	// `export let options = {}`); cmd/run.go will mix this in with CLI-, config- and env-provided
	// values and write it back to the runner.
	GetOptions() Options
	SetOptions(opts Options) error
}

// A VU is a Virtual User, that can be scheduled by an Executor.
type VU interface {
	// Runs the VU once. The VU is responsible for handling the Halting Problem, eg. making sure
	// that execution actually stops when the context is cancelled.
	RunOnce(ctx context.Context) error

	// Assign the VU a new ID. Called by the Executor upon creation, but may be called multiple
	// times if the VU is recycled because the test was scaled down and then back up.
	//TODO: support reconfiguring of env vars, tags and exec
	Reconfigure(id int64) error
}
