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

	"github.com/loadimpact/k6/ui/pb"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/sirupsen/logrus"
)

// Executor is the local implementation of lib.Executor
type Executor struct {
	runner  lib.Runner
	options lib.Options
	logger  *logrus.Logger

	initProgress   *pb.ProgressBar
	schedulers     []lib.Scheduler // sorted by (startTime, ID)
	executionPlan  []lib.ExecutionStep
	maxDuration    time.Duration // cached value derived from the execution plan
	maxPossibleVUs uint64        // cached value derived from the execution plan
	state          *lib.ExecutorState
}

// Check to see if we implement the lib.Executor interface
var _ lib.Executor = &Executor{}

// New creates and returns a new local lib.Executor instance, without
// initializing it beyond the bare minimum. Specifically, it creates the needed
// schedulers instances and a lot of state placeholders, but it doesn't
// initialize the schedulers and it doesn't initialize or run any VUs.
func New(runner lib.Runner, logger *logrus.Logger) (*Executor, error) {
	options := runner.GetOptions()

	executionPlan := options.Execution.GetFullExecutionRequirements(options.ExecutionSegment)
	maxPlannedVUs := lib.GetMaxPlannedVUs(executionPlan)
	maxPossibleVUs := lib.GetMaxPossibleVUs(executionPlan)

	executorState := lib.NewExecutorState(options, maxPlannedVUs, maxPossibleVUs)
	maxDuration, _ := lib.GetEndOffset(executionPlan) // we don't care if the end offset is final

	schedulerConfigs := options.Execution.GetSortedSchedulerConfigs()
	schedulers := make([]lib.Scheduler, len(schedulerConfigs))
	for i, sc := range schedulerConfigs {
		s, err := sc.NewScheduler(executorState, logger.WithField("scheduler", sc.GetName()))
		if err != nil {
			return nil, err
		}
		schedulers[i] = s
	}

	if options.Paused.Bool {
		if err := executorState.Pause(); err != nil {
			return nil, err
		}
	}

	return &Executor{
		runner:  runner,
		logger:  logger,
		options: options,

		initProgress:   pb.New(pb.WithConstLeft("Init")),
		schedulers:     schedulers,
		executionPlan:  executionPlan,
		maxDuration:    maxDuration,
		maxPossibleVUs: maxPossibleVUs,
		state:          executorState,
	}, nil
}

// GetRunner returns the wrapped lib.Runner instance.
func (e *Executor) GetRunner() lib.Runner {
	return e.runner
}

// GetState returns a pointer to the executor state struct for the local
// executor. It's guaranteed to be initialized and present, though see
// the documentation in lib/executor.go for caveats about its usage.
// The most important one is that none of the methods beyond the pause-related
// ones should be used for synchronization.
func (e *Executor) GetState() *lib.ExecutorState {
	return e.state
}

// GetSchedulers returns the slice of configured scheduler instances, sorted by
// their (startTime, name) in an ascending order.
func (e *Executor) GetSchedulers() []lib.Scheduler {
	return e.schedulers
}

// GetInitProgressBar returns a the progress bar assotiated with the Init
// function. After the Init is done, it is "hijacked" to display real-time
// execution statistics as a text bar.
func (e *Executor) GetInitProgressBar() *pb.ProgressBar {
	return e.initProgress
}

// GetExecutionPlan is a helper method so users of the local executor don't have
// to calculate the execution plan again.
func (e *Executor) GetExecutionPlan() []lib.ExecutionStep {
	return e.executionPlan
}

// initVU is just a helper method that's used to both initialize the planned VUs
// in the Init() method, and also passed to schedulers so they can initialize
// any unplanned VUs themselves.
//TODO: actually use the context...
func (e *Executor) initVU(
	_ context.Context, logger *logrus.Entry, engineOut chan<- stats.SampleContainer,
) (lib.VU, error) {

	vu, err := e.runner.NewVU(engineOut)
	if err != nil {
		return nil, fmt.Errorf("error while initializing a VU: '%s'", err)
	}

	// Get the VU ID here, so that the VUs are (mostly) ordered by their
	// number in the channel buffer
	vuID := e.state.GetUniqueVUIdentifier()
	if err := vu.Reconfigure(int64(vuID)); err != nil {
		return nil, fmt.Errorf("error while reconfiguring VU #%d: '%s'", vuID, err)

	}
	logger.Debugf("Initialized VU #%d", vuID)
	return vu, nil
}

