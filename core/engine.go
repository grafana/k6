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
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/metrics/engine"
	"go.k6.io/k6/output"
	"go.k6.io/k6/stats"
)

const (
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

	// TODO: completely remove the engine and use all of these separately, in a
	// much more composable and testable manner
	ExecutionScheduler lib.ExecutionScheduler
	MetricsEngine      *engine.MetricsEngine
	OutputManager      *output.Manager

	runtimeOptions lib.RuntimeOptions

	ingester output.Output

	logger   *logrus.Entry
	stopOnce sync.Once
	stopChan chan struct{}

	Samples chan stats.SampleContainer

	// Are thresholds tainted?
	thresholdsTaintedLock sync.Mutex
	thresholdsTainted     bool
}

// NewEngine instantiates a new Engine, without doing any heavy initialization.
func NewEngine(
	ex lib.ExecutionScheduler, opts lib.Options, rtOpts lib.RuntimeOptions, outputs []output.Output, logger *logrus.Logger,
	registry *metrics.Registry,
) (*Engine, error) {
	if ex == nil {
		return nil, errors.New("missing ExecutionScheduler instance")
	}

	e := &Engine{
		ExecutionScheduler: ex,

		runtimeOptions: rtOpts,
		Samples:        make(chan stats.SampleContainer, opts.MetricSamplesBufferSize.Int64),
		stopChan:       make(chan struct{}),
		logger:         logger.WithField("component", "engine"),
	}

	me, err := engine.NewMetricsEngine(registry, ex.GetState(), opts, rtOpts, logger)
	if err != nil {
		return nil, err
	}
	e.MetricsEngine = me

	if !(rtOpts.NoSummary.Bool && rtOpts.NoThresholds.Bool) {
		e.ingester = me.GetIngester()
		outputs = append(outputs, e.ingester)
	}

	e.OutputManager = output.NewManager(outputs, logger, func(err error) {
		if err != nil {
			logger.WithError(err).Error("Received error to stop from output")
		}
		e.Stop()
	})

	return e, nil
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

	// Update the test run status when the test finishes
	processes.Add(1)
	thresholdAbortChan := make(chan struct{})
	go func() {
		defer processes.Done()
		select {
		case err := <-runResult:
			if err != nil {
				e.logger.WithError(err).Debug("run: execution scheduler returned an error")
				var serr errext.Exception
				switch {
				case errors.As(err, &serr):
					e.OutputManager.SetRunStatus(lib.RunStatusAbortedScriptError)
				case common.IsInterruptError(err):
					e.OutputManager.SetRunStatus(lib.RunStatusAbortedUser)
				default:
					e.OutputManager.SetRunStatus(lib.RunStatusAbortedSystem)
				}
			} else {
				e.logger.Debug("run: execution scheduler terminated")
				e.OutputManager.SetRunStatus(lib.RunStatusFinished)
			}
		case <-runCtx.Done():
			e.logger.Debug("run: context expired; exiting...")
			e.OutputManager.SetRunStatus(lib.RunStatusAbortedUser)
		case <-e.stopChan:
			runSubCancel()
			e.logger.Debug("run: stopped by user; exiting...")
			e.OutputManager.SetRunStatus(lib.RunStatusAbortedUser)
		case <-thresholdAbortChan:
			e.logger.Debug("run: stopped by thresholds; exiting...")
			runSubCancel()
			e.OutputManager.SetRunStatus(lib.RunStatusAbortedThreshold)
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
					thresholdsTainted, shouldAbort := e.MetricsEngine.EvaluateThresholds()
					e.thresholdsTaintedLock.Lock()
					e.thresholdsTainted = thresholdsTainted
					e.thresholdsTaintedLock.Unlock()
					if shouldAbort {
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
		e.OutputManager.AddMetricSamples(sampleContainers)

		if !e.runtimeOptions.NoThresholds.Bool {
			// Process the thresholds one final time
			thresholdsTainted, _ := e.MetricsEngine.EvaluateThresholds()
			e.thresholdsTaintedLock.Lock()
			e.thresholdsTainted = thresholdsTainted
			e.thresholdsTaintedLock.Unlock()
		}
	}()

	ticker := time.NewTicker(collectRate)
	defer ticker.Stop()

	e.logger.Debug("Metrics processing started...")
	processSamples := func() {
		if len(sampleContainers) > 0 {
			e.OutputManager.AddMetricSamples(sampleContainers)
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
				// Ensure the ingester flushes any buffered metrics
				_ = e.ingester.Stop()
				thresholdsTainted, _ := e.MetricsEngine.EvaluateThresholds()
				e.thresholdsTaintedLock.Lock()
				e.thresholdsTainted = thresholdsTainted
				e.thresholdsTaintedLock.Unlock()
			}
			processMetricsAfterRun <- struct{}{}

		case sc := <-e.Samples:
			sampleContainers = append(sampleContainers, sc)
		case <-globalCtx.Done():
			return
		}
	}
}

func (e *Engine) IsTainted() bool {
	e.thresholdsTaintedLock.Lock()
	defer e.thresholdsTaintedLock.Unlock()
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
