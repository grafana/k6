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
	"sync"
	"sync/atomic"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/pkg/errors"
	null "gopkg.in/guregu/null.v3"
)

type vuHandle struct {
	VU     lib.VU
	Cancel context.CancelFunc
	Lock   sync.RWMutex

	runLock sync.Mutex
}

func (h *vuHandle) Run(ctx context.Context) error {
	h.Lock.Lock()
	_, cancel := context.WithCancel(ctx)
	h.Cancel = cancel
	h.Lock.Unlock()

	return nil
}

type Executor struct {
	Runner lib.Runner

	vus       []*vuHandle
	vusLock   sync.RWMutex
	numVUs    int64
	numVUsMax int64

	iterations, endIterations int64
	time, endTime             int64
	paused                    lib.AtomicBool

	runLock sync.Mutex
	ctx     context.Context
}

func New(r lib.Runner) *Executor {
	return &Executor{Runner: r}
}

func (e *Executor) Run(ctx context.Context, out <-chan []stats.Sample) error {
	e.runLock.Lock()
	defer e.runLock.Unlock()

	e.ctx = ctx
	<-ctx.Done()
	e.ctx = nil

	return nil
}

func (e *Executor) IsRunning() bool {
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
	return e.paused.Get()
}

func (e *Executor) SetPaused(paused bool) {
	e.paused.Set(paused)
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

	e.vusLock.Lock()
	defer e.vusLock.Unlock()

	for i, handle := range e.vus {
		if i <= int(num) {
			_, cancel := context.WithCancel(e.ctx)
			handle.Cancel = cancel
		} else if handle.Cancel != nil {
			handle.Cancel()
			handle.Cancel = nil
		}
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
			handle.VU = vu
		}
		vus = append(vus, &handle)
	}
	e.vus = vus

	atomic.StoreInt64(&e.numVUsMax, max)

	return nil
}
