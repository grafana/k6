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
	"fmt"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/stats"
	"go.k6.io/k6/ui/pb"
)

// ExecutionScheduler is the local implementation of lib.ExecutionScheduler
type ExecutionScheduler struct {
	runner  lib.Runner
	options lib.Options
	logger  *logrus.Logger

	initProgress    *pb.ProgressBar
	executorConfigs []lib.ExecutorConfig // sorted by (startTime, ID)
	executors       []lib.Executor       // sorted by (startTime, ID), excludes executors with no work
	executionPlan   []lib.ExecutionStep
	maxDuration     time.Duration // cached value derived from the execution plan
	maxPossibleVUs  uint64        // cached value derived from the execution plan
	state           *lib.ExecutionState
}

// Check to see if we implement the lib.ExecutionScheduler interface
var _ lib.ExecutionScheduler = &ExecutionScheduler{}

// NewExecutionScheduler creates and returns a new local lib.ExecutionScheduler
// instance, without initializing it beyond the bare minimum. Specifically, it
// creates the needed executor instances and a lot of state placeholders, but it
// doesn't initialize the executors and it doesn't initialize or run VUs.
func NewExecutionScheduler(runner lib.Runner, logger *logrus.Logger) (*ExecutionScheduler, error) {
	options := runner.GetOptions()
	et, err := lib.NewExecutionTuple(options.ExecutionSegment, options.ExecutionSegmentSequence)
	if err != nil {
		return nil, err
	}
	executionPlan := options.Scenarios.GetFullExecutionRequirements(et)
	maxPlannedVUs := lib.GetMaxPlannedVUs(executionPlan)
	maxPossibleVUs := lib.GetMaxPossibleVUs(executionPlan)

	executionState := lib.NewExecutionState(options, et, maxPlannedVUs, maxPossibleVUs)
	maxDuration, _ := lib.GetEndOffset(executionPlan) // we don't care if the end offset is final

	executorConfigs := options.Scenarios.GetSortedConfigs()
	executors := make([]lib.Executor, 0, len(executorConfigs))
	// Only take executors which have work.
	for _, sc := range executorConfigs {
		if !sc.HasWork(et) {
			logger.Warnf(
				"Executor '%s' is disabled for segment %s due to lack of work!",
				sc.GetName(), options.ExecutionSegment,
			)
			continue
		}
		s, err := sc.NewExecutor(executionState, logger.WithFields(logrus.Fields{
			"scenario": sc.GetName(),
			"executor": sc.GetType(),
		}))
		if err != nil {
			return nil, err
		}
		executors = append(executors, s)
	}

	if options.Paused.Bool {
		if err := executionState.Pause(); err != nil {
			return nil, err
		}
	}

	return &ExecutionScheduler{
		runner:  runner,
		logger:  logger,
		options: options,

		initProgress:    pb.New(pb.WithConstLeft("Init")),
		executors:       executors,
		executorConfigs: executorConfigs,
		executionPlan:   executionPlan,
		maxDuration:     maxDuration,
		maxPossibleVUs:  maxPossibleVUs,
		state:           executionState,
	}, nil
}

// GetRunner returns the wrapped lib.Runner instance.
func (e *ExecutionScheduler) GetRunner() lib.Runner {
	return e.runner
}

// GetState returns a pointer to the execution state struct for the local
// execution scheduler. It's guaranteed to be initialized and present, though
// see the documentation in lib/execution.go for caveats about its usage. The
// most important one is that none of the methods beyond the pause-related ones
// should be used for synchronization.
func (e *ExecutionScheduler) GetState() *lib.ExecutionState {
	return e.state
}

// GetExecutors returns the slice of configured executor instances which
// have work, sorted by their (startTime, name) in an ascending order.
func (e *ExecutionScheduler) GetExecutors() []lib.Executor {
	return e.executors
}

