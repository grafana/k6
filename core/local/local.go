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

package local

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	null "gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
)

// TODO: totally rewrite this!
// This is an overcomplicated and probably buggy piece of code that is a major PITA to refactor...
// It does a ton of stuff in a very convoluted way, has a and uses a very incomprehensible mix
// of all possible Go synchronization mechanisms (channels, mutexes, rwmutexes, atomics,
// and waitgroups) and has a bunch of contexts and tickers on top...

var _ lib.Executor = &Executor{}

type vuHandle struct {
	sync.RWMutex
	vu     lib.VU
	ctx    context.Context
	cancel context.CancelFunc
}

func (h *vuHandle) run(logger *logrus.Logger, flow <-chan int64, iterDone chan<- struct{}) {
	h.RLock()
	ctx := h.ctx
	h.RUnlock()

	for {
		if _, ok := <-flow; !ok || ctx.Err() != nil {
			return
		}

		if h.vu != nil {
			err := h.vu.RunOnce(ctx)
			if ctx.Err() != nil {
				// Don't log errors or emit iterations metrics from cancelled iterations
				return
			}
			if err != nil {
				errMsg := err.Error()
				if s, ok := err.(fmt.Stringer); ok {
					errMsg = s.String()
				}
				logger.Error(errMsg)
			}
		}
		iterDone <- struct{}{}

	}
}

type Executor struct {
	Runner lib.Runner
	Logger *logrus.Logger

	runLock sync.Mutex
	wg      sync.WaitGroup

	runSetup    bool
	runTeardown bool

	vus       []*vuHandle
	vusLock   sync.RWMutex
	numVUs    int64
	numVUsMax int64
	nextVUID  int64

	iters     int64 // Completed iterations
	partIters int64 // Partial, incomplete iterations
	endIters  int64 // End test at this many iterations

	time    int64 // Current time
	endTime int64 // End test at this timestamp

	pauseLock sync.RWMutex
	pause     chan interface{}

	stages []lib.Stage

	// Lock for: ctx, flow, out
	lock sync.RWMutex

	// Current context, nil if a test isn't running right now.
	ctx context.Context

	// Output channel to which VUs send samples.
	vuOut chan stats.SampleContainer

	// Channel on which VUs sigal that iterations are completed
	iterDone chan struct{}

	// Flow control for VUs; iterations are run only after reading from this channel.
	flow chan int64
}

func New(r lib.Runner) *Executor {
	var bufferSize int64
	if r != nil {
		bufferSize = r.GetOptions().MetricSamplesBufferSize.Int64
	}

	return &Executor{
		Runner:      r,
		Logger:      logrus.StandardLogger(),
		runSetup:    true,
		runTeardown: true,
		endIters:    -1,
		endTime:     -1,
		vuOut:       make(chan stats.SampleContainer, bufferSize),
		iterDone:    make(chan struct{}),
	}
}

