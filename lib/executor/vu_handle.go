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

package executor

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/sirupsen/logrus"

	"github.com/loadimpact/k6/lib"
)

// states
// there is no specific type as this need to be used with atomic
const (
	stopped = iota
	starting
	running
	toGracefulStop
	toHardStop
)

/*
the below is a state transition table (https://en.wikipedia.org/wiki/State-transition_table)
short names for input:
- start is the method start
- loop is a loop of runLoopsIfPossible
- grace is the method gracefulStop
- hard is the method hardStop
+-------+-------------------------------------+---------------------------------------------------+
| input | current         | next state        | notes                                             |
+-------+-------------------------------------+---------------------------------------------------+
| start | stopped         | starting          | normal                                            |
| start | starting        | starting          | nothing                                           |
| start | running         | running           | nothing                                           |
| start | toGracefulStop  | running           | we raced with the loop stopping, just continue    |
| start | toHardStop      | starting          | same as stopped really                            |
| loop  | stopped         | stopped           | we actually are blocked on canStartIter           |
| loop  | starting        | running           | get new VU and context                            |
| loop  | running         | running           | usually fast path                                 |
| loop  | toGracefulStop  | stopped           | cancel the context and make new one               |
| loop  | toHardStop      | stopped           | cancel the context and make new one               |
| grace | stopped         | stopped           | nothing                                           |
| grace | starting        | stopped           | cancel the context to return the VU               |
| grace | running         | toGracefulStop    | normal one, the actual work is in the loop        |
| grace | toGracefulStop  | toGracefulStop    | nothing                                           |
| grace | toHardSTop      | toHardStop        | nothing                                           |
| hard  | stopped         | stopped           | nothing                                           |
| hard  | starting        | stopped           | short circuit as in the grace case, not necessary |
| hard  | running         | toHardStop        | normal, cancel context and reinitialize it        |
| hard  | toGracefulStop  | toHardStop        | normal, cancel context and reinitialize it        |
| hard  | toHardStop      | toHardStop        | nothing                                           |
+-------+-----------------+-------------------+----------------------------------------------------+
*/

// This is a helper type used in executors where we have to dynamically control
// the number of VUs that are simultaneously running. For the moment, it is used
// in the RampingVUs and the ExternallyControlled executors.
// Notes on the implementation requirements:
// - it needs to be able to start and stop VUs in thread safe fashion
// - for each call to getVU there must be 1 (and only 1) call to returnVU
// - gracefulStop must let an iteration which has started to finish. For reasons of ease of
// implementation and lack of good evidence it's not required to let a not started iteration to
// finish in other words if you call start and then gracefulStop, there is no requirement for
// 1 iteration to have been started.
// - hardStop must stop an iteration in process
// - it's not required but preferable, if where possible to not reactivate VUs and to reuse context
// as this speed ups the execution
type vuHandle struct {
	mutex     *sync.Mutex
	parentCtx context.Context
	getVU     func() (lib.InitializedVU, error)
	returnVU  func(lib.InitializedVU)
	config    *BaseConfig

	initVU       lib.InitializedVU
	activeVU     lib.ActiveVU
	canStartIter chan struct{}

	state int32 // see the table above for meanings
	// stateH []int32 // helper for debugging

	ctx    context.Context
	cancel func()
	logger *logrus.Entry
}

func newStoppedVUHandle(
	parentCtx context.Context, getVU func() (lib.InitializedVU, error),
	returnVU func(lib.InitializedVU), config *BaseConfig, logger *logrus.Entry,
) *vuHandle {
	ctx, cancel := context.WithCancel(parentCtx)

	return &vuHandle{
		mutex:     &sync.Mutex{},
		parentCtx: parentCtx,
		getVU:     getVU,
		config:    config,

		canStartIter: make(chan struct{}),
		state:        stopped,

		ctx:      ctx,
		cancel:   cancel,
		logger:   logger,
		returnVU: returnVU,
	}
}

