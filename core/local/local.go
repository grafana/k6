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

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	null "gopkg.in/guregu/null.v3"
)

var _ lib.Executor = &Executor{}

type vuHandle struct {
	sync.RWMutex
	vu     lib.VU
	ctx    context.Context
	cancel context.CancelFunc
}

func (h *vuHandle) run(logger *log.Logger, flow <-chan int64, out chan<- []stats.Sample) {
	h.RLock()
	ctx := h.ctx
	h.RUnlock()

	for {
		select {
		case _, ok := <-flow:
			if !ok {
				return
			}
		case <-ctx.Done():
			return
		}

		var samples []stats.Sample
		if h.vu != nil {
			s, err := h.vu.RunOnce(ctx)
			if err != nil {
				if s, ok := err.(fmt.Stringer); ok {
					logger.Error(s.String())
				} else {
					logger.Error(err.Error())
				}
			}
			samples = s
		}
		out <- samples
	}
}

type Executor struct {
	Runner lib.Runner
	Logger *log.Logger

	runLock sync.Mutex
	wg      sync.WaitGroup

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

	// Lock for: ctx, flow, out
	lock sync.RWMutex

	// Current context, nil if a test isn't running right now.
	ctx context.Context

	// Engineward output channel for samples.
	out chan<- []stats.Sample

	// Flow control for VUs; iterations are run only after reading from this channel.
	flow chan int64
}

func New(r lib.Runner) *Executor {
	return &Executor{
		Runner:   r,
		Logger:   log.StandardLogger(),
		endIters: -1,
		endTime:  -1,
	}
}

func (e *Executor) Run(parent context.Context, out chan<- []stats.Sample) error {
	e.runLock.Lock()
	defer e.runLock.Unlock()

	ctx, cancel := context.WithCancel(parent)
	vuOut := make(chan []stats.Sample)
	vuFlow := make(chan int64)

	e.lock.Lock()
	e.ctx = ctx
	e.out = vuOut
	e.flow = vuFlow
	e.lock.Unlock()

	var cutoff time.Time
	defer func() {
		close(vuFlow)
		cancel()

		e.lock.Lock()
		e.ctx = nil
		e.out = nil
		e.flow = nil
		e.lock.Unlock()

		wait := make(chan interface{})
		go func() {
			e.wg.Wait()
			close(wait)
		}()

		var samples []stats.Sample
		for {
			select {
			case ss := <-vuOut:
				for _, s := range ss {
					if cutoff.IsZero() || s.Time.Before(cutoff) {
						samples = append(samples, s)
					}
				}
			case <-wait:
			}
			select {
			case <-wait:
				close(vuOut)
				if out != nil && len(samples) > 0 {
					out <- samples
				}
				return
			default:
			}
		}
	}()

	if err := e.scale(ctx, lib.Max(0, atomic.LoadInt64(&e.numVUs))); err != nil {
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
			// Every tick, increment the clock and see if we passed the end point. If the test ends
			// this way, set a cutoff point; any samples collected past the cutoff point are excluded.
			d := t.Sub(lastTick)
			lastTick = t

			end := time.Duration(atomic.LoadInt64(&e.endTime))
			at := time.Duration(atomic.AddInt64(&e.time, int64(d)))
			if end >= 0 && at >= end {
				e.Logger.WithFields(log.Fields{"at": at, "end": end}).Debug("Local: Hit time limit")
				cutoff = time.Now()
				return nil
			}
		case samples := <-vuOut:
			// Every iteration ends with a write to vuOut. Check if we've hit the end point.
			if out != nil {
				out <- samples
			}

			end := atomic.LoadInt64(&e.endIters)
			at := atomic.AddInt64(&e.iters, 1)
			if end >= 0 && at >= end {
				e.Logger.WithFields(log.Fields{"at": at, "end": end}).Debug("Local: Hit iteration limit")
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
	e.vusLock.Lock()
	defer e.vusLock.Unlock()

	e.lock.RLock()
	flow := e.flow
	out := e.out
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
					handle.run(e.Logger, flow, out)
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

func (e *Executor) SetLogger(l *log.Logger) {
	e.Logger = l
}

func (e *Executor) GetLogger() *log.Logger {
	return e.Logger
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
	atomic.StoreInt64(&e.endIters, i.Int64)
}

func (e *Executor) GetTime() time.Duration {
	return time.Duration(atomic.LoadInt64(&e.time))
}

func (e *Executor) GetEndTime() lib.NullDuration {
	v := atomic.LoadInt64(&e.endTime)
	if v < 0 {
		return lib.NullDuration{}
	}
	return lib.NullDurationFrom(time.Duration(v))
}

func (e *Executor) SetEndTime(t lib.NullDuration) {
	if !t.Valid {
		t.Duration = -1
	}
	atomic.StoreInt64(&e.endTime, int64(t.Duration))
}

func (e *Executor) IsPaused() bool {
	e.pauseLock.RLock()
	defer e.pauseLock.RUnlock()
	return e.pause != nil
}

func (e *Executor) SetPaused(paused bool) {
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

	e.vusLock.Lock()
	defer e.vusLock.Unlock()

	vus := e.vus
	for i := numVUsMax; i < max; i++ {
		var handle vuHandle
		if e.Runner != nil {
			vu, err := e.Runner.NewVU()
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
