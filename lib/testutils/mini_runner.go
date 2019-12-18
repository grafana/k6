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

package testutils

import (
	"context"
	"sync/atomic"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
)

// Ensure mock implementations conform to the interfaces.
var _ lib.Runner = &MiniRunner{}
var _ lib.VU = &MiniRunnerVU{}

// MiniRunner partially implements the lib.Runner interface, but instead of
// using a real JS runtime, it allows us to directly specify the options and
// functions with Go code.
type MiniRunner struct {
	Fn         func(ctx context.Context, out chan<- stats.SampleContainer) error
	SetupFn    func(ctx context.Context, out chan<- stats.SampleContainer) ([]byte, error)
	TeardownFn func(ctx context.Context, out chan<- stats.SampleContainer) error

	SetupData []byte

	NextVUID int64
	Group    *lib.Group
	Options  lib.Options
}

// MakeArchive isn't implemented, it always returns nil and is just here to
// satisfy the lib.Runner interface.
func (r MiniRunner) MakeArchive() *lib.Archive {
	return nil
}

// NewVU returns a new MiniRunnerVU with an incremental ID.
func (r *MiniRunner) NewVU(out chan<- stats.SampleContainer) (lib.VU, error) {
	nextVUNum := atomic.AddInt64(&r.NextVUID, 1)
	return &MiniRunnerVU{R: r, Out: out, ID: nextVUNum - 1}, nil
}

// Setup calls the supplied mock setup() function, if present.
func (r *MiniRunner) Setup(ctx context.Context, out chan<- stats.SampleContainer) (err error) {
	if fn := r.SetupFn; fn != nil {
		r.SetupData, err = fn(ctx, out)
	}
	return
}

// GetSetupData returns json representation of the setup data if setup() is
// specified and was ran, nil otherwise.
func (r MiniRunner) GetSetupData() []byte {
	return r.SetupData
}

// SetSetupData saves the externally supplied setup data as JSON in the runner.
func (r *MiniRunner) SetSetupData(data []byte) {
	r.SetupData = data
}

// Teardown calls the supplied mock teardown() function, if present.
func (r MiniRunner) Teardown(ctx context.Context, out chan<- stats.SampleContainer) error {
	if fn := r.TeardownFn; fn != nil {
		return fn(ctx, out)
	}
	return nil
}

// GetDefaultGroup returns the default group.
func (r MiniRunner) GetDefaultGroup() *lib.Group {
	if r.Group == nil {
		r.Group = &lib.Group{}
	}
	return r.Group
}

// GetOptions returns the supplied options struct.
func (r MiniRunner) GetOptions() lib.Options {
	return r.Options
}

// SetOptions allows you to override the runner options.
func (r *MiniRunner) SetOptions(opts lib.Options) error {
	r.Options = opts
	return nil
}

// MiniRunnerVU is a mock VU, spawned by a MiniRunner.
type MiniRunnerVU struct {
	R         *MiniRunner
	Out       chan<- stats.SampleContainer
	ID        int64
	Iteration int64
}

// RunOnce runs the mock default function once, incrementing its iteration.
func (vu MiniRunnerVU) RunOnce(ctx context.Context) error {
	if vu.R.Fn == nil {
		return nil
	}

	state := &lib.State{
		Vu:        vu.ID,
		Iteration: vu.Iteration,
	}
	newctx := lib.WithState(ctx, state)

	vu.Iteration++

	return vu.R.Fn(newctx, vu.Out)
}

// Reconfigure changes the VU ID.
func (vu *MiniRunnerVU) Reconfigure(id int64) error {
	vu.ID = id
	return nil
}
