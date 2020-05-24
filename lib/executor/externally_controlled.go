/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package executor

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	null "gopkg.in/guregu/null.v4"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/ui/pb"
)

const externallyControlledType = "externally-controlled"

func init() {
	lib.RegisterExecutorConfigType(
		externallyControlledType,
		func(name string, rawJSON []byte) (lib.ExecutorConfig, error) {
			config := ExternallyControlledConfig{BaseConfig: NewBaseConfig(name, externallyControlledType)}
			err := lib.StrictJSONUnmarshal(rawJSON, &config)
			if err != nil {
				return config, err
			}
			if !config.MaxVUs.Valid {
				config.MaxVUs = config.VUs
			}
			return config, nil
		},
	)
}

// ExternallyControlledConfigParams contains all of the options that actually
// determine the scheduling of VUs in the externally controlled executor.
type ExternallyControlledConfigParams struct {
	VUs      null.Int           `json:"vus"`
	Duration types.NullDuration `json:"duration"` // 0 is a valid value, meaning infinite duration
	MaxVUs   null.Int           `json:"maxVUs"`
}

// Validate just checks the control options in isolation.
func (mecc ExternallyControlledConfigParams) Validate() (errors []error) {
	if mecc.VUs.Int64 < 0 {
		errors = append(errors, fmt.Errorf("the number of VUs shouldn't be negative"))
	}

	if mecc.MaxVUs.Int64 < mecc.VUs.Int64 {
		errors = append(errors, fmt.Errorf(
			"the specified maxVUs (%d) should be more than or equal to the the number of active VUs (%d)",
			mecc.MaxVUs.Int64, mecc.VUs.Int64,
		))
	}

	if !mecc.Duration.Valid {
		errors = append(errors, fmt.Errorf("the duration should be specified, for infinite duration use 0"))
	} else if time.Duration(mecc.Duration.Duration) < 0 {
		errors = append(errors, fmt.Errorf(
			"the duration shouldn't be negative, for infinite duration use 0",
		))
	}

	return errors
}

// ExternallyControlledConfig stores the number of currently active VUs, the max
// number of VUs and the executor duration. The duration can be 0, which means
// "infinite duration", i.e. the user has to manually abort the script.
type ExternallyControlledConfig struct {
	BaseConfig
	ExternallyControlledConfigParams
}

// Make sure we implement the lib.ExecutorConfig interface
var _ lib.ExecutorConfig = &ExternallyControlledConfig{}

// GetDescription returns a human-readable description of the executor options
func (mec ExternallyControlledConfig) GetDescription(_ *lib.ExecutionTuple) string {
	duration := "infinite"
	if mec.Duration.Duration != 0 {
		duration = mec.Duration.String()
	}
	return fmt.Sprintf(
		"Externally controlled execution with %d VUs, %d max VUs, %s duration",
		mec.VUs.Int64, mec.MaxVUs.Int64, duration,
	)
}

// Validate makes sure all options are configured and valid
func (mec ExternallyControlledConfig) Validate() []error {
	errors := append(mec.BaseConfig.Validate(), mec.ExternallyControlledConfigParams.Validate()...)
	if mec.GracefulStop.Valid {
		errors = append(errors, fmt.Errorf(
			"gracefulStop is not supported by the externally controlled executor",
		))
	}
	return errors
}

// GetExecutionRequirements reserves the configured number of max VUs for the
// whole duration of the executor, so these VUs can be externally initialized in
// the beginning of the test.
//
// Importantly, if 0 (i.e. infinite) duration is configured, this executor
// doesn't emit the last step to relinquish these VUs.
//
// Also, the externally controlled executor doesn't set MaxUnplannedVUs in the
// returned steps, since their initialization and usage is directly controlled
// by the user, can be changed during the test runtime, and is effectively
// bounded only by the resources of the machine k6 is running on.
//
// This is not a problem, because the MaxUnplannedVUs are mostly meant to be
// used for calculating the maximum possible number of initialized VUs at any
// point during a test run. That's used for sizing purposes and for user qouta
// checking in the cloud execution, where the externally controlled executor
// isn't supported.
func (mec ExternallyControlledConfig) GetExecutionRequirements(et *lib.ExecutionTuple) []lib.ExecutionStep {
	startVUs := lib.ExecutionStep{
		TimeOffset:      0,
		PlannedVUs:      uint64(et.Segment.Scale(mec.MaxVUs.Int64)), // user-configured, VUs to be pre-initialized
		MaxUnplannedVUs: 0,                                          // intentional, see function comment
	}

	maxDuration := time.Duration(mec.Duration.Duration)
	if maxDuration == 0 {
		// Infinite duration, don't emit 0 VUs at the end since there's no planned end
		return []lib.ExecutionStep{startVUs}
	}
	return []lib.ExecutionStep{startVUs, {
		TimeOffset:      maxDuration,
		PlannedVUs:      0,
		MaxUnplannedVUs: 0, // intentional, see function comment
	}}
}

