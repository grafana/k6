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
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/stats"
	"github.com/pkg/errors"
	"gopkg.in/guregu/null.v3"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	TickRate        = 1 * time.Millisecond
	MetricsRate     = 1 * time.Second
	CollectRate     = 10 * time.Millisecond
	ThresholdsRate  = 2 * time.Second
	ShutdownTimeout = 10 * time.Second

	BackoffAmount = 50 * time.Millisecond
	BackoffMax    = 10 * time.Second
)

type vuEntry struct {
	VU     VU
	Cancel context.CancelFunc

	Samples    []stats.Sample
	Iterations int64
	lock       sync.Mutex
}

type submetric struct {
	Name       string
	Conditions map[string]string
	Metric     *stats.Metric
}

func parseSubmetric(name string) (string, map[string]string) {
	halves := strings.SplitN(strings.TrimSuffix(name, "}"), "{", 2)
	if len(halves) != 2 {
		return halves[0], nil
	}

	kvs := strings.Split(halves[1], ",")
	conditions := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		if kv == "" {
			continue
		}

		parts := strings.SplitN(kv, ":", 2)

		key := strings.TrimSpace(strings.Trim(parts[0], `"'`))
		if len(parts) != 2 {
			conditions[key] = ""
			continue
		}

		value := strings.TrimSpace(strings.Trim(parts[1], `"'`))
		conditions[key] = value
	}
	return halves[0], conditions
}

// The Engine is the beating heart of K6.
type Engine struct {
	Runner    Runner
	Options   Options
	Collector Collector
	Logger    *log.Logger

	Stages      []Stage
	Thresholds  map[string]Thresholds
	Metrics     map[*stats.Metric]stats.Sink
	MetricsLock sync.Mutex

	// Submetrics, mapped from parent metric names.
	submetrics map[string][]*submetric

	// Stage tracking.
	atTime          time.Duration
	atStage         int
	atStageSince    time.Duration
	atStageStartVUs int64

	// VU tracking.
	vus       int64
	vusMax    int64
	vuEntries []*vuEntry
	vuStop    chan interface{}
	vuPause   chan interface{}

	nextVUID int64

	// Atomic counters.
	numIterations int64
	numErrors     int64

	thresholdsTainted bool

	// Subsystem-related.
	lock      sync.Mutex
	subctx    context.Context
	subcancel context.CancelFunc
	subwg     sync.WaitGroup
}

func NewEngine(r Runner, o Options) (*Engine, error) {
	e := &Engine{
		Runner:  r,
		Options: o,
		Logger:  log.StandardLogger(),

		Metrics:    make(map[*stats.Metric]stats.Sink),
		Thresholds: make(map[string]Thresholds),

		vuStop: make(chan interface{}),
	}
	e.clearSubcontext()

	if o.Duration.Valid {
		d, err := time.ParseDuration(o.Duration.String)
		if err != nil {
			return nil, errors.Wrap(err, "options.duration")
		}
		e.Stages = []Stage{{Duration: d}}
	}
	if o.Stages != nil {
		e.Stages = o.Stages
	}
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

	if o.Thresholds != nil {
		e.Thresholds = o.Thresholds
		e.submetrics = make(map[string][]*submetric)

		for name := range e.Thresholds {
			if !strings.Contains(name, "{") {
				continue
			}

			parent, conds := parseSubmetric(name)
			e.submetrics[parent] = append(e.submetrics[parent], &submetric{
				Name:       name,
				Conditions: conds,
			})
		}
	}

	return e, nil
}

