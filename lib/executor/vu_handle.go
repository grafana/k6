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

// This is a helper type used in executors where we have to dynamically control
// the number of VUs that are simultaneously running. For the moment, it is used
// in the VariableLoopingVUs and the ExternallyControlled executors.
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
	// This is here only to signal that something has changed it must be added to and read with atomics
	// and helps to skip checking all the contexts and channels all the time
	change int32

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
		change:       1,

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
			atomic.StoreInt32(&vh.change, 1)
			vh.mutex.Unlock()
			returnVU(v)
		default:
			select {
			case <-vh.canStartIter:
				// we can continue with itearting - lets not return the vu
				vh.activateVU() // we still need to reactivate it to get the new context and cancel
				atomic.StoreInt32(&vh.change, 1)
				vh.mutex.Unlock()
			default: // we actually have to return the vu
				vh.initVU = nil
				vh.activeVU = nil
				atomic.StoreInt32(&vh.change, 1)
				vh.mutex.Unlock()
				returnVU(v)
			}
		}
	}

	return vh
}

func (vh *vuHandle) start() (err error) {
	vh.mutex.Lock()
	vh.logger.Debug("Start")
	if vh.initVU == nil {
		vh.initVU, err = vh.getVU()
		if err != nil {
			return err
		}
		vh.activateVU()
		atomic.AddInt32(&vh.change, 1)
	}
	select {
	case <-vh.canStartIter: // we are alrady started do nothing
	default:
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
	select {
	case <-vh.canStartIter:
		atomic.AddInt32(&vh.change, 1)
		vh.activeVU = nil
		vh.canStartIter = make(chan struct{})
		vh.logger.Debug("Graceful stop")
	default:
		// do nothing, the signalling channel was already initialized by hardStop()
	}
	vh.mutex.Unlock()
}

func (vh *vuHandle) hardStop() {
	vh.mutex.Lock()
	vh.logger.Debug("Hard stop")
	vh.cancel() // cancel the previous context
	atomic.AddInt32(&vh.change, 1)
	vh.initVU = nil
	vh.activeVU = nil
	vh.ctx, vh.cancel = context.WithCancel(vh.parentCtx) // create a new context
	select {
	case <-vh.canStartIter:
		vh.canStartIter = make(chan struct{})
	default:
		// do nothing, the signalling channel was already initialized by gracefulStop()
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
		ch := atomic.LoadInt32(&vh.change)
		if ch == 0 && runIter(vuCtx, vu) { // fast path
			continue
		}

		// slow path - something has changed - get what and wait until we can do more iterations
		// TODO: I think we can skip cancelling in some cases but I doubt it will do much good in most
		if cancel != nil {
			cancel() // signal to return the vu before we continue
		}
		vh.mutex.Lock()
		canStartIter, ctx := vh.canStartIter, vh.ctx
		cancel = vh.vuCancel
		vh.mutex.Unlock()

		select {
		case <-executorDone:
			// The whole executor is done, nothing more to do.
			return
		default:
		}
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
				atomic.StoreInt32(&vh.change, 0) // clear changes here
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