// IsDistributable simply returns false because there's no way to reliably
// distribute the externally controlled executor.
func (ExternallyControlledConfig) IsDistributable() bool {
	return false
}

// NewExecutor creates a new ExternallyControlled executor
func (mec ExternallyControlledConfig) NewExecutor(es *lib.ExecutionState, logger *logrus.Entry) (lib.Executor, error) {
	return &ExternallyControlled{
		BaseExecutor:         NewBaseExecutor(mec, es, logger),
		config:               mec,
		currentControlConfig: mec.ExternallyControlledConfigParams,
		configLock:           &sync.RWMutex{},
		newControlConfigs:    make(chan updateConfigEvent),
		pauseEvents:          make(chan pauseEvent),
		hasStarted:           make(chan struct{}),
	}, nil
}

// HasWork reports whether there is any work to be done for the given execution segment.
func (mec ExternallyControlledConfig) HasWork(_ *lib.ExecutionTuple) bool {
	// We can always initialize new VUs via the REST API, so return true.
	return true
}

type pauseEvent struct {
	isPaused bool
	err      chan error
}

type updateConfigEvent struct {
	newConfig ExternallyControlledConfigParams
	err       chan error
}

// ExternallyControlled is an implementation of the old k6 executor that could be
// controlled externally, via the k6 REST API. It implements both the
// lib.PausableExecutor and the lib.LiveUpdatableExecutor interfaces.
type ExternallyControlled struct {
	*BaseExecutor
	config               ExternallyControlledConfig
	currentControlConfig ExternallyControlledConfigParams
	configLock           *sync.RWMutex
	newControlConfigs    chan updateConfigEvent
	pauseEvents          chan pauseEvent
	hasStarted           chan struct{}
}

// Make sure we implement all the interfaces
var (
	_ lib.Executor              = &ExternallyControlled{}
	_ lib.PausableExecutor      = &ExternallyControlled{}
	_ lib.LiveUpdatableExecutor = &ExternallyControlled{}
)

// GetCurrentConfig just returns the executor's current configuration.
func (mex *ExternallyControlled) GetCurrentConfig() ExternallyControlledConfig {
	mex.configLock.RLock()
	defer mex.configLock.RUnlock()
	return ExternallyControlledConfig{
		BaseConfig:                       mex.config.BaseConfig,
		ExternallyControlledConfigParams: mex.currentControlConfig,
	}
}

// GetConfig just returns the executor's current configuration, it's basically
// an alias of GetCurrentConfig that implements the more generic interface.
func (mex *ExternallyControlled) GetConfig() lib.ExecutorConfig {
	return mex.GetCurrentConfig()
}

// GetProgress just returns the executor's progress bar instance.
func (mex ExternallyControlled) GetProgress() *pb.ProgressBar {
	mex.configLock.RLock()
	defer mex.configLock.RUnlock()
	return mex.progress
}

// GetLogger just returns the executor's logger instance.
func (mex ExternallyControlled) GetLogger() *logrus.Entry {
	mex.configLock.RLock()
	defer mex.configLock.RUnlock()
	return mex.logger
}

// Init doesn't do anything...
func (mex ExternallyControlled) Init(ctx context.Context) error {
	return nil
}

// SetPaused pauses or resumes the executor.
func (mex *ExternallyControlled) SetPaused(paused bool) error {
	select {
	case <-mex.hasStarted:
		event := pauseEvent{isPaused: paused, err: make(chan error)}
		mex.pauseEvents <- event
		return <-event.err
	default:
		return fmt.Errorf("cannot pause the externally controlled executor before it has started")
	}
}

