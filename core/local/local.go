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
	null "gopkg.in/guregu/null.v3"
)

type VUHandle struct {
	VU      lib.VU
	Cancel  context.CancelFunc
	Samples []stats.Sample
}

type Executor struct {
	Runner lib.Runner
	VUs    []VUHandle

	runLock   sync.Mutex
	isRunning bool

	iterations, endIterations int64
	time, endTime             int64
	paused                    lib.AtomicBool
	vus, vusMax               int64
}

func New(r lib.Runner) *Executor {
	return &Executor{Runner: r}
}

func (e *Executor) Run(ctx context.Context, out <-chan []stats.Sample) error {
	e.runLock.Lock()
	e.isRunning = true
	defer func() {
		e.isRunning = false
		e.runLock.Unlock()
	}()

	<-ctx.Done()
	return nil
}

func (e *Executor) IsRunning() bool {
	return e.isRunning
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
	return atomic.LoadInt64(&e.vus)
}

func (e *Executor) SetVUs(vus int64) error {
	atomic.StoreInt64(&e.vus, vus)
	return nil
}

func (e *Executor) GetVUsMax() int64 {
	return atomic.LoadInt64(&e.vusMax)
}

func (e *Executor) SetVUsMax(max int64) error {
	atomic.StoreInt64(&e.vusMax, max)
	return nil
}