func (e *Executor) Run(parent context.Context, engineOut chan<- stats.SampleContainer) (reterr error) {
	e.runLock.Lock()
	defer e.runLock.Unlock()

	if e.Runner != nil && e.runSetup {
		if err := e.Runner.Setup(parent, engineOut); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithCancel(parent)
	vuFlow := make(chan int64)
	e.lock.Lock()
	vuOut := e.vuOut
	iterDone := e.iterDone
	e.ctx = ctx
	e.flow = vuFlow
	e.lock.Unlock()

	var cutoff time.Time
	defer func() {
		if e.Runner != nil && e.runTeardown {
			err := e.Runner.Teardown(parent, engineOut)
			if reterr == nil {
				reterr = err
			} else if err != nil {
				reterr = fmt.Errorf("teardown error %#v\nPrevious error: %#v", err, reterr)
			}
		}

		close(vuFlow)
		cancel()

		e.lock.Lock()
		e.ctx = nil
		e.vuOut = nil
		e.flow = nil
		e.lock.Unlock()

		wait := make(chan interface{})
		go func() {
			e.wg.Wait()
			close(wait)
		}()

		for {
			select {
			case <-iterDone:
				// Spool through all remaining iterations, do not emit stats since the Run() is over
			case newSampleContainer := <-vuOut:
				if cutoff.IsZero() {
					engineOut <- newSampleContainer
				} else if csc, ok := newSampleContainer.(stats.ConnectedSampleContainer); ok && csc.GetTime().Before(cutoff) {
					engineOut <- newSampleContainer
				} else {
					for _, s := range newSampleContainer.GetSamples() {
						if s.Time.Before(cutoff) {
							engineOut <- s
						}
					}
				}
			case <-wait:
			}
			select {
			case <-wait:
				close(vuOut)
				return
			default:
			}
		}
	}()

	startVUs := atomic.LoadInt64(&e.numVUs)
	if err := e.scale(ctx, lib.Max(0, startVUs)); err != nil {
		return err
	}

	ticker := time.NewTicker(1 * time.Millisecond)
	defer ticker.Stop()

	lastTick := time.Now()
	for {
		// If the test is paused, sleep until either the pause or the test ends.
		// Also shift the last tick to omit time spent paused, but not partial ticks.
		e.pauseLock.RLock()
		pause := e.pause
		e.pauseLock.RUnlock()
		if pause != nil {
			e.Logger.Debug("Local: Pausing!")
			leftovers := time.Since(lastTick)
			select {
			case <-pause:
				e.Logger.Debug("Local: No longer paused")
				lastTick = time.Now().Add(-leftovers)
			case <-ctx.Done():
				e.Logger.Debug("Local: Terminated while in paused state")
				return nil
			}
		}

		// Dumb hack: we don't wanna start any more iterations than the max, but we can't
		// conditionally select on a channel either...so, we cheat: swap out the flow channel for a
		// nil channel (writing to nil always blocks) if we don't wanna write an iteration.
		flow := vuFlow
		end := atomic.LoadInt64(&e.endIters)
		partials := atomic.LoadInt64(&e.partIters)
		if end >= 0 && partials >= end {
			flow = nil
		}

		select {
		case flow <- partials:
			// Start an iteration if there's a VU waiting. See also: the big comment block above.
			atomic.AddInt64(&e.partIters, 1)
		case t := <-ticker.C:
			// Every tick, increment the clock, see if we passed the end point, and process stages.
			// If the test ends this way, set a cutoff point; any samples collected past the cutoff
			// point are excluded.
			d := t.Sub(lastTick)
			lastTick = t

			end := time.Duration(atomic.LoadInt64(&e.endTime))
			at := time.Duration(atomic.AddInt64(&e.time, int64(d)))
			if end >= 0 && at >= end {
				e.Logger.WithFields(logrus.Fields{"at": at, "end": end}).Debug("Local: Hit time limit")
				cutoff = time.Now()
				return nil
			}

			stages := e.stages
			if len(stages) > 0 {
				vus, keepRunning := ProcessStages(startVUs, stages, at)
				if !keepRunning {
					e.Logger.WithField("at", at).Debug("Local: Ran out of stages")
					cutoff = time.Now()
					return nil
				}
				if vus.Valid {
					if err := e.SetVUs(vus.Int64); err != nil {
						return err
					}
				}
			}
		case sampleContainer := <-vuOut:
			engineOut <- sampleContainer
		case <-iterDone:
			// Every iteration ends with a write to iterDone. Check if we've hit the end point.
			// If not, make sure to include an Iterations bump in the list!
			end := atomic.LoadInt64(&e.endIters)
			at := atomic.AddInt64(&e.iters, 1)
			if end >= 0 && at >= end {
				e.Logger.WithFields(logrus.Fields{"at": at, "end": end}).Debug("Local: Hit iteration limit")
				return nil
			}
		case <-ctx.Done():
			// If the test is cancelled, just set the cutoff point to now and proceed down the same
			// logic as if the time limit was hit.
			e.Logger.Debug("Local: Exiting with context")
			cutoff = time.Now()
			return nil
		}
	}
}

func (e *Executor) scale(ctx context.Context, num int64) error {
	e.Logger.WithField("num", num).Debug("Local: Scaling...")

	e.vusLock.Lock()
	defer e.vusLock.Unlock()

	e.lock.RLock()
	flow := e.flow
	iterDone := e.iterDone
	e.lock.RUnlock()

	for i, handle := range e.vus {
		handle := handle
		handle.RLock()
		cancel := handle.cancel
		handle.RUnlock()

		if i < int(num) {
			if cancel == nil {
				vuctx, cancel := context.WithCancel(ctx)
				handle.Lock()
				handle.ctx = vuctx
				handle.cancel = cancel
				handle.Unlock()

				if handle.vu != nil {
					if err := handle.vu.Reconfigure(atomic.AddInt64(&e.nextVUID, 1)); err != nil {
						return err
					}
				}

				e.wg.Add(1)
				go func() {
					handle.run(e.Logger, flow, iterDone)
					e.wg.Done()
				}()
			}
		} else if cancel != nil {
			handle.Lock()
			handle.cancel()
			handle.cancel = nil
			handle.Unlock()
		}
	}

	atomic.StoreInt64(&e.numVUs, num)
	return nil
}

func (e *Executor) IsRunning() bool {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.ctx != nil
}

func (e *Executor) GetRunner() lib.Runner {
	return e.Runner
}

// SetLogger sets Executor's logger.
func (e *Executor) SetLogger(l *logrus.Logger) {
	e.Logger = l
}

// GetLogger returns current Executor's logger.
func (e *Executor) GetLogger() *logrus.Logger {
	return e.Logger
}

func (e *Executor) GetStages() []lib.Stage {
	return e.stages
}

func (e *Executor) SetStages(s []lib.Stage) {
	e.stages = s
}

func (e *Executor) GetIterations() int64 {
	return atomic.LoadInt64(&e.iters)
}

func (e *Executor) GetEndIterations() null.Int {
	v := atomic.LoadInt64(&e.endIters)
	if v < 0 {
		return null.Int{}
	}
	return null.IntFrom(v)
}

func (e *Executor) SetEndIterations(i null.Int) {
	if !i.Valid {
		i.Int64 = -1
	}
	e.Logger.WithField("i", i.Int64).Debug("Local: Setting end iterations")
	atomic.StoreInt64(&e.endIters, i.Int64)
}

func (e *Executor) GetTime() time.Duration {
	return time.Duration(atomic.LoadInt64(&e.time))
}

func (e *Executor) GetEndTime() types.NullDuration {
	v := atomic.LoadInt64(&e.endTime)
	if v < 0 {
		return types.NullDuration{}
	}
	return types.NullDurationFrom(time.Duration(v))
}

func (e *Executor) SetEndTime(t types.NullDuration) {
	if !t.Valid {
		t.Duration = -1
	}
	e.Logger.WithField("d", t.Duration).Debug("Local: Setting end time")
	atomic.StoreInt64(&e.endTime, int64(t.Duration))
}

func (e *Executor) IsPaused() bool {
	e.pauseLock.RLock()
	defer e.pauseLock.RUnlock()
	return e.pause != nil
}

func (e *Executor) SetPaused(paused bool) {
	e.Logger.WithField("paused", paused).Debug("Local: Setting paused")
	e.pauseLock.Lock()
	defer e.pauseLock.Unlock()

	if paused && e.pause == nil {
		e.pause = make(chan interface{})
	} else if !paused && e.pause != nil {
		close(e.pause)
		e.pause = nil
	}
}

func (e *Executor) GetVUs() int64 {
	return atomic.LoadInt64(&e.numVUs)
}

func (e *Executor) SetVUs(num int64) error {
	if num < 0 {
		return errors.New("vu count can't be negative")
	}

	if atomic.LoadInt64(&e.numVUs) == num {
		return nil
	}

	e.Logger.WithField("vus", num).Debug("Local: Setting VUs")

	if numVUsMax := atomic.LoadInt64(&e.numVUsMax); num > numVUsMax {
		return errors.Errorf("can't raise vu count (to %d) above vu cap (%d)", num, numVUsMax)
	}

	if ctx := e.ctx; ctx != nil {
		if err := e.scale(ctx, num); err != nil {
			return err
		}
	} else {
		atomic.StoreInt64(&e.numVUs, num)
	}

	return nil
}

func (e *Executor) GetVUsMax() int64 {
	return atomic.LoadInt64(&e.numVUsMax)
}

func (e *Executor) SetVUsMax(max int64) error {
	e.Logger.WithField("max", max).Debug("Local: Setting max VUs")
	if max < 0 {
		return errors.New("vu cap can't be negative")
	}

	numVUsMax := atomic.LoadInt64(&e.numVUsMax)

	if numVUsMax == max {
		return nil
	}

	if numVUs := atomic.LoadInt64(&e.numVUs); max < numVUs {
		return errors.Errorf("can't lower vu cap (to %d) below vu count (%d)", max, numVUs)
	}

	if max < numVUsMax {
		e.vus = e.vus[:max]
		atomic.StoreInt64(&e.numVUsMax, max)
		return nil
	}

	e.lock.RLock()
	vuOut := e.vuOut
	e.lock.RUnlock()

	e.vusLock.Lock()
	defer e.vusLock.Unlock()

	vus := e.vus
	for i := numVUsMax; i < max; i++ {
		var handle vuHandle
		if e.Runner != nil {
			vu, err := e.Runner.NewVU(vuOut)
			if err != nil {
				return err
			}
			handle.vu = vu
		}
		vus = append(vus, &handle)
	}
	e.vus = vus

	atomic.StoreInt64(&e.numVUsMax, max)

	return nil
}

func (e *Executor) SetRunSetup(r bool) {
	e.runSetup = r
}

func (e *Executor) SetRunTeardown(r bool) {
	e.runTeardown = r
}