func (vh *vuHandle) start() (err error) {
	vh.mutex.Lock()
	defer vh.mutex.Unlock()

	if vh.state == starting || vh.state == running {
		return nil // nothing to do
	}

	vh.logger.Debug("Start")
	if vh.state == toGracefulStop { // No point returning the VU just continue as it was
		close(vh.canStartIter)
		vh.changeState(running)
		return
	}

	vh.initVU, err = vh.getVU()
	if err != nil {
		return err
	}

	vh.activeVU = vh.initVU.Activate(getVUActivationParams(vh.ctx, *vh.config, vh.returnVU))
	close(vh.canStartIter)
	vh.changeState(starting)
	return nil
}

// just a helper function for debugging
func (vh *vuHandle) changeState(newState int32) {
	// vh.stateH = append(vh.stateH, newState)
	atomic.StoreInt32(&vh.state, newState)
}

func (vh *vuHandle) gracefulStop() {
	vh.mutex.Lock()
	defer vh.mutex.Unlock()

	if vh.state == toGracefulStop || vh.state == toHardStop || vh.state == stopped {
		return
	}

	vh.logger.Debug("Graceful stop")

	if vh.state == starting { // we raced with starting
		vh.cancel()
		vh.ctx, vh.cancel = context.WithCancel(vh.parentCtx)
		vh.changeState(stopped)
	} else {
		vh.changeState(toGracefulStop)
	}
	vh.canStartIter = make(chan struct{})
}

func (vh *vuHandle) hardStop() {
	vh.mutex.Lock()
	defer vh.mutex.Unlock()

	if vh.state == toHardStop || vh.state == stopped {
		return
	}

	vh.logger.Debug("Hard stop")

	vh.cancel()
	vh.ctx, vh.cancel = context.WithCancel(vh.parentCtx)
	if vh.state == starting { // we raced with starting
		vh.changeState(stopped)
	} else {
		vh.changeState(toHardStop)
	}
	vh.canStartIter = make(chan struct{})
}

// runLoopsIfPossible is where all the fun is :D. Unfortunately somewhere we need to check most
// of the cases and this is where this happens.
func (vh *vuHandle) runLoopsIfPossible(runIter func(context.Context, lib.ActiveVU) bool) {
	// We can probably initialize here, but it's also easier to just use the slow path in the second
	// part of the for loop
	defer func() {
		// not sure if this is needed, because here the parentCtx is canceled and I would argue it doesn't matter
		// if we set the correct state
		vh.mutex.Lock()
		vh.changeState(stopped)
		vh.mutex.Unlock()
	}()

	var (
		executorDone = vh.parentCtx.Done()
		ctx          context.Context
		cancel       func()
		vu           lib.ActiveVU
	)

	for {
		state := atomic.LoadInt32(&vh.state)
		if state == running && runIter(ctx, vu) { // fast path
			continue
		}

		// slow path - something has changed - get what and wait until we can do more iterations
		vh.mutex.Lock()
		select {
		case <-executorDone:
			// The whole executor is done, nothing more to do.
			vh.mutex.Unlock()
			return
		default:
		}

		if vh.state == running { // start raced us to toGracefulStop
			vh.mutex.Unlock()
			continue
		}

		if vh.state == toGracefulStop || vh.state == toHardStop {
			if cancel != nil && vh.state == toGracefulStop {
				// we need to cancel the context, to return the vu
				// and because *we* did, lets reinitialize it
				cancel()
				vh.ctx, vh.cancel = context.WithCancel(vh.parentCtx)
			}
			// we have *now* stopped
			vh.changeState(stopped)
		}
		canStartIter := vh.canStartIter
		ctx = vh.ctx
		vh.mutex.Unlock()

		// We're are stopped, but the executor isn't done yet, so we wait
		// for either one of those conditions.
		select {
		case <-canStartIter: // we can start again
			vh.mutex.Lock()
			select {
			case <-vh.canStartIter: // we check again in case of race
				// reinitialize
				vu, ctx, cancel = vh.activeVU, vh.ctx, vh.cancel
				vh.changeState(running)
			default:
				// well we got raced to here by something ... loop again ...
			}
			vh.mutex.Unlock()
		case <-ctx.Done():
			// hardStop was called, start a fresh iteration to get the new
			// context and signal channel
		case <-executorDone:
			// The whole executor is done, nothing more to do.
			return
		}
	}
}
