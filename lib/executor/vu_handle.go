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
const (
	stopped = iota
	starting
	running
	toGracefulStop
	toHardStop
)

// This is a helper type used in executors where we have to dynamically control
// the number of VUs that are simultaneously running. For the moment, it is used
// in the RampingVUs and the ExternallyControlled executors.
//
// TODO: something simpler?
type vuHandle struct {
	mutex     *sync.Mutex
	parentCtx context.Context
	getVU     func() (lib.InitializedVU, error)
	returnVU  func(lib.InitializedVU)
	config    *BaseConfig

	initVU       lib.InitializedVU
	activeVU     lib.ActiveVU
	canStartIter chan struct{}
	// If state is not 0, it signals that the VU needs to be reinitialized. It must be added to and
	// read with atomics and helps to skip checking all the contexts and channels all the time.
	state int32
	// stateH []int32 // helper for debugging

	ctx, vuCtx       context.Context
	cancel, vuCancel func()
	logger           *logrus.Entry
}

func newStoppedVUHandle(
	parentCtx context.Context, getVU func() (lib.InitializedVU, error),
	returnVU func(lib.InitializedVU), config *BaseConfig, logger *logrus.Entry,
) *vuHandle {
	lock := &sync.Mutex{}
	ctx, cancel := context.WithCancel(parentCtx)

	vh := &vuHandle{
		mutex:     lock,
		parentCtx: parentCtx,
		getVU:     getVU,
		config:    config,

		canStartIter: make(chan struct{}),
		state:        stopped,

		ctx:    ctx,
		cancel: cancel,
		logger: logger,
	}

	// TODO maybe move the repeating parts in a function?
	vh.returnVU = func(v lib.InitializedVU) {
		vh.mutex.Lock()
		// the returnVU could be called after those have reinitialize, don't nil them if that is the
		// case, maybe just don't nil them
		if vh.state != starting && vh.state != running {
			vh.initVU = nil
			vh.activeVU = nil
		}
		vh.mutex.Unlock()
		returnVU(v)
	}

	return vh
}

// start is one of the more complicated ones as it can race with toGracefulStop and has special
// cases
// state changes
// toGracefulStop => running, we have raced with toGracefulStop if it was finished the state
// would've been stoppped
// starting, running => nothing, we have done what needs to be done
// stopped, toHardStop => reactivate everything, it is either stopped or hardStop has uninitialized
// some stuff and we need to reinitialize them
func (vh *vuHandle) start() (err error) {
	vh.mutex.Lock()
	if vh.state == toGracefulStop { // No point returning the VU just continue as it was
		close(vh.canStartIter)
		vh.changeState(running)
		vh.mutex.Unlock()
		return
	}

	if vh.state == starting || vh.state == running {
		vh.mutex.Unlock()
		return nil
	}

	vh.logger.Debug("Start")
	vh.initVU, err = vh.getVU()
	if err != nil {
		vh.mutex.Unlock()
		return err
	}
	vh.activateVU()
	close(vh.canStartIter)
	vh.changeState(starting)
	vh.mutex.Unlock()
	return nil
}

// this must be called with the mutex locked
func (vh *vuHandle) activateVU() {
	vh.vuCtx, vh.vuCancel = context.WithCancel(vh.ctx)
	vh.activeVU = vh.initVU.Activate(getVUActivationParams(vh.vuCtx, *vh.config, vh.returnVU))
}

// just a helper function for debugging
func (vh *vuHandle) changeState(newState int32) {
	// vh.stateH = append(vh.stateH, newState)
	atomic.StoreInt32(&vh.state, newState)
}

// gracefulStop
// state changes
// stopped,toGracefulStop,toHardStop - nothing to do, skip
// starting - we couldn't manage to make an iteration since start, so move to stopped and cancel the
// vuCtx
// running -> toGracefulStop
func (vh *vuHandle) gracefulStop() {
	vh.mutex.Lock()

	if vh.state == toGracefulStop || vh.state == toHardStop || vh.state == stopped {
		vh.mutex.Unlock()
		return
	}

	if vh.state == starting { // we raced with starting
		vh.vuCancel()
		vh.changeState(stopped)
	} else {
		vh.changeState(toGracefulStop)
	}
	vh.canStartIter = make(chan struct{})
	vh.logger.Debug("Graceful stop")

	vh.mutex.Unlock()
}

// hardStop
// state changes:
// stopped, toHardStop -> nothing to do
// else -> cancel the main context and move to toHardStop, reinitialize the main context
func (vh *vuHandle) hardStop() {
	vh.mutex.Lock()
	vh.logger.Debug("Hard stop")
	if vh.state == toHardStop || vh.state == stopped {
		vh.mutex.Unlock()
		return
	}

	vh.cancel() // cancel the previous context
	vh.changeState(toHardStop)
	vh.initVU = nil
	vh.ctx, vh.cancel = context.WithCancel(vh.parentCtx) // create a new context
	vh.canStartIter = make(chan struct{})
	vh.mutex.Unlock()
}

// runLoopsIfPossible this is where all the fun is :D. Unfortunately somewhere we need to check most
// of the cases and this is where
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
		vuCtx        context.Context
		cancel       func()
		vu           lib.ActiveVU
	)

	for {
		state := atomic.LoadInt32(&vh.state)
		if state == running && runIter(vuCtx, vu) { // fast path
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
			if cancel != nil {
				cancel() // signal to return the vu before we continue
			}
			// we have *now* stopped
			vh.changeState(stopped)
		}
		canStartIter, ctx := vh.canStartIter, vh.ctx
		vh.mutex.Unlock()

		// We're not running, but the executor isn't done yet, so we wait
		// for either one of those conditions.
		select {
		case <-canStartIter:
			vh.mutex.Lock()
			select {
			case <-vh.canStartIter: // we check again in case of race
				// reinitialize
				vu = vh.activeVU
				vuCtx = vh.vuCtx
				cancel = vh.vuCancel
				vh.changeState(running) // clear changes here
				vh.mutex.Unlock()
			default: // TODO find out if this is actually necessary anymore
				// well we got raced to here by something ... loop again ...
				vh.mutex.Unlock()
				continue
			}
		case <-ctx.Done():
			// hardStop was called, start a fresh iteration to get the new
			// context and signal channel
		case <-executorDone:
			// The whole executor is done, nothing more to do.
			return
		}
	}
}