// UpdateConfig validates the supplied config and updates it in real time. It is
// possible to update the configuration even when k6 is paused, either in the
// beginning (i.e. when running k6 with --paused) or in the middle of the script
// execution.
func (mex *ExternallyControlled) UpdateConfig(ctx context.Context, newConf interface{}) error {
	newConfigParams, ok := newConf.(ExternallyControlledConfigParams)
	if !ok {
		return errors.New("invalid config type")
	}
	if errs := newConfigParams.Validate(); len(errs) != 0 {
		return fmt.Errorf("invalid configuration supplied: %s", lib.ConcatErrors(errs, ", "))
	}

	if newConfigParams.Duration.Valid && newConfigParams.Duration != mex.config.Duration {
		return fmt.Errorf("the externally controlled executor duration cannot be changed")
	}
	if newConfigParams.MaxVUs.Valid && newConfigParams.MaxVUs.Int64 < mex.config.MaxVUs.Int64 {
		// This limitation is because the externally controlled executor is
		// still an executor that participates in the overall k6 scheduling.
		// Thus, any VUs that were explicitly specified by the user in the
		// config may be reused from or by other executors.
		return fmt.Errorf(
			"the new number of max VUs cannot be lower than the starting number of max VUs (%d)",
			mex.config.MaxVUs.Int64,
		)
	}

	mex.configLock.Lock() // guard against a simultaneous start of the test (which will close hasStarted)
	select {
	case <-mex.hasStarted:
		mex.configLock.Unlock()
		event := updateConfigEvent{newConfig: newConfigParams, err: make(chan error)}
		mex.newControlConfigs <- event
		return <-event.err
	case <-ctx.Done():
		mex.configLock.Unlock()
		return ctx.Err()
	default:
		mex.currentControlConfig = newConfigParams
		mex.configLock.Unlock()
		return nil
	}
}

// This is a helper function that is used in run for non-infinite durations.
func (mex *ExternallyControlled) stopWhenDurationIsReached(ctx context.Context, duration time.Duration, cancel func()) {
	ctxDone := ctx.Done()
	checkInterval := time.NewTicker(100 * time.Millisecond)
	for {
		select {
		case <-ctxDone:
			checkInterval.Stop()
			return

		// TODO: something more optimized that sleeps for pauses?
		case <-checkInterval.C:
			if mex.executionState.GetCurrentTestRunDuration() >= duration {
				cancel()
				return
			}
		}
	}
}

// manualVUHandle is a wrapper around the vuHandle helper, used in the
// variable-looping-vus executor. Here, instead of using its getVU and returnVU
// methods to retrieve and return a VU from the global buffer, we use them to
// accurately update the local and global active VU counters and to ensure that
// the pausing and reducing VUs operations wait for VUs to fully finish
// executing their current iterations before returning.
type manualVUHandle struct {
	*vuHandle
	initVU lib.InitializedVU
	wg     *sync.WaitGroup

	// This is the cancel of the local context, used to kill its goroutine when
	// we reduce the number of MaxVUs, so that the Go GC can clean up the VU.
	cancelVU func()
}

func (rs *externallyControlledRunState) newManualVUHandle(
	initVU lib.InitializedVU, logger *logrus.Entry,
) *manualVUHandle {
	wg := sync.WaitGroup{}
	state := rs.executor.executionState
	getVU := func() (lib.InitializedVU, error) {
		wg.Add(1)
		state.ModCurrentlyActiveVUsCount(+1)
		atomic.AddInt64(rs.activeVUsCount, +1)
		return initVU, nil
	}
	returnVU := func(_ lib.InitializedVU) {
		state.ModCurrentlyActiveVUsCount(-1)
		atomic.AddInt64(rs.activeVUsCount, -1)
		wg.Done()
	}
	ctx, cancel := context.WithCancel(rs.ctx)
	return &manualVUHandle{
		vuHandle: newStoppedVUHandle(ctx, getVU, returnVU, &rs.executor.config.BaseConfig, logger),
		initVU:   initVU,
		wg:       &wg,
		cancelVU: cancel,
	}
}

