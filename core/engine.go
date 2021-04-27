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

	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/output"
	"github.com/loadimpact/k6/stats"
)

const (
	metricsRate    = 1 * time.Second
	collectRate    = 50 * time.Millisecond
	thresholdsRate = 2 * time.Second
)

// The Engine is the beating heart of k6.
type Engine struct {
	// TODO: Make most of the stuff here private! And think how to refactor the
	// engine to be less stateful... it's currently one big mess of moving
	// pieces, and you implicitly first have to call Init() and then Run() -
	// maybe we should refactor it so we have a `Session` dauther-object that
	// Init() returns? The only problem with doing this is the REST API - it
	// expects to be able to get information from the Engine and is initialized
	// before the Init() call...

	ExecutionScheduler lib.ExecutionScheduler
	executionState     *lib.ExecutionState

	Options        lib.Options
	runtimeOptions lib.RuntimeOptions
	outputs        []output.Output

	logger   *logrus.Entry
	stopOnce sync.Once
	stopChan chan struct{}

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
func NewEngine(
	ex lib.ExecutionScheduler, opts lib.Options, rtOpts lib.RuntimeOptions, outputs []output.Output, logger *logrus.Logger,
) (*Engine, error) {
	if ex == nil {
		return nil, errors.New("missing ExecutionScheduler instance")
	}

	e := &Engine{
		ExecutionScheduler: ex,
		executionState:     ex.GetState(),

		Options:        opts,
		runtimeOptions: rtOpts,
		outputs:        outputs,
		Metrics:        make(map[string]*stats.Metric),
		Samples:        make(chan stats.SampleContainer, opts.MetricSamplesBufferSize.Int64),
		stopChan:       make(chan struct{}),
		logger:         logger.WithField("component", "engine"),
	}

	e.thresholds = opts.Thresholds
	e.submetrics = make(map[string][]*stats.Submetric)
	for name := range e.thresholds {
		if !strings.Contains(name, "{") {
			continue
		}

		parent, sm := stats.NewSubmetric(name)
		e.submetrics[parent] = append(e.submetrics[parent], sm)
	}

	// TODO: refactor this out of here when https://github.com/loadimpact/k6/issues/1832 lands and
	// there is a better way to enable a metric with tag
	if opts.SystemTags.Has(stats.TagExpectedResponse) {
		for _, name := range []string{
			"http_req_duration{expected_response:true}",
		} {
			if _, ok := e.thresholds[name]; ok {
				continue
			}
			parent, sm := stats.NewSubmetric(name)
			e.submetrics[parent] = append(e.submetrics[parent], sm)
		}
	}

	return e, nil
}

// StartOutputs spins up all configured outputs, giving the thresholds to any
// that can accept them. And if some output fails, stop the already started
// ones. This may take some time, since some outputs make initial network
// requests to set up whatever remote services are going to listen to them.
//
// TODO: this doesn't really need to be in the Engine, so take it out?
func (e *Engine) StartOutputs() error {
	e.logger.Debugf("Starting %d outputs...", len(e.outputs))
	for i, out := range e.outputs {
		if thresholdOut, ok := out.(output.WithThresholds); ok {
			thresholdOut.SetThresholds(e.thresholds)
		}

		if stopOut, ok := out.(output.WithTestRunStop); ok {
			stopOut.SetTestRunStopCallback(
				func(err error) {
					e.logger.WithError(err).Error("Received error to stop from output")
					e.Stop()
				})
		}

		if err := out.Start(); err != nil {
			e.stopOutputs(i)
			return err
		}
	}
	return nil
}

// StopOutputs stops all configured outputs.
func (e *Engine) StopOutputs() {
	e.stopOutputs(len(e.outputs))
}

func (e *Engine) stopOutputs(upToID int) {
	e.logger.Debugf("Stopping %d outputs...", upToID)
	for i := 0; i < upToID; i++ {
		if err := e.outputs[i].Stop(); err != nil {
			e.logger.WithError(err).Errorf("Stopping output %d failed", i)
		}
	}
}

// Init is used to initialize the execution scheduler and all metrics processing
// in the engine. The first is a costly operation, since it initializes all of
// the planned VUs and could potentially take a long time.
//
// This method either returns an error immediately, or it returns test run() and
// wait() functions.
//
// Things to note:
//  - The first lambda, Run(), synchronously executes the actual load test.
//  - It can be prematurely aborted by cancelling the runCtx - this won't stop
//    the metrics collection by the Engine.
//  - Stopping the metrics collection can be done at any time after Run() has
//    returned by cancelling the globalCtx
//  - The second returned lambda can be used to wait for that process to finish.
func (e *Engine) Init(globalCtx, runCtx context.Context) (run func() error, wait func(), err error) {
	e.logger.Debug("Initialization starting...")
	// TODO: if we ever need metrics processing in the init context, we can move
	// this below the other components... or even start them concurrently?
	if err := e.ExecutionScheduler.Init(runCtx, e.Samples); err != nil {
		return nil, nil, err
	}

	// TODO: move all of this in a separate struct? see main TODO above
	runSubCtx, runSubCancel := context.WithCancel(runCtx)

	resultCh := make(chan error)
	processMetricsAfterRun := make(chan struct{})
	runFn := func() error {
		e.logger.Debug("Execution scheduler starting...")
		err := e.ExecutionScheduler.Run(globalCtx, runSubCtx, e.Samples)
		e.logger.WithError(err).Debug("Execution scheduler terminated")

		select {
		case <-runSubCtx.Done():
			// do nothing, the test run was aborted somehow
		default:
			resultCh <- err // we finished normally, so send the result
		}

		// Make the background jobs process the currently buffered metrics and
		// run the thresholds, then wait for that to be done.
		processMetricsAfterRun <- struct{}{}
		<-processMetricsAfterRun

		return err
	}

	waitFn := e.startBackgroundProcesses(globalCtx, runCtx, resultCh, runSubCancel, processMetricsAfterRun)
	return runFn, waitFn, nil
}

// This starts a bunch of goroutines to process metrics, thresholds, and set the
// test run status when it ends. It returns a function that can be used after
// the provided context is called, to wait for the complete winding down of all
// started goroutines.
func (e *Engine) startBackgroundProcesses(
	globalCtx, runCtx context.Context, runResult <-chan error, runSubCancel func(), processMetricsAfterRun chan struct{},
) (wait func()) {
	processes := new(sync.WaitGroup)

	// Siphon and handle all produced metric samples
	processes.Add(1)
	go func() {
		defer processes.Done()
		e.processMetrics(globalCtx, processMetricsAfterRun)
	}()

	// Run VU metrics emission, only while the test is running.
	// TODO: move? this seems like something the ExecutionScheduler should emit...
	processes.Add(1)
	go func() {
		defer processes.Done()
		e.logger.Debug("Starting emission of VU metrics...")
		e.runMetricsEmission(runCtx)
		e.logger.Debug("Metrics emission terminated")
	}()

	// Update the test run status when the test finishes
	processes.Add(1)
	thresholdAbortChan := make(chan struct{})
	go func() {
		defer processes.Done()
		select {
		case err := <-runResult:
			if err != nil {
				e.logger.WithError(err).Debug("run: execution scheduler returned an error")
				var serr types.ScriptException
				if errors.As(err, &serr) {
					e.setRunStatus(lib.RunStatusAbortedScriptError)
				} else {
					e.setRunStatus(lib.RunStatusAbortedSystem)
				}
			} else {
				e.logger.Debug("run: execution scheduler terminated")
				e.setRunStatus(lib.RunStatusFinished)
			}
		case <-runCtx.Done():
			e.logger.Debug("run: context expired; exiting...")
			e.setRunStatus(lib.RunStatusAbortedUser)
		case <-e.stopChan:
			runSubCancel()
			e.logger.Debug("run: stopped by user; exiting...")
			e.setRunStatus(lib.RunStatusAbortedUser)
		case <-thresholdAbortChan:
			e.logger.Debug("run: stopped by thresholds; exiting...")
			runSubCancel()
			e.setRunStatus(lib.RunStatusAbortedThreshold)
		}
	}()

	// Run thresholds, if not disabled.
	if !e.runtimeOptions.NoThresholds.Bool {
		processes.Add(1)
		go func() {
			defer processes.Done()
			defer e.logger.Debug("Engine: Thresholds terminated")
			ticker := time.NewTicker(thresholdsRate)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					if e.processThresholds() {
						close(thresholdAbortChan)
						return
					}
				case <-runCtx.Done():
					return
				}
			}
		}()
	}

	return processes.Wait
}

