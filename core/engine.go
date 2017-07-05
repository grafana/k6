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

package core

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/loadimpact/k6/core/local"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/stats"
	log "github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"
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

// The Engine is the beating heart of K6.
type Engine struct {
	runLock sync.Mutex

	Executor  lib.Executor
	Options   lib.Options
	Collector lib.Collector

	logger *log.Logger

	Stages      []lib.Stage
	Metrics     map[string]*stats.Metric
	MetricsLock sync.RWMutex

	// Assigned to metrics upon first received sample.
	thresholds map[string]stats.Thresholds
	submetrics map[string][]stats.Submetric

	// Are thresholds tainted?
	thresholdsTainted bool
}

func NewEngine(ex lib.Executor, o lib.Options) (*Engine, error) {
	if ex == nil {
		ex = local.New(nil)
	}

	e := &Engine{
		Executor: ex,
		Options:  o,
		Metrics:  make(map[string]*stats.Metric),
	}
	e.SetLogger(log.StandardLogger())

	if err := ex.SetVUsMax(o.VUsMax.Int64); err != nil {
		return nil, err
	}
	if err := ex.SetVUs(o.VUs.Int64); err != nil {
		return nil, err
	}
	ex.SetPaused(o.Paused.Bool)

	// Use Stages if available, if not, construct a stage to fill the specified duration.
	// Special case: A valid duration of 0 = an infinite (invalid duration) stage.
	if o.Stages != nil {
		e.Stages = o.Stages
	} else if o.Duration.Valid && o.Duration.Duration > 0 {
		e.Stages = []lib.Stage{{Duration: o.Duration}}
	} else {
		e.Stages = []lib.Stage{{}}
	}

	ex.SetEndTime(SumStages(e.Stages))
	ex.SetEndIterations(o.Iterations)

	e.thresholds = o.Thresholds
	e.submetrics = make(map[string][]stats.Submetric)
	for name := range e.thresholds {
		if !strings.Contains(name, "{") {
			continue
		}

		parent, sm := stats.NewSubmetric(name)
		e.submetrics[parent] = append(e.submetrics[parent], sm)
	}

	return e, nil
}

func (e *Engine) Run(ctx context.Context) error {
	e.runLock.Lock()
	defer e.runLock.Unlock()

	e.logger.Debug("Engine: Starting with parameters...")
	for i, st := range e.Stages {
		fields := make(log.Fields)
		if st.Target.Valid {
			fields["tgt"] = st.Target.Int64
		}
		if st.Duration.Valid {
			fields["d"] = st.Duration.Duration
		}
		e.logger.WithFields(fields).Debugf(" - stage #%d", i)
	}

	fields := make(log.Fields)
	if endTime := e.Executor.GetEndTime(); endTime.Valid {
		fields["time"] = endTime.Duration
	}
	if endIter := e.Executor.GetEndIterations(); endIter.Valid {
		fields["iter"] = endIter.Int64
	}
	e.logger.WithFields(fields).Debug(" - end conditions (if any)")

	collectorwg := sync.WaitGroup{}
	collectorctx, collectorcancel := context.WithCancel(context.Background())
	if e.Collector != nil {
		collectorwg.Add(1)
		go func() {
			e.Collector.Run(collectorctx)
			collectorwg.Done()
		}()
		for !e.Collector.IsReady() {
			runtime.Gosched()
		}
	}

	subctx, subcancel := context.WithCancel(context.Background())
	subwg := sync.WaitGroup{}

	// Run metrics emission.
	subwg.Add(1)
	go func() {
		e.runMetricsEmission(subctx)
		e.logger.Debug("Engine: Emission terminated")
		subwg.Done()
	}()

	// Run thresholds.
	subwg.Add(1)
	go func() {
		e.runThresholds(subctx)
		e.logger.Debug("Engine: Thresholds terminated")
		subwg.Done()
	}()

	// Run the executor.
	out := make(chan []stats.Sample)
	errC := make(chan error)
	subwg.Add(1)
	go func() {
		errC <- e.Executor.Run(subctx, out)
		e.logger.Debug("Engine: Executor terminated")
		subwg.Done()
	}()

	defer func() {
		// Shut down subsystems.
		subcancel()

		// Process samples until the subsystems have shut down.
		// Filter out samples produced past the end of a test.
		go func() {
			if errC != nil {
				<-errC
				errC = nil
			}
			subwg.Wait()
			close(out)
		}()
		for samples := range out {
			e.processSamples(samples...)
		}

		// Emit final metrics.
		e.emitMetrics()

		// Process final thresholds.
		e.processThresholds()

		// Finally, shut down collector.
		collectorcancel()
		collectorwg.Wait()
	}()

	ticker := time.NewTicker(TickRate)
	for {
		select {
		case <-ticker.C:
			vus, keepRunning := ProcessStages(e.Stages, e.Executor.GetTime())
			if !keepRunning {
				e.logger.Debug("run: ProcessStages() returned false; exiting...")
				return nil
			}
			if err := e.Executor.SetVUs(vus); err != nil {
				return err
			}
		case samples := <-out:
			e.processSamples(samples...)
		case err := <-errC:
			errC = nil
			if err != nil {
				e.logger.WithError(err).Debug("run: executor returned an error")
				return err
			}
			e.logger.Debug("run: executor terminated")
			return nil
		case <-ctx.Done():
			e.logger.Debug("run: context expired; exiting...")
			return nil
		}
	}
}

func (e *Engine) IsTainted() bool {
	return e.thresholdsTainted
}

func (e *Engine) SetLogger(l *log.Logger) {
	e.logger = l
	e.Executor.SetLogger(l)
}

func (e *Engine) GetLogger() *log.Logger {
	return e.logger
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
	t := time.Now()
	e.processSamples(
		stats.Sample{
			Time:   t,
			Metric: metrics.VUs,
			Value:  float64(e.Executor.GetVUs()),
		},
		stats.Sample{
			Time:   t,
			Metric: metrics.VUsMax,
			Value:  float64(e.Executor.GetVUsMax()),
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
	for _, m := range e.Metrics {
		if len(m.Thresholds.Thresholds) == 0 {
			continue
		}
		m.Tainted = null.BoolFrom(false)

		e.logger.WithField("m", m.Name).Debug("running thresholds")
		succ, err := m.Thresholds.Run(m.Sink)
		if err != nil {
			e.logger.WithField("m", m.Name).WithError(err).Error("Threshold error")
			continue
		}
		if !succ {
			e.logger.WithField("m", m.Name).Debug("Thresholds failed")
			m.Tainted = null.BoolFrom(true)
			e.thresholdsTainted = true
		}
	}
}

func (e *Engine) processSamples(samples ...stats.Sample) {
	if len(samples) == 0 {
		return
	}

	e.MetricsLock.Lock()
	defer e.MetricsLock.Unlock()

	for _, sample := range samples {
		m, ok := e.Metrics[sample.Metric.Name]
		if !ok {
			m = sample.Metric
			m.Thresholds = e.thresholds[m.Name]
			m.Submetrics = e.submetrics[m.Name]
			e.Metrics[m.Name] = m
		}
		m.Sink.Add(sample)

		for _, sm := range m.Submetrics {
			passing := true
			for k, v := range sm.Tags {
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
				sm.Metric.Thresholds = e.thresholds[sm.Name]
				e.Metrics[sm.Name] = sm.Metric
			}
			sm.Metric.Sink.Add(sample)
		}
	}

	if e.Collector != nil {
		e.Collector.Collect(samples)
	}
}