// GetExecutorConfigs returns the slice of all executor configs, sorted by
// their (startTime, name) in an ascending order.
func (e *ExecutionScheduler) GetExecutorConfigs() []lib.ExecutorConfig {
	return e.executorConfigs
}

// GetInitProgressBar returns the progress bar associated with the Init
// function. After the Init is done, it is "hijacked" to display real-time
// execution statistics as a text bar.
func (e *ExecutionScheduler) GetInitProgressBar() *pb.ProgressBar {
	return e.initProgress
}

// GetExecutionPlan is a helper method so users of the local execution scheduler
// don't have to calculate the execution plan again.
func (e *ExecutionScheduler) GetExecutionPlan() []lib.ExecutionStep {
	return e.executionPlan
}

// initVU is a helper method that's used to both initialize the planned VUs
// in the Init() method, and also passed to executors so they can initialize
// any unplanned VUs themselves.
func (e *ExecutionScheduler) initVU(
	samplesOut chan<- stats.SampleContainer, logger *logrus.Entry,
) (lib.InitializedVU, error) {
	// Get the VU IDs here, so that the VUs are (mostly) ordered by their
	// number in the channel buffer
	vuIDLocal, vuIDGlobal := e.state.GetUniqueVUIdentifiers()
	vu, err := e.runner.NewVU(vuIDLocal, vuIDGlobal, samplesOut)
	if err != nil {
		return nil, errext.WithHint(err, fmt.Sprintf("error while initializing VU #%d", vuIDGlobal))
	}

	logger.Debugf("Initialized VU #%d", vuIDGlobal)
	return vu, nil
}

// getRunStats is a helper function that can be used as the execution
// scheduler's progressbar substitute (i.e. hijack).
func (e *ExecutionScheduler) getRunStats() string {
	status := "running"
	if e.state.IsPaused() {
		status = "paused"
	}
	if e.state.HasStarted() {
		dur := e.state.GetCurrentTestRunDuration()
		status = fmt.Sprintf("%s (%s)", status, pb.GetFixedLengthDuration(dur, e.maxDuration))
	}

	vusFmt := pb.GetFixedLengthIntFormat(int64(e.maxPossibleVUs))
	return fmt.Sprintf(
		"%s, "+vusFmt+"/"+vusFmt+" VUs, %d complete and %d interrupted iterations",
		status, e.state.GetCurrentlyActiveVUsCount(), e.state.GetInitializedVUsCount(),
		e.state.GetFullIterationCount(), e.state.GetPartialIterationCount(),
	)
}

