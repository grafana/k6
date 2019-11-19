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
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/stats"
	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"
)

const (
	TickRate        = 1 * time.Millisecond
	MetricsRate     = 1 * time.Second
	CollectRate     = 50 * time.Millisecond
	ThresholdsRate  = 2 * time.Second
	ShutdownTimeout = 10 * time.Second

	BackoffAmount = 50 * time.Millisecond
	BackoffMax    = 10 * time.Second
)

// The Engine is the beating heart of K6.
type Engine struct {
	runLock sync.Mutex // y tho? TODO: remove?

	//TODO: make most of the stuff here private!
	ExecutionScheduler lib.ExecutionScheduler
	executionState     *lib.ExecutionState

	Options       lib.Options
	Collectors    []lib.Collector
	NoThresholds  bool
	NoSummary     bool
	SummaryExport bool

	logger *logrus.Logger

	Metrics     map[string]*stats.Metric
	MetricsLock sync.Mutex

	Samples chan stats.SampleContainer

	// Assigned to metrics upon first received sample.
	thresholds map[string]stats.Thresholds
	submetrics map[string][]*stats.Submetric

	// Are thresholds tainted?
	thresholdsTainted bool
}

// NewEngine instantiates a new Engine, without doing any heavy initialization.
func NewEngine(ex lib.ExecutionScheduler, o lib.Options, logger *logrus.Logger) (*Engine, error) {
	if ex == nil {
		return nil, errors.New("missing ExecutionScheduler instance")
	}

	e := &Engine{
		ExecutionScheduler: ex,
		executionState:     ex.GetState(),

		Options: o,
		Metrics: make(map[string]*stats.Metric),
		Samples: make(chan stats.SampleContainer, o.MetricSamplesBufferSize.Int64),
		logger:  logger,
	}

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

// Init is used to initialize the execution scheduler. That's a costly operation, since it
// initializes all of the planned VUs and could potentially take a long time.
func (e *Engine) Init(ctx context.Context) error {
	return e.ExecutionScheduler.Init(ctx, e.Samples)
}

func (e *Engine) setRunStatus(status lib.RunStatus) {
	for _, c := range e.Collectors {
		c.SetRunStatus(status)
	}
}

func (e *Engine) Run(ctx context.Context) error {
	e.runLock.Lock()
	defer e.runLock.Unlock()

	e.logger.Debug("Engine: Starting with parameters...")

	collectorwg := sync.WaitGroup{}
	collectorctx, collectorcancel := context.WithCancel(context.Background())

	for _, collector := range e.Collectors {
		collectorwg.Add(1)
		go func(collector lib.Collector) {
			collector.Run(collectorctx)
			collectorwg.Done()
		}(collector)
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

	// Run the execution scheduler.
	errC := make(chan error)
	subwg.Add(1)
	go func() {
		errC <- e.ExecutionScheduler.Run(subctx, e.Samples)
		e.logger.Debug("Engine: Execution scheduler terminated")
		subwg.Done()
	}()

	sampleContainers := []stats.SampleContainer{}
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
			close(e.Samples)
		}()

		for sc := range e.Samples {
			sampleContainers = append(sampleContainers, sc)
		}

		e.processSamples(sampleContainers)

		// Process final thresholds.
		if !e.NoThresholds {
			e.processThresholds(nil)
		}

		// Finally, shut down collector.
		collectorcancel()
		collectorwg.Wait()
	}()

	ticker := time.NewTicker(CollectRate)
	for {
		select {
		case <-ticker.C:
			if len(sampleContainers) > 0 {
				e.processSamples(sampleContainers)
				sampleContainers = []stats.SampleContainer{}
			}
		case sc := <-e.Samples:
			sampleContainers = append(sampleContainers, sc)
		case err := <-errC:
			errC = nil
			if err != nil {
				e.logger.WithError(err).Debug("run: execution scheduler returned an error")
				e.setRunStatus(lib.RunStatusAbortedSystem)
				return err
			}
			e.logger.Debug("run: execution scheduler terminated")
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

	executionState := e.ExecutionScheduler.GetState()
	e.processSamples([]stats.SampleContainer{stats.ConnectedSamples{
		Samples: []stats.Sample{
			{
				Time:   t,
				Metric: metrics.VUs,
				Value:  float64(executionState.GetCurrentlyActiveVUsCount()),
				Tags:   e.Options.RunTags,
			}, {
				Time:   t,
				Metric: metrics.VUsMax,
				Value:  float64(executionState.GetInitializedVUsCount()),
				Tags:   e.Options.RunTags,
			},
		},
		Tags: e.Options.RunTags,
		Time: t,
	}})
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

	t := e.executionState.GetCurrentTestRunDuration()
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

func (e *Engine) processSamplesForMetrics(sampleCointainers []stats.SampleContainer) {
	for _, sampleCointainer := range sampleCointainers {
		samples := sampleCointainer.GetSamples()

		if len(samples) == 0 {
			continue
		}

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
				if !sample.Tags.Contains(sm.Tags) {
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
	}
}

func (e *Engine) processSamples(sampleContainers []stats.SampleContainer) {
	if len(sampleContainers) == 0 {
		return
	}

	// TODO: optimize this...
	e.MetricsLock.Lock()
	defer e.MetricsLock.Unlock()

	// TODO: run this and the below code in goroutines?
	if !(e.NoSummary && e.NoThresholds && !e.SummaryExport) {
		e.processSamplesForMetrics(sampleContainers)
	}

	for _, collector := range e.Collectors {
		collector.Collect(sampleContainers)
	}
}