func (e *Engine) Run(ctx context.Context) error {
	collectorctx, collectorcancel := context.WithCancel(ctx)
	collectorch := make(chan interface{})
	if e.Collector != nil {
		go func(ctx context.Context) {
			e.Collector.Run(ctx)
			close(collectorch)
		}(collectorctx)
	} else {
		close(collectorch)
	}

	e.lock.Lock()
	{
		// Run metrics emission.
		e.subwg.Add(1)
		go func(ctx context.Context) {
			e.runMetricsEmission(ctx)
			e.subwg.Done()
		}(e.subctx)

		// Run metrics collection.
		e.subwg.Add(1)
		go func(ctx context.Context) {
			e.runCollection(ctx)
			e.subwg.Done()
		}(e.subctx)

		// Run thresholds.
		e.subwg.Add(1)
		go func(ctx context.Context) {
			e.runThresholds(ctx)
			e.subwg.Done()
		}(e.subctx)
	}
	e.lock.Unlock()

	close(e.vuStop)
	defer func() {
		e.vuStop = make(chan interface{})
		e.SetPaused(false)

		// Shut down subsystems, wait for graceful termination.
		e.clearSubcontext()
		e.subwg.Wait()

		// Process any leftover samples.
		e.processSamples(e.collect()...)

		// Emit final metrics.
		e.emitMetrics()

		// Shut down collector
		collectorcancel()
		<-collectorch
	}()

	// Set tracking to defaults.
	e.lock.Lock()
	e.atTime = 0
	e.atStage = 0
	e.atStageSince = 0
	e.atStageStartVUs = e.vus
	e.nextVUID = 0
	e.numIterations = 0
	e.numErrors = 0
	e.lock.Unlock()

	var lastTick time.Time
	ticker := time.NewTicker(TickRate)

	maxIterations := e.Options.Iterations.Int64
	for {
		// Don't do anything while the engine is paused.
		vuPause := e.vuPause
		if vuPause != nil {
			select {
			case <-vuPause:
			case <-ctx.Done():
				return nil
			}
		}

		// If we have an iteration cap, exit once we hit it.
		if maxIterations > 0 && e.numIterations == e.vusMax*maxIterations {
			return nil
		}

		// Calculate the time delta between now and the last tick.
		now := time.Now()
		if lastTick.IsZero() {
			lastTick = now
		}
		dT := now.Sub(lastTick)
		lastTick = now

		// Update state.
		keepRunning, err := e.processStages(dT)
		if err != nil {
			return err
		}
		if !keepRunning {
			return nil
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil
		}
	}
}

func (e *Engine) IsRunning() bool {
	select {
	case <-e.vuStop:
		return true
	default:
		return false
	}
}

func (e *Engine) SetPaused(v bool) {
	e.lock.Lock()
	defer e.lock.Unlock()

	if v && e.vuPause == nil {
		e.vuPause = make(chan interface{})
	} else if !v && e.vuPause != nil {
		close(e.vuPause)
		e.vuPause = nil
	}
}

func (e *Engine) IsPaused() bool {
	e.lock.Lock()
	defer e.lock.Unlock()

	return e.vuPause != nil
}

