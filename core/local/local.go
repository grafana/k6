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

type vuHandle struct {
	sync.RWMutex
	vu     lib.VU
	ctx    context.Context
	cancel context.CancelFunc
}

func (h *vuHandle) run(logger *log.Logger, flow <-chan struct{}, out chan<- []stats.Sample) {
	h.RLock()
	ctx := h.ctx
	h.RUnlock()

	for {
		select {
		case <-flow:
		case <-ctx.Done():
			return
		}

		samples, err := h.vu.RunOnce(ctx)
		if err != nil {
			if s, ok := err.(fmt.Stringer); ok {
				logger.Error(s.String())
			} else {
				logger.Error(err.Error())
			}
			continue
		}

		select {
		case out <- samples:
		case <-ctx.Done():
			return
		}
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

	iterations, endIterations int64
	time, endTime             int64

	pauseLock sync.RWMutex
	pause     chan interface{}

	// Lock for: ctx, flow, out
	lock sync.RWMutex

	// Current context, nil if a test isn't running right now.
	ctx context.Context

	// Engineward output channel for samples.
	out chan<- []stats.Sample

	// Flow control for VUs; iterations are run only after reading from this channel.
	flow chan struct{}
}

func New(r lib.Runner) *Executor {
	return &Executor{
		Runner:        r,
		Logger:        log.StandardLogger(),
		endIterations: -1,
		endTime:       -1,
	}
}

func (e *Executor) Run(parent context.Context, out chan<- []stats.Sample) error {
	e.runLock.Lock()
	defer e.runLock.Unlock()

	ctx, cancel := context.WithCancel(parent)

	e.lock.Lock()
	e.ctx = ctx
	e.out = out
	e.flow = make(chan struct{})
	e.lock.Unlock()

	defer func() {
		cancel()

		e.lock.Lock()
		e.ctx = nil
		e.out = nil
		e.flow = nil
		e.lock.Unlock()

		e.wg.Wait()
	}()

	e.scale(ctx, lib.Max(0, atomic.LoadInt64(&e.numVUs)))

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
			leftovers := time.Since(lastTick)
			select {
			case <-pause:
				lastTick = time.Now().Add(-leftovers)
			case <-ctx.Done():
				return nil
			}
		}

		select {
		case t := <-ticker.C:
			d := t.Sub(lastTick)
			lastTick = t

			at := atomic.AddInt64(&e.time, int64(d))
			end := atomic.LoadInt64(&e.endTime)
			if end >= 0 && at >= end {
				return nil
			}
		case e.flow <- struct{}{}:
			at := atomic.AddInt64(&e.iterations, 1)
			end := atomic.LoadInt64(&e.endIterations)
			if end >= 0 && at >= end {
				return nil
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (e *Executor) scale(ctx context.Context, num int64) {
	e.lock.RLock()
	flow := e.flow
	out := e.out
	e.lock.RUnlock()

	e.vusLock.Lock()
	defer e.vusLock.Unlock()

	for i, handle := range e.vus {
		handle := handle
		if i <= int(num) && handle.cancel == nil {
			vuctx, cancel := context.WithCancel(ctx)
			handle.Lock()
			handle.ctx = vuctx
			handle.cancel = cancel
			handle.Unlock()

			e.wg.Add(1)
			go func() {
				handle.run(e.Logger, flow, out)
				e.wg.Done()
			}()
		} else if handle.cancel != nil {
			handle.cancel()
			handle.cancel = nil
		}
	}
}

func (e *Executor) IsRunning() bool {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.ctx != nil
}

func (e *Executor) GetIterations() int64 {
	return atomic.LoadInt64(&e.iterations)
}

func (e *Executor) GetEndIterations() null.Int {
	v := atomic.LoadInt64(&e.endIterations)
	if v < 0 {
		return null.Int{}
	}
	return null.IntFrom(v)
}

func (e *Executor) SetEndIterations(i null.Int) {
	if !i.Valid {
		i.Int64 = -1
	}
	atomic.StoreInt64(&e.endIterations, i.Int64)
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

	if numVUsMax := atomic.LoadInt64(&e.numVUsMax); num > numVUsMax {
		return errors.Errorf("can't raise vu count (to %d) above vu cap (%d)", num, numVUsMax)
	}

	if ctx := e.ctx; ctx != nil {
		e.scale(ctx, num)
	}

	atomic.StoreInt64(&e.numVUs, num)

	return nil
}

func (e *Executor) GetVUsMax() int64 {
	return atomic.LoadInt64(&e.numVUsMax)
}

func (e *Executor) SetVUsMax(max int64) error {
	if max < 0 {
		return errors.New("vu cap can't be negative")
	}

	if numVUs := atomic.LoadInt64(&e.numVUs); max < numVUs {
		return errors.Errorf("can't lower vu cap (to %d) below vu count (%d)", max, numVUs)
	}

	numVUsMax := atomic.LoadInt64(&e.numVUsMax)

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