// getRunStats is a helper function that can be used as the executor's
// progressbar substitute (i.e. hijack).
func (e *Executor) getRunStats() string {
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
		"%s, "+vusFmt+"/"+vusFmt+" VUs, %d complete and %d incomplete iterations",
		status, e.state.GetCurrentlyActiveVUsCount(), e.state.GetInitializedVUsCount(),
		e.state.GetFullIterationCount(), e.state.GetPartialIterationCount(),
	)
}

// Init concurrently initializes all of the planned VUs and then sequentially
// initializes all of the configured schedulers.
func (e *Executor) Init(ctx context.Context, engineOut chan<- stats.SampleContainer) error {
	logger := e.logger.WithField("phase", "local-executor-init")

	vusToInitialize := lib.GetMaxPlannedVUs(e.executionPlan)
	logger.WithFields(logrus.Fields{
		"neededVUs":       vusToInitialize,
		"schedulersCount": len(e.schedulers),
	}).Debugf("Start of initialization")

	doneInits := make(chan error, vusToInitialize) // poor man's early-return waitgroup
	//TODO: make this an option?
	initConcurrency := runtime.NumCPU()
	limiter := make(chan struct{}, initConcurrency)
	subctx, cancel := context.WithCancel(ctx)
	defer cancel()

	initPlannedVU := func() {
		newVU, err := e.initVU(ctx, logger, engineOut)
		if err == nil {
			e.state.AddInitializedVU(newVU)
			<-limiter
		}
		doneInits <- err
	}

	go func() {
		for vuNum := uint64(0); vuNum < vusToInitialize; vuNum++ {
			select {
			case limiter <- struct{}{}:
				go initPlannedVU()
			case <-subctx.Done():
				return
			}
		}
	}()

	initializedVUs := new(uint64)
	vusFmt := pb.GetFixedLengthIntFormat(int64(vusToInitialize))
	e.initProgress.Modify(
		pb.WithProgress(func() (float64, string) {
			doneVUs := atomic.LoadUint64(initializedVUs)
			return float64(doneVUs) / float64(vusToInitialize),
				fmt.Sprintf(vusFmt+"/%d VUs initialized", doneVUs, vusToInitialize)
		}),
	)

	for vuNum := uint64(0); vuNum < vusToInitialize; vuNum++ {
		select {
		case err := <-doneInits:
			if err != nil {
				return err
			}
			atomic.AddUint64(initializedVUs, 1)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	e.state.SetInitVUFunc(func(ctx context.Context, logger *logrus.Entry) (lib.VU, error) {
		return e.initVU(ctx, logger, engineOut)
	})

	logger.Debugf("Finished initializing needed VUs, start initializing schedulers...")
	for _, sched := range e.schedulers {
		schedConfig := sched.GetConfig()

		if err := sched.Init(ctx); err != nil {
			return fmt.Errorf("error while initializing scheduler %s: %s", schedConfig.GetName(), err)
		}
		logger.Debugf("Initialized scheduler %s", schedConfig.GetName())
	}

	logger.Debugf("Initization completed")
	return nil
}

// Run the Executor, funneling all generated metric samples through the supplied
// out channel.
func (e *Executor) Run(ctx context.Context, engineOut chan<- stats.SampleContainer) error {
	schedulersCount := len(e.schedulers)
	logger := e.logger.WithField("phase", "local-executor-run")
	e.initProgress.Modify(pb.WithConstLeft("Run"))

	if e.state.IsPaused() {
		logger.Debug("Execution is paused, waiting for resume or interrupt...")
		e.initProgress.Modify(pb.WithConstProgress(1, "paused"))
		select {
		case <-e.state.ResumeNotify():
			// continue
		case <-ctx.Done():
			return nil
		}
	}

	e.state.MarkStarted()
	defer e.state.MarkEnded()
	e.initProgress.Modify(pb.WithConstProgress(1, "running"))

	logger.WithFields(logrus.Fields{"schedulersCount": schedulersCount}).Debugf("Start of test run")

	runResults := make(chan error, schedulersCount) // nil values are successful runs

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel() // just in case, and to shut up go vet...

	// Run setup() before any schedulers, if it's not disabled
	if !e.options.NoSetup.Bool {
		logger.Debug("Running setup()")
		e.initProgress.Modify(pb.WithConstProgress(1, "setup()"))
		if err := e.runner.Setup(runCtx, engineOut); err != nil {
			logger.WithField("error", err).Debug("setup() aborted by error")
			return err
		}
	}
	e.initProgress.Modify(pb.WithHijack(e.getRunStats))

	runCtxDone := runCtx.Done()
	runScheduler := func(sched lib.Scheduler) {
		schedConfig := sched.GetConfig()
		schedStartTime := schedConfig.GetStartTime()
		schedLogger := logger.WithFields(logrus.Fields{
			"scheduler": schedConfig.GetName(),
			"type":      schedConfig.GetType(),
			"startTime": schedStartTime,
		})
		schedProgress := sched.GetProgress()

		// Check if we have to wait before starting the actual scheduler execution
		if schedStartTime > 0 {
			startTime := time.Now()
			schedProgress.Modify(pb.WithProgress(func() (float64, string) {
				remWait := (schedStartTime - time.Since(startTime))
				return 0, fmt.Sprintf("waiting %s", pb.GetFixedLengthDuration(remWait, schedStartTime))
			}))

			schedLogger.Debugf("Waiting for scheduler start time...")
			select {
			case <-runCtxDone:
				runResults <- nil // no error since scheduler hasn't started yet
				return
			case <-time.After(schedStartTime):
				// continue
			}
		}

		schedProgress.Modify(pb.WithConstProgress(0, "started"))
		schedLogger.Debugf("Starting scheduler")
		err := sched.Run(runCtx, engineOut) // scheduler should handle context cancel itself
		if err == nil {
			schedLogger.Debugf("Scheduler finished successfully")
		} else {
			schedLogger.WithField("error", err).Errorf("Scheduler error")
		}
		runResults <- err
	}

	// Start all schedulers at their particular startTime in a separate goroutine...
	logger.Debug("Start all schedulers...")
	for _, sched := range e.schedulers {
		go runScheduler(sched)
	}

	// Wait for all schedulers to finish
	var firstErr error
	for range e.schedulers {
		err := <-runResults
		if err != nil && firstErr == nil {
			firstErr = err
			cancel()
		}
	}

	// Run teardown() after all schedulers are done, if it's not disabled
	if !e.options.NoTeardown.Bool {
		logger.Debug("Running teardown()")
		if err := e.runner.Teardown(ctx, engineOut); err != nil {
			logger.WithField("error", err).Debug("teardown() aborted by error")
			return err
		}
	}

	return firstErr
}

// SetPaused pauses a test, if called with true. And if called with
// false, tries to start/resume it. See the lib.Executor interface documentation
// of the methods for the various caveats about its usage.
func (e *Executor) SetPaused(pause bool) error {
	if !e.state.HasStarted() && e.state.IsPaused() {
		if pause {
			return fmt.Errorf("execution is already paused")
		}
		e.logger.Debug("Starting execution")
		return e.state.Resume()
	}

	for _, sched := range e.schedulers {
		pausableSched, ok := sched.(lib.PausableScheduler)
		if !ok {
			return fmt.Errorf(
				"%s scheduler '%s' doesn't support pause and resume operations after its start",
				sched.GetConfig().GetType(), sched.GetConfig().GetName(),
			)
		}
		if err := pausableSched.SetPaused(pause); err != nil {
			return err
		}
	}
	if pause {
		return e.state.Pause()
	}
	return e.state.Resume()
}