func (e *Engine) SetVUs(v int64) error {
	if v < 0 {
		return errors.New("vus can't be negative")
	}

	e.lock.Lock()
	defer e.lock.Unlock()

	if v > e.vusMax {
		return errors.New("more vus than allocated requested")
	}

	// Scale up
	for i := e.vus; i < v; i++ {
		vu := e.vuEntries[i]
		if vu.Cancel != nil {
			panic(errors.New("fatal miscalculation: attempted to re-schedule active VU"))
		}

		id := atomic.AddInt64(&e.nextVUID, 1)

		// nil runners are used for testing.
		if vu.VU != nil {
			if err := vu.VU.Reconfigure(id); err != nil {
				return err
			}
		}

		ctx, cancel := context.WithCancel(e.subctx)
		vu.Cancel = cancel

		e.subwg.Add(1)
		go func() {
			e.runVU(ctx, vu)
			e.subwg.Done()
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
	e.lock.Lock()
	defer e.lock.Unlock()

	return e.vus
}

func (e *Engine) SetVUsMax(v int64) error {
	if v < 0 {
		return errors.New("vus-max can't be negative")
	}

	e.lock.Lock()
	defer e.lock.Unlock()

	if v < e.vus {
		return errors.New("can't reduce vus-max below vus")
	}

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
	e.lock.Lock()
	defer e.lock.Unlock()

	return e.vusMax
}

func (e *Engine) IsTainted() bool {
	e.MetricsLock.Lock()
	defer e.MetricsLock.Unlock()

	return e.thresholdsTainted
}

func (e *Engine) AtTime() time.Duration {
	e.lock.Lock()
	defer e.lock.Unlock()

	return e.atTime
}

func (e *Engine) TotalTime() time.Duration {
	e.lock.Lock()
	defer e.lock.Unlock()

	var total time.Duration
	for _, stage := range e.Stages {
		if stage.Duration <= 0 {
			return 0
		}
		total += stage.Duration
	}
	return total
}

func (e *Engine) clearSubcontext() {
	e.lock.Lock()
	defer e.lock.Unlock()

	if e.subcancel != nil {
		e.subcancel()
	}
	subctx, subcancel := context.WithCancel(context.Background())
	e.subctx = subctx
	e.subcancel = subcancel
}

func (e *Engine) processStages(dT time.Duration) (bool, error) {
	e.lock.Lock()
	defer e.lock.Unlock()

	e.atTime += dT

	// If there are no stages, just keep going indefinitely at a stable VU count.
	if len(e.Stages) == 0 {
		return true, nil
	}

	stage := e.Stages[e.atStage]
	if stage.Duration > 0 && e.atTime > e.atStageSince+stage.Duration {
		if e.atStage != len(e.Stages)-1 {
			e.atStage++
			e.atStageSince = e.atTime
			e.atStageStartVUs = e.vus
			stage = e.Stages[e.atStage]
		} else {
			return false, nil
		}
	}
	if stage.Target.Valid {
		from := e.atStageStartVUs
		to := stage.Target.Int64
		t := 1.0
		if stage.Duration > 0 {
			t = Clampf(float64(e.atTime)/float64(e.atStageSince+stage.Duration), 0.0, 1.0)
		}
		if err := e.SetVUs(Lerp(from, to, t)); err != nil {
			return false, errors.Wrapf(err, "stage #%d", e.atStage+1)
		}
	}

	return true, nil
}

func (e *Engine) runVU(ctx context.Context, vu *vuEntry) {
	maxIterations := e.Options.Iterations.Int64

	// nil runners that produce nil VUs are used for testing.
	if vu.VU == nil {
		<-ctx.Done()
		return
	}

	// Sleep until the engine starts running.
	<-e.vuStop

	backoffCounter := 0
	backoff := time.Duration(0)
	for {
		// Exit if the VU has run all its intended iterations.
		if maxIterations > 0 && vu.Iterations >= maxIterations {
			return
		}

		// If the engine is paused, sleep until it resumes.
		vuPause := e.vuPause
		if vuPause != nil {
			<-vuPause
		}

		select {
		case <-ctx.Done():
			return
		default:
		}

		succ := e.runVUOnce(ctx, vu)
		if !succ {
			backoffCounter++
			backoff += BackoffAmount * time.Duration(backoffCounter)
			if backoff > BackoffMax {
				backoff = BackoffMax
			}
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
			}
		} else {
			backoff = 0
		}
	}
}

func (e *Engine) runVUOnce(ctx context.Context, vu *vuEntry) bool {
	samples, err := vu.VU.RunOnce(ctx)

	// Expired VUs usually have request cancellation errors, and thus skewed metrics and
	// unhelpful "request cancelled" errors. Don't process those.
	select {
	case <-ctx.Done():
		return true
	default:
	}

	if err != nil {
		if serr, ok := err.(fmt.Stringer); ok {
			e.Logger.Error(serr.String())
		} else {
			e.Logger.WithError(err).Error("VU Error")
		}
		samples = append(samples,
			stats.Sample{
				Time:   time.Now(),
				Metric: metrics.Errors,
				Tags:   map[string]string{"error": err.Error()},
				Value:  1,
			},
		)
	}

	vu.lock.Lock()
	vu.Samples = append(vu.Samples, samples...)
	vu.lock.Unlock()

	atomic.AddInt64(&vu.Iterations, 1)
	atomic.AddInt64(&e.numIterations, 1)
	if err != nil {
		atomic.AddInt64(&e.numErrors, 1)
		return false
	}
	return true
}

func (e *Engine) runMetricsEmission(ctx context.Context) {
	ticker := time.NewTicker(MetricsRate)
	for {
		select {
		case <-ticker.C:
			e.emitMetrics()
		case <-ctx.Done():
			return
		}
	}
}

func (e *Engine) emitMetrics() {
	e.lock.Lock()
	defer e.lock.Unlock()

	t := time.Now()
	e.processSamples(
		stats.Sample{
			Time:   t,
			Metric: metrics.VUs,
			Value:  float64(e.vus),
		},
		stats.Sample{
			Time:   t,
			Metric: metrics.VUsMax,
			Value:  float64(e.vusMax),
		},
		stats.Sample{
			Time:   t,
			Metric: metrics.Iterations,
			Value:  float64(atomic.LoadInt64(&e.numIterations)),
		},
	)
}

func (e *Engine) runThresholds(ctx context.Context) {
	ticker := time.NewTicker(ThresholdsRate)
	for {
		select {
		case <-ticker.C:
			e.processThresholds()
		case <-ctx.Done():
			return
		}
	}
}

func (e *Engine) processThresholds() {
	e.MetricsLock.Lock()
	defer e.MetricsLock.Unlock()

	e.thresholdsTainted = false
	for m, s := range e.Metrics {
		ts, ok := e.Thresholds[m.Name]
		if !ok {
			continue
		}

		m.Tainted = null.BoolFrom(false)

		e.Logger.WithField("m", m.Name).Debug("running thresholds")
		succ, err := ts.Run(s)
		if err != nil {
			e.Logger.WithField("m", m.Name).WithError(err).Error("Threshold error")
			continue
		}
		if !succ {
			e.Logger.WithField("m", m.Name).Debug("Thresholds failed")
			m.Tainted = null.BoolFrom(true)
			e.thresholdsTainted = true
		}
	}
}

func (e *Engine) runCollection(ctx context.Context) {
	ticker := time.NewTicker(CollectRate)
	for {
		select {
		case <-ticker.C:
			e.processSamples(e.collect()...)
		case <-ctx.Done():
			return
		}
	}
}

func (e *Engine) collect() []stats.Sample {
	e.lock.Lock()
	entries := e.vuEntries
	e.lock.Unlock()

	samples := []stats.Sample{}
	for _, vu := range entries {
		vu.lock.Lock()
		if len(vu.Samples) > 0 {
			samples = append(samples, vu.Samples...)
			vu.Samples = nil
		}
		vu.lock.Unlock()
	}
	return samples
}

func (e *Engine) processSamples(samples ...stats.Sample) {
	e.MetricsLock.Lock()
	defer e.MetricsLock.Unlock()

	for _, sample := range samples {
		sink := e.Metrics[sample.Metric]
		if sink == nil {
			sink = sample.Metric.NewSink()
			e.Metrics[sample.Metric] = sink
		}
		sink.Add(sample)

		for _, sm := range e.submetrics[sample.Metric.Name] {
			passing := true
			for k, v := range sm.Conditions {
				if sample.Tags[k] != v {
					passing = false
					break
				}
			}
			if !passing {
				continue
			}

			if sm.Metric == nil {
				sm.Metric = stats.New(sm.Name, sample.Metric.Type, sample.Metric.Contains)
				e.Metrics[sm.Metric] = sm.Metric.NewSink()
			}
			e.Metrics[sm.Metric].Add(sample)
		}
	}

	if e.Collector != nil {
		e.Collector.Collect(samples)
	}
}