// externallyControlledRunState is created and initialized by the Run() method
// of the externally controlled executor. It is used to track and modify various
// details of the execution, including handling of live config changes.
type externallyControlledRunState struct {
	ctx             context.Context
	executor        *ExternallyControlled
	startMaxVUs     int64             // the scaled number of initially configured MaxVUs
	duration        time.Duration     // the total duration of the executor, could be 0 for infinite
	activeVUsCount  *int64            // the current number of active VUs, used only for the progress display
	maxVUs          *int64            // the current number of initialized VUs
	vuHandles       []*manualVUHandle // handles for manipulating and tracking all of the VUs
	currentlyPaused bool              // whether the executor is currently paused

	runIteration func(context.Context, lib.ActiveVU) // a helper closure function that runs a single iteration
}

// retrieveStartMaxVUs gets and initializes the (scaled) number of MaxVUs
// from the global VU buffer. These are the VUs that the user originally
// specified in the JS config, and that the ExecutionScheduler pre-initialized
// for us.
func (rs *externallyControlledRunState) retrieveStartMaxVUs() error {
	for i := int64(0); i < rs.startMaxVUs; i++ { // get the initial planned VUs from the common buffer
		initVU, vuGetErr := rs.executor.executionState.GetPlannedVU(rs.executor.logger, false)
		if vuGetErr != nil {
			return vuGetErr
		}
		vuHandle := rs.newManualVUHandle(initVU, rs.executor.logger.WithField("vuNum", i))
		go vuHandle.runLoopsIfPossible(rs.runIteration)
		rs.vuHandles[i] = vuHandle
	}
	return nil
}

func (rs *externallyControlledRunState) progresFn() (float64, []string) {
	// TODO: simulate spinner for the other case or cycle 0-100?
	currentActiveVUs := atomic.LoadInt64(rs.activeVUsCount)
	currentMaxVUs := atomic.LoadInt64(rs.maxVUs)
	vusFmt := pb.GetFixedLengthIntFormat(currentMaxVUs)
	progVUs := fmt.Sprintf(vusFmt+"/"+vusFmt+" VUs", currentActiveVUs, currentMaxVUs)

	right := []string{progVUs, rs.duration.String(), ""}

	spent := rs.executor.executionState.GetCurrentTestRunDuration()
	if spent > rs.duration {
		return 1, right
	}

	progress := 0.0
	if rs.duration > 0 {
		progress = math.Min(1, float64(spent)/float64(rs.duration))
	}

	spentDuration := pb.GetFixedLengthDuration(spent, rs.duration)
	progDur := fmt.Sprintf("%s/%s", spentDuration, rs.duration)
	right[1] = progDur

	return progress, right
}

func (rs *externallyControlledRunState) handleConfigChange(oldCfg, newCfg ExternallyControlledConfigParams) error {
	executionState := rs.executor.executionState
	segment := executionState.Options.ExecutionSegment
	oldActiveVUs := segment.Scale(oldCfg.VUs.Int64)
	oldMaxVUs := segment.Scale(oldCfg.MaxVUs.Int64)
	newActiveVUs := segment.Scale(newCfg.VUs.Int64)
	newMaxVUs := segment.Scale(newCfg.MaxVUs.Int64)

	rs.executor.logger.WithFields(logrus.Fields{
		"oldActiveVUs": oldActiveVUs, "oldMaxVUs": oldMaxVUs,
		"newActiveVUs": newActiveVUs, "newMaxVUs": newMaxVUs,
	}).Debug("Updating execution configuration...")

	for i := oldMaxVUs; i < newMaxVUs; i++ {
		select { // check if the user didn't try to abort k6 while we're scaling up the VUs
		case <-rs.ctx.Done():
			return rs.ctx.Err()
		default: // do nothing
		}
		initVU, vuInitErr := executionState.InitializeNewVU(rs.ctx, rs.executor.logger)
		if vuInitErr != nil {
			return vuInitErr
		}
		vuHandle := rs.newManualVUHandle(initVU, rs.executor.logger.WithField("vuNum", i))
		go vuHandle.runLoopsIfPossible(rs.runIteration)
		rs.vuHandles = append(rs.vuHandles, vuHandle)
	}

	if oldActiveVUs < newActiveVUs {
		for i := oldActiveVUs; i < newActiveVUs; i++ {
			if !rs.currentlyPaused {
				rs.vuHandles[i].start()
			}
		}
	} else {
		for i := newActiveVUs; i < oldActiveVUs; i++ {
			rs.vuHandles[i].hardStop()
		}
		for i := newActiveVUs; i < oldActiveVUs; i++ {
			rs.vuHandles[i].wg.Wait()
		}
	}

	if oldMaxVUs > newMaxVUs {
		for i := newMaxVUs; i < oldMaxVUs; i++ {
			rs.vuHandles[i].cancelVU()
			if i < rs.startMaxVUs {
				// return the initial planned VUs to the common buffer
				executionState.ReturnVU(rs.vuHandles[i].initVU, false)
			}
			rs.vuHandles[i] = nil
		}
		rs.vuHandles = rs.vuHandles[:newMaxVUs]
	}

	atomic.StoreInt64(rs.maxVUs, newMaxVUs)
	return nil
}

