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

package minirunner

import (
	"context"
	"io"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/stats"
)

// Ensure mock implementations conform to the interfaces.
var (
	_ lib.Runner        = &MiniRunner{}
	_ lib.InitializedVU = &VU{}
	_ lib.ActiveVU      = &ActiveVU{}
)

// MiniRunner partially implements the lib.Runner interface, but instead of
// using a real JS runtime, it allows us to directly specify the options and
// functions with Go code.
type MiniRunner struct {
	Fn              func(ctx context.Context, state *lib.State, out chan<- stats.SampleContainer) error
	SetupFn         func(ctx context.Context, out chan<- stats.SampleContainer) ([]byte, error)
	TeardownFn      func(ctx context.Context, out chan<- stats.SampleContainer) error
	HandleSummaryFn func(context.Context, *lib.Summary) (map[string]io.Reader, error)

	SetupData []byte

	Group   *lib.Group
	Options lib.Options
}

// MakeArchive isn't implemented, it always returns nil and is just here to
// satisfy the lib.Runner interface.
func (r MiniRunner) MakeArchive() *lib.Archive {
	return nil
}

// NewVU returns a new VU with an incremental ID.
func (r *MiniRunner) NewVU(idLocal, idGlobal uint64, out chan<- stats.SampleContainer) (lib.InitializedVU, error) {
	state := &lib.State{VUID: idLocal, VUIDGlobal: idGlobal, Iteration: int64(-1)}
	return &VU{
		R:            r,
		Out:          out,
		ID:           idLocal,
		IDGlobal:     idGlobal,
		state:        state,
		scenarioIter: make(map[string]uint64),
	}, nil
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

// IsExecutable satisfies lib.Runner, but is mocked for MiniRunner since
// it doesn't deal with JS.
func (r MiniRunner) IsExecutable(name string) bool {
	return true
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

// HandleSummary calls the specified summary callback, if supplied.
func (r *MiniRunner) HandleSummary(ctx context.Context, s *lib.Summary) (map[string]io.Reader, error) {
	if r.HandleSummaryFn != nil {
		return r.HandleSummaryFn(ctx, s)
	}
	return nil, nil
}

// VU is a mock VU, spawned by a MiniRunner.
type VU struct {
	R            *MiniRunner
	Out          chan<- stats.SampleContainer
	ID, IDGlobal uint64
	Iteration    int64
	state        *lib.State
	// count of iterations executed by this VU in each scenario
	scenarioIter map[string]uint64
}

// ActiveVU holds a VU and its activation parameters
type ActiveVU struct {
	*VU
	*lib.VUActivationParams
	busy chan struct{}

	scenarioName              string
	getNextIterations         func() (uint64, uint64)
	scIterLocal, scIterGlobal uint64
}

// GetID returns the unique VU ID.
func (vu *VU) GetID() uint64 {
	return vu.ID
}

// State returns the VU's State.
func (vu *VU) State() *lib.State {
	return vu.state
}

// Activate the VU so it will be able to run code.
func (vu *VU) Activate(params *lib.VUActivationParams) lib.ActiveVU {
	ctx := params.RunContext

	vu.state.GetScenarioVUIter = func() uint64 {
		return vu.scenarioIter[params.Scenario]
	}

	avu := &ActiveVU{
		VU:                 vu,
		VUActivationParams: params,
		busy:               make(chan struct{}, 1),
		scenarioName:       params.Scenario,
		scIterLocal:        ^uint64(0),
		scIterGlobal:       ^uint64(0),
		getNextIterations:  params.GetNextIterationCounters,
	}

	vu.state.GetScenarioLocalVUIter = func() uint64 {
		return avu.scIterLocal
	}
	vu.state.GetScenarioGlobalVUIter = func() uint64 {
		return avu.scIterGlobal
	}

	go func() {
		<-ctx.Done()

		// Wait for the VU to stop running, if it was, and prevent it from
		// running again for this activation
		avu.busy <- struct{}{}

		if params.DeactivateCallback != nil {
			params.DeactivateCallback(vu)
		}
	}()

	return avu
}

func (vu *ActiveVU) incrIteration() {
	vu.Iteration++
	vu.state.Iteration = vu.Iteration

	if _, ok := vu.scenarioIter[vu.scenarioName]; ok {
		vu.scenarioIter[vu.scenarioName]++
	} else {
		vu.scenarioIter[vu.scenarioName] = 0
	}
	vu.scIterLocal, vu.scIterGlobal = vu.getNextIterations()
}

// RunOnce runs the mock default function once, incrementing its iteration.
func (vu *ActiveVU) RunOnce() error {
	if vu.R.Fn == nil {
		return nil
	}

	select {
	case <-vu.RunContext.Done():
		return vu.RunContext.Err() // we are done, return
	case vu.busy <- struct{}{}:
		// nothing else can run now, and the VU cannot be deactivated
	}
	defer func() {
		<-vu.busy // unlock deactivation again
	}()

	vu.incrIteration()
	return vu.R.Fn(vu.RunContext, vu.State(), vu.Out)
}
