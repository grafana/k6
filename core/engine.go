package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"go.k6.io/k6/execution"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/metrics/engine"
	"go.k6.io/k6/output"
)

const collectRate = 50 * time.Millisecond

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
	ExecutionScheduler *execution.Scheduler
	MetricsEngine      *engine.MetricsEngine
	OutputManager      *output.Manager

	runtimeOptions lib.RuntimeOptions

	ingester output.Output

	logger  *logrus.Entry
	AbortFn func(error) // temporary

	Samples chan metrics.SampleContainer
}

// NewEngine instantiates a new Engine, without doing any heavy initialization.
func NewEngine(testState *lib.TestRunState, ex *execution.Scheduler, outputs []output.Output) (*Engine, error) {
	if ex == nil {
		return nil, errors.New("missing ExecutionScheduler instance")
	}

	e := &Engine{
		ExecutionScheduler: ex,

		runtimeOptions: testState.RuntimeOptions,
		Samples:        make(chan metrics.SampleContainer, testState.Options.MetricSamplesBufferSize.Int64),
		logger:         testState.Logger.WithField("component", "engine"),
	}

	me, err := engine.NewMetricsEngine(ex.GetState())
	if err != nil {
		return nil, err
	}
	e.MetricsEngine = me

	if !(testState.RuntimeOptions.NoSummary.Bool && testState.RuntimeOptions.NoThresholds.Bool) {
		e.ingester = me.CreateIngester()
		outputs = append(outputs, e.ingester)
	}

	e.OutputManager = output.NewManager(outputs, testState.Logger, func(err error) {
		if err != nil {
			testState.Logger.WithError(err).Error("Received error to stop from output")
		}
		e.AbortFn(err)
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
//   - The first lambda, Run(), synchronously executes the actual load test.
//   - It can be prematurely aborted by cancelling the runCtx - this won't stop
//     the metrics collection by the Engine.
//   - Stopping the metrics collection can be done at any time after Run() has
//     returned by cancelling the globalCtx
//   - The second returned lambda can be used to wait for that process to finish.
func (e *Engine) Init(globalCtx, runCtx context.Context) (run func() error, wait func(), err error) {
	e.logger.Debug("Initialization starting...")

	// TODO: move all of this in a separate struct? see main TODO above
	processMetricsAfterRun := make(chan struct{})
	runFn := func() error {
		e.logger.Debug("Execution scheduler starting...")
		err := e.ExecutionScheduler.Run(globalCtx, runCtx, e.Samples)
		if err == nil {
			e.logger.Debug("Execution scheduler finished normally")
			err = runCtx.Err()
		}
		if err != nil {
			e.logger.WithError(err).Debug("Engine run returned an error")
		} else {
			e.logger.Debug("Execution scheduler and engine finished normally")
		}

		// Make the background jobs process the currently buffered metrics and
		// run the thresholds, then wait for that to be done.
		select {
		case processMetricsAfterRun <- struct{}{}:
			<-processMetricsAfterRun
		case <-globalCtx.Done():
		}

		return err
	}

	waitFn := e.startBackgroundProcesses(globalCtx, processMetricsAfterRun)
	return runFn, waitFn, nil
}

// This starts a bunch of goroutines to process metrics, thresholds, and set the
// test run status when it ends. It returns a function that can be used after
// the provided context is called, to wait for the complete winding down of all
// started goroutines.
//
// Because the background process is not aware of the execution's state, `processMetricsAfterRun`
// will be used to signal that the test run is finished, no more metric samples will be produced,
// and that the remaining metrics samples in the pipeline should be processed as the background
// process is about to exit.
func (e *Engine) startBackgroundProcesses(
	globalCtx context.Context, processMetricsAfterRun chan struct{},
) (wait func()) {
	processes := new(sync.WaitGroup)

	// Siphon and handle all produced metric samples
	processes.Add(1)
	go func() {
		defer processes.Done()
		e.processMetrics(globalCtx, processMetricsAfterRun)
	}()

	return processes.Wait
}

// processMetrics process the execution's metrics samples as they are collected.
// The processing of samples happens at a fixed rate defined by the `collectRate`
// constant.
//
// The `processMetricsAfterRun` channel argument is used by the caller to signal
// that the test run is finished, no more metric samples will be produced, and that
// the metrics samples remaining in the pipeline should be should be processed.
func (e *Engine) processMetrics(globalCtx context.Context, processMetricsAfterRun chan struct{}) {
	sampleContainers := []metrics.SampleContainer{}

	// Run thresholds, if not disabled.
	var finalizeThresholds func() (breached []string)
	if !e.runtimeOptions.NoThresholds.Bool {
		finalizeThresholds = e.MetricsEngine.StartThresholdCalculations(e.AbortFn)
	}

	ticker := time.NewTicker(collectRate)
	defer ticker.Stop()

	e.logger.Debug("Metrics processing started...")
	processSamples := func() {
		if len(sampleContainers) > 0 {
			e.OutputManager.AddMetricSamples(sampleContainers)
			// Make the new container with the same size as the previous
			// one, assuming that we produce roughly the same amount of
			// metrics data between ticks...
			sampleContainers = make([]metrics.SampleContainer, 0, cap(sampleContainers))
		}
	}

	finalize := func() {
		// Process any remaining metrics in the pipeline, by this point Run()
		// has already finished and nothing else should be producing metrics.
		e.logger.Debug("Metrics processing winding down...")

		close(e.Samples)
		for sc := range e.Samples {
			sampleContainers = append(sampleContainers, sc)
		}
		processSamples()

		if finalizeThresholds != nil {
			// Ensure the ingester flushes any buffered metrics
			_ = e.ingester.Stop()
			breached := finalizeThresholds()
			e.logger.Debugf("Engine: thresholds done, breached: '%s'", strings.Join(breached, ", "))
		}
		e.logger.Debug("Metrics processing finished!")
	}

	for {
		select {
		case <-ticker.C:
			processSamples()
		case <-processMetricsAfterRun:
			e.logger.Debug("Processing metrics and thresholds after the test run has ended...")
			finalize()
			processMetricsAfterRun <- struct{}{}
			return
		case sc := <-e.Samples:
			sampleContainers = append(sampleContainers, sc)
		case <-globalCtx.Done():
			finalize()
			return
		}
	}
}

func (e *Engine) IsTainted() bool {
	return e.MetricsEngine.GetMetricsWithBreachedThresholdsCount() > 0
}
