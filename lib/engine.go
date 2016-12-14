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

package lib

import (
	"context"
	"errors"
	"github.com/loadimpact/k6/stats"
	"sync"
	"time"
)

const (
	TickRate          = 1 * time.Millisecond
	ThresholdTickRate = 2 * time.Second
	ShutdownTimeout   = 10 * time.Second
)

// Special error used to signal that a VU wants a taint, without logging an error.
var ErrVUWantsTaint = errors.New("test is tainted")

// The Engine is the beating heart of K6.
type Engine struct {
	Runner  Runner
	Options Options

	Metrics    map[*stats.Metric]stats.Sink
	Thresholds map[string]Thresholds

	atTime time.Duration

	// Stubbing these out to pass tests.
	running bool
	paused  bool
	vus     int64
	vusMax  int64

	// Subsystem-related.
	subctx    context.Context
	subcancel context.CancelFunc
	submutex  sync.Mutex
	subwg     sync.WaitGroup
}

func NewEngine(r Runner, o Options) (*Engine, error) {
	e := &Engine{
		Runner:  r,
		Options: o,

		Metrics:    make(map[*stats.Metric]stats.Sink),
		Thresholds: make(map[string]Thresholds),
	}
	e.clearSubcontext()

	return e, nil
}

func (e *Engine) Run(ctx context.Context) error {
	<-ctx.Done()
	e.clearSubcontext()
	e.subwg.Wait()
	return nil
}

func (e *Engine) SetRunning(v bool) {
	e.running = true
}

func (e *Engine) IsRunning() bool {
	return e.running
}

func (e *Engine) SetPaused(v bool) {
	e.paused = true
}

func (e *Engine) IsPaused() bool {
	return e.paused
}

func (e *Engine) SetVUs(v int64) error {
	if v > e.vusMax {
		return errors.New("more VUs than allocated requested")
	}

	e.vus = v
	return nil
}

func (e *Engine) GetVUs() int64 {
	return e.vus
}

func (e *Engine) SetVUsMax(v int64) error {
	e.vusMax = v
	return nil
}

func (e *Engine) GetVUsMax() int64 {
	return e.vusMax
}

func (e *Engine) IsTainted() bool {
	return false
}

func (e *Engine) AtTime() time.Duration {
	return e.atTime
}

func (e *Engine) TotalTime() (time.Duration, bool) {
	return 0, false
}

func (e *Engine) clearSubcontext() {
	e.submutex.Lock()
	defer e.submutex.Unlock()

	if e.subcancel != nil {
		e.subcancel()
	}
	subctx, subcancel := context.WithCancel(context.Background())
	e.subctx = subctx
	e.subcancel = subcancel
}
