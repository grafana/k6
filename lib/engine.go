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

type vuEntry struct {
	VU     VU
	Cancel context.CancelFunc
}

// The Engine is the beating heart of K6.
type Engine struct {
	Runner  Runner
	Options Options

	Metrics    map[*stats.Metric]stats.Sink
	Thresholds map[string]Thresholds

	atTime    time.Duration
	vuEntries []*vuEntry
	vuMutex   sync.Mutex

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

	if o.VUsMax.Valid {
		if err := e.SetVUsMax(o.VUsMax.Int64); err != nil {
			return nil, err
		}
	}
	if o.VUs.Valid {
		if err := e.SetVUs(o.VUs.Int64); err != nil {
			return nil, err
		}
	}
	if o.Paused.Valid {
		e.SetPaused(o.Paused.Bool)
	}

	return e, nil
}

func (e *Engine) Run(ctx context.Context) error {
	e.running = true
	<-ctx.Done()
	e.running = false

	e.clearSubcontext()
	e.subwg.Wait()

	return nil
}

func (e *Engine) IsRunning() bool {
	return e.running
}

func (e *Engine) SetPaused(v bool) {
	e.paused = v
}

func (e *Engine) IsPaused() bool {
	return e.paused
}

func (e *Engine) SetVUs(v int64) error {
	if v < 0 {
		return errors.New("vus can't be negative")
	}
	if v > e.vusMax {
		return errors.New("more vus than allocated requested")
	}

	e.vuMutex.Lock()
	defer e.vuMutex.Unlock()

	// Scale up
	for i := e.vus; i < v; i++ {
		vu := e.vuEntries[i]
		if vu.Cancel != nil {
			panic(errors.New("fatal miscalculation: attempted to re-schedule active VU"))
		}

		ctx, cancel := context.WithCancel(e.subctx)
		vu.Cancel = cancel

		e.subwg.Add(1)
		go func() {
			e.subwg.Done()
			e.runVU(ctx, vu)
		}()
	}

	// Scale down
	for i := e.vus - 1; i >= v; i-- {
		vu := e.vuEntries[i]
		vu.Cancel()
		vu.Cancel = nil
	}

	e.vus = v
	return nil
}

func (e *Engine) GetVUs() int64 {
	return e.vus
}

func (e *Engine) SetVUsMax(v int64) error {
	if v < 0 {
		return errors.New("vus-max can't be negative")
	}
	if v < e.vus {
		return errors.New("can't reduce vus-max below vus")
	}

	e.vuMutex.Lock()
	defer e.vuMutex.Unlock()

	// Scale up
	for len(e.vuEntries) < int(v) {
		var entry vuEntry
		if e.Runner != nil {
			vu, err := e.Runner.NewVU()
			if err != nil {
				return err
			}
			entry.VU = vu
		}
		e.vuEntries = append(e.vuEntries, &entry)
	}

	// Scale down
	if len(e.vuEntries) > int(v) {
		e.vuEntries = e.vuEntries[:int(v)]
	}

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

func (e *Engine) runVU(ctx context.Context, vu *vuEntry) {
	// nil runners that produce nil VUs are used for testing.
	if vu.VU == nil {
		<-ctx.Done()
		return
	}

	for {
		_, _ = vu.VU.RunOnce(ctx)

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}
