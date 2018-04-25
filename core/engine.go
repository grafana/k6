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

	Executor     lib.Executor
	Options      lib.Options
	Collector    lib.Collector
	NoThresholds bool

	logger *log.Logger

	Metrics     map[string]*stats.Metric
	MetricsLock sync.Mutex

	// Assigned to metrics upon first received sample.
	thresholds map[string]stats.Thresholds
	submetrics map[string][]*stats.Submetric

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
	ex.SetStages(o.Stages)
	ex.SetEndTime(o.Duration)
	ex.SetEndIterations(o.Iterations)

	e.thresholds = o.Thresholds
	e.submetrics = make(map[string][]*stats.Submetric)
	for name := range e.thresholds {
		if !strings.Contains(name, "{") {
			continue
		}

		parent, sm := stats.NewSubmetric(name)
		e.submetrics[parent] = append(e.submetrics[parent], sm)
	}

	return e, nil
}

func (e *Engine) setRunStatus(status int) {
	if e.Collector == nil {
		return
	}

	e.Collector.SetRunStatus(status)
}

func (e *Engine) Run(ctx context.Context) error {
	e.runLock.Lock()
	defer e.runLock.Unlock()

	e.logger.Debug("Engine: Starting with parameters...")
	for i, st := range e.Executor.GetStages() {
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
	if !e.NoThresholds {
		subwg.Add(1)
		go func() {
			e.runThresholds(subctx, subcancel)
			e.logger.Debug("Engine: Thresholds terminated")
			subwg.Done()
		}()
	}

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
		if !e.NoThresholds {
			e.processThresholds(nil)
		}

		// Finally, shut down collector.
		collectorcancel()
		collectorwg.Wait()
	}()

	for {
		select {
		case samples := <-out:
			e.processSamples(samples...)
		case err := <-errC:
			errC = nil
			if err != nil {
				e.logger.WithError(err).Debug("run: executor returned an error")
				e.setRunStatus(lib.RunStatusAbortedSystem)
				return err
			}
			e.logger.Debug("run: executor terminated")
			return nil
		case <-ctx.Done():
			e.logger.Debug("run: context expired; exiting...")
			e.setRunStatus(lib.RunStatusAbortedUser)
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
			Tags:   e.Options.RunTags,
		},
		stats.Sample{
			Time:   t,
			Metric: metrics.VUsMax,
			Value:  float64(e.Executor.GetVUsMax()),
			Tags:   e.Options.RunTags,
		},
	)
}

func (e *Engine) runThresholds(ctx context.Context, abort func()) {
	ticker := time.NewTicker(ThresholdsRate)
	for {
		select {
		case <-ticker.C:
			e.processThresholds(abort)
		case <-ctx.Done():
			return
		}
	}
}

func (e *Engine) processThresholds(abort func()) {
	e.MetricsLock.Lock()
	defer e.MetricsLock.Unlock()

	t := e.Executor.GetTime()
	abortOnFail := false

	e.thresholdsTainted = false
	for _, m := range e.Metrics {
		if len(m.Thresholds.Thresholds) == 0 {
			continue
		}
		m.Tainted = null.BoolFrom(false)

		e.logger.WithField("m", m.Name).Debug("running thresholds")
		succ, err := m.Thresholds.Run(m.Sink, t)
		if err != nil {
			e.logger.WithField("m", m.Name).WithError(err).Error("Threshold error")
			continue
		}
		if !succ {
			e.logger.WithField("m", m.Name).Debug("Thresholds failed")
			m.Tainted = null.BoolFrom(true)
			e.thresholdsTainted = true
			if !abortOnFail && m.Thresholds.Abort {
				abortOnFail = true
			}
		}
	}

	if abortOnFail && abort != nil {
		//TODO: When sending this status we get a 422 Unprocessable Entity
		e.setRunStatus(lib.RunStatusAbortedThreshold)
		abort()
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
			m = stats.New(sample.Metric.Name, sample.Metric.Type, sample.Metric.Contains)
			m.Thresholds = e.thresholds[m.Name]
			m.Submetrics = e.submetrics[m.Name]
			e.Metrics[m.Name] = m
		}
		m.Sink.Add(sample)

		for _, sm := range m.Submetrics {
			if !sm.Tags.IsEqual(sample.Tags) {
				continue
			}

			if sm.Metric == nil {
				sm.Metric = stats.New(sm.Name, sample.Metric.Type, sample.Metric.Contains)
				sm.Metric.Sub = *sm
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