func (e *ExecutionScheduler) initVUsConcurrently(
	ctx context.Context, samplesOut chan<- stats.SampleContainer, count uint64,
	concurrency int, logger *logrus.Entry,
) chan error {
	doneInits := make(chan error, count) // poor man's early-return waitgroup
	limiter := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		go func() {
			for range limiter {
				newVU, err := e.initVU(samplesOut, logger)
				if err == nil {
					e.state.AddInitializedVU(newVU)
				}
				doneInits <- err
			}
		}()
	}

	go func() {
		defer close(limiter)
		for vuNum := uint64(0); vuNum < count; vuNum++ {
			select {
			case limiter <- struct{}{}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return doneInits
}

// Init concurrently initializes all of the planned VUs and then sequentially
// initializes all of the configured executors.
func (e *ExecutionScheduler) Init(ctx context.Context, samplesOut chan<- stats.SampleContainer) error {
	logger := e.logger.WithField("phase", "local-execution-scheduler-init")

	vusToInitialize := lib.GetMaxPlannedVUs(e.executionPlan)
	logger.WithFields(logrus.Fields{
		"neededVUs":      vusToInitialize,
		"executorsCount": len(e.executors),
	}).Debugf("Start of initialization")

	subctx, cancel := context.WithCancel(ctx)
	defer cancel()

	e.state.SetExecutionStatus(lib.ExecutionStatusInitVUs)
	doneInits := e.initVUsConcurrently(subctx, samplesOut, vusToInitialize, runtime.GOMAXPROCS(0), logger)

	initializedVUs := new(uint64)
	vusFmt := pb.GetFixedLengthIntFormat(int64(vusToInitialize))
	e.initProgress.Modify(
		pb.WithProgress(func() (float64, []string) {
			doneVUs := atomic.LoadUint64(initializedVUs)
			right := fmt.Sprintf(vusFmt+"/%d VUs initialized", doneVUs, vusToInitialize)
			return float64(doneVUs) / float64(vusToInitialize), []string{right}
		}),
	)

	for vuNum := uint64(0); vuNum < vusToInitialize; vuNum++ {
		select {
		case err := <-doneInits:
			if err != nil {
				logger.WithError(err).Debug("VU initialization returned with an error, aborting...")
				// the context's cancel() is called in a defer above and will
				// abort any in-flight VU initializations
				return err
			}
			atomic.AddUint64(initializedVUs, 1)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	e.state.SetInitVUFunc(func(ctx context.Context, logger *logrus.Entry) (lib.InitializedVU, error) {
		return e.initVU(samplesOut, logger)
	})

	e.state.SetExecutionStatus(lib.ExecutionStatusInitExecutors)
	logger.Debugf("Finished initializing needed VUs, start initializing executors...")
	for _, exec := range e.executors {
		executorConfig := exec.GetConfig()

		if err := exec.Init(ctx); err != nil {
			return fmt.Errorf("error while initializing executor %s: %w", executorConfig.GetName(), err)
		}
		logger.Debugf("Initialized executor %s", executorConfig.GetName())
	}

	e.state.SetExecutionStatus(lib.ExecutionStatusInitDone)
	logger.Debugf("Initialization completed")
	return nil
}

// runExecutor gets called by the public Run() method once per configured
// executor, each time in a new goroutine. It is responsible for waiting out the
// configured startTime for the specific executor and then running its Run()
// method.
func (e *ExecutionScheduler) runExecutor(
	runCtx context.Context, runResults chan<- error, engineOut chan<- stats.SampleContainer, executor lib.Executor,
	builtinMetrics *metrics.BuiltinMetrics,
) {
	executorConfig := executor.GetConfig()
	executorStartTime := executorConfig.GetStartTime()
	executorLogger := e.logger.WithFields(logrus.Fields{
		"executor":  executorConfig.GetName(),
		"type":      executorConfig.GetType(),
		"startTime": executorStartTime,
	})
	executorProgress := executor.GetProgress()

	// Check if we have to wait before starting the actual executor execution
	if executorStartTime > 0 {
		startTime := time.Now()
		executorProgress.Modify(
			pb.WithStatus(pb.Waiting),
			pb.WithProgress(func() (float64, []string) {
				remWait := (executorStartTime - time.Since(startTime))
				return 0, []string{"waiting", pb.GetFixedLengthDuration(remWait, executorStartTime)}
			}),
		)

		executorLogger.Debugf("Waiting for executor start time...")
		select {
		case <-runCtx.Done():
			runResults <- nil // no error since executor hasn't started yet
			return
		case <-time.After(executorStartTime):
			// continue
		}
	}

	executorProgress.Modify(
		pb.WithStatus(pb.Running),
		pb.WithConstProgress(0, "started"),
	)
	executorLogger.Debugf("Starting executor")
	err := executor.Run(runCtx, engineOut, builtinMetrics) // executor should handle context cancel itself
	if err == nil {
		executorLogger.Debugf("Executor finished successfully")
	} else {
		executorLogger.WithField("error", err).Errorf("Executor error")
	}
	runResults <- err
}

// Run the ExecutionScheduler, funneling all generated metric samples through the supplied
// out channel.
//nolint:cyclop
func (e *ExecutionScheduler) Run(
	globalCtx, runCtx context.Context, engineOut chan<- stats.SampleContainer, builtinMetrics *metrics.BuiltinMetrics,
) error {
	executorsCount := len(e.executors)
	logger := e.logger.WithField("phase", "local-execution-scheduler-run")
	e.initProgress.Modify(pb.WithConstLeft("Run"))
	defer e.state.MarkEnded()

	if e.state.IsPaused() {
		logger.Debug("Execution is paused, waiting for resume or interrupt...")
		e.state.SetExecutionStatus(lib.ExecutionStatusPausedBeforeRun)
		e.initProgress.Modify(pb.WithConstProgress(1, "paused"))
		select {
		case <-e.state.ResumeNotify():
			// continue
		case <-runCtx.Done():
			return nil
		}
	}

	e.state.MarkStarted()
	e.initProgress.Modify(pb.WithConstProgress(1, "running"))

	logger.WithFields(logrus.Fields{"executorsCount": executorsCount}).Debugf("Start of test run")

	runResults := make(chan error, executorsCount) // nil values are successful runs

	runCtx = lib.WithExecutionState(runCtx, e.state)
	runSubCtx, cancel := context.WithCancel(runCtx)
	defer cancel() // just in case, and to shut up go vet...

	// Run setup() before any executors, if it's not disabled
	if !e.options.NoSetup.Bool {
		logger.Debug("Running setup()")
		e.state.SetExecutionStatus(lib.ExecutionStatusSetup)
		e.initProgress.Modify(pb.WithConstProgress(1, "setup()"))
		if err := e.runner.Setup(runSubCtx, engineOut); err != nil {
			logger.WithField("error", err).Debug("setup() aborted by error")
			return err
		}
	}
	e.initProgress.Modify(pb.WithHijack(e.getRunStats))

	// Start all executors at their particular startTime in a separate goroutine...
	logger.Debug("Start all executors...")
	e.state.SetExecutionStatus(lib.ExecutionStatusRunning)
	for _, exec := range e.executors {
		go e.runExecutor(runSubCtx, runResults, engineOut, exec, builtinMetrics)
	}

	// Wait for all executors to finish
	var firstErr error
	for range e.executors {
		err := <-runResults
		if err != nil && firstErr == nil {
			logger.WithError(err).Debug("Executor returned with an error, cancelling test run...")
			firstErr = err
			cancel()
		}
	}

	// Run teardown() after all executors are done, if it's not disabled
	if !e.options.NoTeardown.Bool {
		logger.Debug("Running teardown()")
		e.state.SetExecutionStatus(lib.ExecutionStatusTeardown)
		e.initProgress.Modify(pb.WithConstProgress(1, "teardown()"))

		// We run teardown() with the global context, so it isn't interrupted by
		// aborts caused by thresholds or even Ctrl+C (unless used twice).
		if err := e.runner.Teardown(globalCtx, engineOut); err != nil {
			logger.WithField("error", err).Debug("teardown() aborted by error")
			return err
		}
	}

	return firstErr
}

// SetPaused pauses a test, if called with true. And if called with false, tries
// to start/resume it. See the lib.ExecutionScheduler interface documentation of
// the methods for the various caveats about its usage.
func (e *ExecutionScheduler) SetPaused(pause bool) error {
	if !e.state.HasStarted() && e.state.IsPaused() {
		if pause {
			return fmt.Errorf("execution is already paused")
		}
		e.logger.Debug("Starting execution")
		return e.state.Resume()
	}

	for _, exec := range e.executors {
		pausableExecutor, ok := exec.(lib.PausableExecutor)
		if !ok {
			return fmt.Errorf(
				"%s executor '%s' doesn't support pause and resume operations after its start",
				exec.GetConfig().GetType(), exec.GetConfig().GetName(),
			)
		}
		if err := pausableExecutor.SetPaused(pause); err != nil {
			return err
		}
	}
	if pause {
		return e.state.Pause()
	}
	return e.state.Resume()
}