// Run constantly loops through as many iterations as possible on a variable
// dynamically controlled number of VUs either for the specified duration, or
// until the test is manually stopped.
// nolint:funlen,gocognit
func (mex *ExternallyControlled) Run(parentCtx context.Context, out chan<- stats.SampleContainer) (err error) {
	mex.configLock.RLock()
	// Safely get the current config - it's important that the close of the
	// hasStarted channel is inside of the lock, so that there are no data races
	// between it and the UpdateConfig() method.
	currentControlConfig := mex.currentControlConfig
	close(mex.hasStarted)
	mex.configLock.RUnlock()

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	duration := time.Duration(currentControlConfig.Duration.Duration)
	if duration > 0 { // Only keep track of duration if it's not infinite
		go mex.stopWhenDurationIsReached(ctx, duration, cancel)
	}

	mex.logger.WithFields(
		logrus.Fields{"type": externallyControlledType, "duration": duration},
	).Debug("Starting executor run...")

	startMaxVUs := mex.executionState.Options.ExecutionSegment.Scale(mex.config.MaxVUs.Int64)
	runState := &externallyControlledRunState{
		ctx:             ctx,
		executor:        mex,
		startMaxVUs:     startMaxVUs,
		duration:        duration,
		vuHandles:       make([]*manualVUHandle, startMaxVUs),
		currentlyPaused: false,
		activeVUsCount:  new(int64),
		maxVUs:          new(int64),
		runIteration:    getIterationRunner(mex.executionState, mex.logger),
	}
	*runState.maxVUs = startMaxVUs
	if err = runState.retrieveStartMaxVUs(); err != nil {
		return err
	}

	mex.progress.Modify(pb.WithProgress(runState.progresFn)) // Keep track of the progress
	go trackProgress(parentCtx, ctx, ctx, mex, runState.progresFn)

	err = runState.handleConfigChange( // Start by setting MaxVUs to the starting MaxVUs
		ExternallyControlledConfigParams{MaxVUs: mex.config.MaxVUs}, currentControlConfig,
	)
	if err != nil {
		return err
	}
	defer func() { // Make sure we release the VUs at the end
		err = runState.handleConfigChange(currentControlConfig, ExternallyControlledConfigParams{})
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case updateConfigEvent := <-mex.newControlConfigs:
			err := runState.handleConfigChange(currentControlConfig, updateConfigEvent.newConfig)
			if err != nil {
				updateConfigEvent.err <- err
				if ctx.Err() == err {
					return nil // we've already returned an error to the API client, but k6 should stop normally
				}
				return err
			}
			currentControlConfig = updateConfigEvent.newConfig
			mex.configLock.Lock()
			mex.currentControlConfig = updateConfigEvent.newConfig
			mex.configLock.Unlock()
			updateConfigEvent.err <- nil

		case pauseEvent := <-mex.pauseEvents:
			if pauseEvent.isPaused == runState.currentlyPaused {
				pauseEvent.err <- nil
				continue
			}
			activeVUs := currentControlConfig.VUs.Int64
			if pauseEvent.isPaused {
				for i := int64(0); i < activeVUs; i++ {
					runState.vuHandles[i].gracefulStop()
				}
				for i := int64(0); i < activeVUs; i++ {
					runState.vuHandles[i].wg.Wait()
				}
			} else {
				for i := int64(0); i < activeVUs; i++ {
					runState.vuHandles[i].start()
				}
			}
			runState.currentlyPaused = pauseEvent.isPaused
			pauseEvent.err <- nil
		}
	}
}
