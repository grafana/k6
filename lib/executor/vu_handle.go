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
		// Don't return the initialized VU back
		vh.mutex.Lock()
		select {
		case <-vh.parentCtx.Done():
			// we are done just ruturn the VU
			vh.initVU = nil
			vh.activeVU = nil
			atomic.StoreInt32(&vh.state, stopped) // change this to soemthign that exits ?
			vh.mutex.Unlock()
			returnVU(v)
		default:
			if vh.state == running { // we raced with start, it won .. lets not just return the VU for no reason
				vh.mutex.Unlock()
				return
			}
			select {
			case <-vh.canStartIter:
				// we can continue with itearting - lets not return the vu
				vh.activateVU() // we still need to reactivate it to get the new context and cancel
				atomic.StoreInt32(&vh.state, starting)
				vh.mutex.Unlock()
			default: // we actually have to return the vu
				vh.initVU = nil
				vh.activeVU = nil
				atomic.StoreInt32(&vh.state, stopped)
				vh.mutex.Unlock()
				returnVU(v)
			}
		}
	}

	return vh
}

func (vh *vuHandle) start() (err error) {
	vh.mutex.Lock()
	if vh.state == toGracefulStop { // No point returning the VU just continue as it was
		atomic.StoreInt32(&vh.state, running)
		vh.mutex.Unlock()
		return
	}

	if vh.state != starting && vh.state != running && vh.initVU == nil {
		vh.logger.Debug("Start")
		vh.initVU, err = vh.getVU()
		if err != nil {
			vh.mutex.Unlock()
			return err
		}
		vh.activateVU()
		atomic.StoreInt32(&vh.state, starting)
		close(vh.canStartIter)
	}
	vh.mutex.Unlock()
	return nil
}

// this must be called with the mutex locked
func (vh *vuHandle) activateVU() {
	vh.vuCtx, vh.vuCancel = context.WithCancel(vh.ctx)
	vh.activeVU = vh.initVU.Activate(getVUActivationParams(vh.vuCtx, *vh.config, vh.returnVU))
}

func (vh *vuHandle) gracefulStop() {
	vh.mutex.Lock()

	if vh.state != toGracefulStop && vh.state != toHardStop && vh.state != stopped {
		if vh.state == starting { // we raced with starting
			vh.vuCancel()
			atomic.StoreInt32(&vh.state, stopped)
		} else {
			atomic.StoreInt32(&vh.state, toGracefulStop)
		}
		vh.canStartIter = make(chan struct{})
		vh.logger.Debug("Graceful stop")
	}

	vh.mutex.Unlock()
}

func (vh *vuHandle) hardStop() {
	vh.mutex.Lock()
	vh.logger.Debug("Hard stop")
	if vh.state != toHardStop && vh.state != stopped {
		vh.cancel() // cancel the previous context
		atomic.StoreInt32(&vh.state, toHardStop)
		vh.initVU = nil
		vh.ctx, vh.cancel = context.WithCancel(vh.parentCtx) // create a new context
		vh.canStartIter = make(chan struct{})
	}
	vh.mutex.Unlock()
}

func (vh *vuHandle) runLoopsIfPossible(runIter func(context.Context, lib.ActiveVU) bool) {
	// We can probably initialize here, but it's also easier to just use the slow path in the second
	// part of the for loop
	var (
		executorDone = vh.parentCtx.Done()
		vuCtx        context.Context
		cancel       func()
		vu           lib.ActiveVU
	)

	for {
		state := atomic.LoadInt32(&vh.state)
		if vu != nil && state == running && runIter(vuCtx, vu) { // fast path
			continue
		}

		// slow path - something has changed - get what and wait until we can do more iterations

		select {
		case <-executorDone:
			// The whole executor is done, nothing more to do.
			return
		default:
		}

		vh.mutex.Lock()
		if vh.state == running { // start raced us to toGracefulStop
			vh.mutex.Unlock()
			continue
		}

		if cancel != nil {
			cancel() // signal to return the vu before we continue
		}
		if vh.state == toGracefulStop || vh.state == toHardStop {
			vh.state = stopped
		}
		canStartIter, ctx := vh.canStartIter, vh.ctx
		select {
		case <-executorDone:
			// The whole executor is done, nothing more to do.
			vh.mutex.Unlock()
			return
		default:
		}
		vh.mutex.Unlock()

		// We're not running, but the executor isn't done yet, so we wait
		// for either one of those conditions.

		select {
		case <-canStartIter:
			vh.mutex.Lock()
			select {
			case <-vh.canStartIter: // we check again in case of race
				// reinitialize
				if vh.activeVU == nil {
					// we've raced with the ReturnVU: we can continue doing iterations but
					// a stop has happened and returnVU hasn't managed to run yet ... so we loop
					// TODO call runtime.GoSched() in the else to give priority to possibly the returnVU goroutine
					vh.mutex.Unlock()
					continue
				}

				vu = vh.activeVU
				vuCtx = vh.vuCtx
				cancel = vh.vuCancel
				atomic.StoreInt32(&vh.state, running) // clear changes here
				vh.mutex.Unlock()
			default:
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