func (e *Engine) processMetrics(globalCtx context.Context, processMetricsAfterRun chan struct{}) {
	sampleContainers := []stats.SampleContainer{}

	defer func() {
		// Process any remaining metrics in the pipeline, by this point Run()
		// has already finished and nothing else should be producing metrics.
		e.logger.Debug("Metrics processing winding down...")

		close(e.Samples)
		for sc := range e.Samples {
			sampleContainers = append(sampleContainers, sc)
		}
		e.processSamples(sampleContainers)

		if !e.runtimeOptions.NoThresholds.Bool {
			e.processThresholds() // Process the thresholds one final time
		}
	}()

	ticker := time.NewTicker(collectRate)
	defer ticker.Stop()

	e.logger.Debug("Metrics processing started...")
	processSamples := func() {
		if len(sampleContainers) > 0 {
			e.processSamples(sampleContainers)
			// Make the new container with the same size as the previous
			// one, assuming that we produce roughly the same amount of
			// metrics data between ticks...
			sampleContainers = make([]stats.SampleContainer, 0, cap(sampleContainers))
		}
	}
	for {
		select {
		case <-ticker.C:
			processSamples()
		case <-processMetricsAfterRun:
		getCachedMetrics:
			for {
				select {
				case sc := <-e.Samples:
					sampleContainers = append(sampleContainers, sc)
				default:
					break getCachedMetrics
				}
			}
			e.logger.Debug("Processing metrics and thresholds after the test run has ended...")
			processSamples()
			if !e.runtimeOptions.NoThresholds.Bool {
				e.processThresholds()
			}
			processMetricsAfterRun <- struct{}{}

		case sc := <-e.Samples:
			sampleContainers = append(sampleContainers, sc)
		case <-globalCtx.Done():
			return
		}
	}
}

func (e *Engine) setRunStatus(status lib.RunStatus) {
	for _, out := range e.outputs {
		if statUpdOut, ok := out.(output.WithRunStatusUpdates); ok {
			statUpdOut.SetRunStatus(status)
		}
	}
}

func (e *Engine) IsTainted() bool {
	return e.thresholdsTainted
}

// Stop closes a signal channel, forcing a running Engine to return
func (e *Engine) Stop() {
	e.stopOnce.Do(func() {
		close(e.stopChan)
	})
}

// IsStopped returns a bool indicating whether the Engine has been stopped
func (e *Engine) IsStopped() bool {
	select {
	case <-e.stopChan:
		return true
	default:
		return false
	}
}

func (e *Engine) runMetricsEmission(ctx context.Context) {
	ticker := time.NewTicker(metricsRate)
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
	// TODO: optimize and move this, it shouldn't call processSamples() directly
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

func (e *Engine) processThresholds() (shouldAbort bool) {
	e.MetricsLock.Lock()
	defer e.MetricsLock.Unlock()

	t := e.executionState.GetCurrentTestRunDuration()

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
			if m.Thresholds.Abort {
				shouldAbort = true
			}
		}
	}

	return shouldAbort
}

func (e *Engine) processSamplesForMetrics(sampleContainers []stats.SampleContainer) {
	for _, sampleContainer := range sampleContainers {
		samples := sampleContainer.GetSamples()

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
	if !(e.runtimeOptions.NoSummary.Bool && e.runtimeOptions.NoThresholds.Bool) {
		e.processSamplesForMetrics(sampleContainers)
	}

	for _, out := range e.outputs {
		out.AddMetricSamples(sampleContainers)
	}
}
