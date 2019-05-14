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

package scheduler

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/ui/pb"
	"github.com/sirupsen/logrus"
	null "gopkg.in/guregu/null.v3"
)

const manualExecutionType = "manual-execution"

func init() {
	lib.RegisterSchedulerConfigType(
		manualExecutionType,
		func(name string, rawJSON []byte) (lib.SchedulerConfig, error) {
			config := ManualExecutionConfig{BaseConfig: NewBaseConfig(name, manualExecutionType)}
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

// ManualExecutionControlConfig contains all of the options that actually
// determine the scheduling of VUs in the manual execution scheduler.
type ManualExecutionControlConfig struct {
	VUs      null.Int           `json:"vus"`
	Duration types.NullDuration `json:"duration"`
	MaxVUs   null.Int           `json:"maxVUs"`
}

// Validate just checks the control options in isolation.
func (mecc ManualExecutionControlConfig) Validate() (errors []error) {
	if mecc.VUs.Int64 < 0 {
		errors = append(errors, fmt.Errorf("the number of VUs shouldn't be negative"))
	}

	if mecc.MaxVUs.Int64 < mecc.VUs.Int64 {
		errors = append(errors, fmt.Errorf(
			"the specified maxVUs (%d) should more than or equal to the the number of active VUs (%d)",
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

// ManualExecutionConfig stores the number of currently active VUs, the max
// number of VUs and the scheduler duration. The duration can be 0, which means
// "infinite duration", i.e. the user has to manually abort the script.
type ManualExecutionConfig struct {
	BaseConfig
	ManualExecutionControlConfig
}

// Make sure we implement the lib.SchedulerConfig interface
var _ lib.SchedulerConfig = &ManualExecutionConfig{}

// GetDescription returns a human-readable description of the scheduler options
func (mec ManualExecutionConfig) GetDescription(_ *lib.ExecutionSegment) string {
	duration := "infinite"
	if mec.Duration.Duration != 0 {
		duration = mec.Duration.String()
	}
	return fmt.Sprintf(
		"Manually controlled execution with %d VUs, %d max VUs, %s duration",
		mec.VUs.Int64, mec.MaxVUs.Int64, duration,
	)
}

// Validate makes sure all options are configured and valid
func (mec ManualExecutionConfig) Validate() []error {
	errors := append(mec.BaseConfig.Validate(), mec.ManualExecutionControlConfig.Validate()...)
	if mec.GracefulStop.Valid {
		errors = append(errors, fmt.Errorf(
			"gracefulStop is not supported by the manual execution scheduler",
		))
	}
	return errors
}

// GetExecutionRequirements just reserves the specified number of max VUs for
// the whole duration of the scheduler, so these VUs can be initialized in the
// beginning of the test.
//
// Importantly, if 0 (i.e. infinite) duration is configured, this scheduler
// doesn't emit the last step to relinquish these VUs.
//
// Also, the manual execution scheduler doesn't set MaxUnplannedVUs in the
// returned steps, since their initialization and usage is directly controlled
// by the user and is effectively bounded only by the resources of the machine
// k6 is running on.
//
// This is not a problem, because the MaxUnplannedVUs are mostly meant to be
// used for calculating the maximum possble number of initialized VUs at any
// point during a test run. That's used for sizing purposes and for user qouta
// checking in the cloud execution, where the manual scheduler isn't supported.
func (mec ManualExecutionConfig) GetExecutionRequirements(es *lib.ExecutionSegment) []lib.ExecutionStep {
	startVUs := lib.ExecutionStep{
		TimeOffset:      0,
		PlannedVUs:      uint64(es.Scale(mec.MaxVUs.Int64)), // use
		MaxUnplannedVUs: 0,                                  // intentional, see function comment
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
// distribute the manual execution scheduler.
func (ManualExecutionConfig) IsDistributable() bool {
	return false
}

// NewScheduler creates a new ManualExecution "scheduler"
func (mec ManualExecutionConfig) NewScheduler(
	es *lib.ExecutorState, logger *logrus.Entry) (lib.Scheduler, error) {

	return &ManualExecution{
		startConfig:          mec,
		currentControlConfig: mec.ManualExecutionControlConfig,
		configLock:           &sync.RWMutex{},
		newControlConfigs:    make(chan updateConfigEvent),
		pauseEvents:          make(chan pauseEvent),
		hasStarted:           make(chan struct{}),

		executorState: es,
		logger:        logger,
		progress:      pb.New(pb.WithLeft(mec.GetName)),
	}, nil
}

type pauseEvent struct {
	isPaused bool
	err      chan error
}

type updateConfigEvent struct {
	newConfig ManualExecutionControlConfig
	err       chan error
}

// ManualExecution is an implementation of the old k6 scheduler that could be
// controlled externally, via the k6 REST API. It implements both the
// lib.PausableScheduler and the lib.LiveUpdatableScheduler interfaces.
type ManualExecution struct {
	startConfig          ManualExecutionConfig
	currentControlConfig ManualExecutionControlConfig
	configLock           *sync.RWMutex
	newControlConfigs    chan updateConfigEvent
	pauseEvents          chan pauseEvent
	hasStarted           chan struct{}

	executorState *lib.ExecutorState
	logger        *logrus.Entry
	progress      *pb.ProgressBar
}

// Make sure we implement all the interfaces
var _ lib.Scheduler = &ManualExecution{}
var _ lib.PausableScheduler = &ManualExecution{}
var _ lib.LiveUpdatableScheduler = &ManualExecution{}

// GetCurrentConfig just returns the scheduler's current configuration.
func (mex *ManualExecution) GetCurrentConfig() ManualExecutionConfig {
	mex.configLock.RLock()
	defer mex.configLock.RUnlock()
	return ManualExecutionConfig{
		BaseConfig:                   mex.startConfig.BaseConfig,
		ManualExecutionControlConfig: mex.currentControlConfig,
	}
}

// GetConfig just returns the scheduler's current configuration, it's basically
// an alias of GetCurrentConfig that implements the more generic interface.
func (mex *ManualExecution) GetConfig() lib.SchedulerConfig {
	return mex.GetCurrentConfig()
}

// GetProgress just returns the scheduler's progress bar instance.
func (mex ManualExecution) GetProgress() *pb.ProgressBar {
	return mex.progress
}

// GetLogger just returns the scheduler's logger instance.
func (mex ManualExecution) GetLogger() *logrus.Entry {
	return mex.logger
}

// Init doesn't do anything...
func (mex ManualExecution) Init(ctx context.Context) error {
	return nil
}

// SetPaused pauses or resumes the scheduler.
func (mex *ManualExecution) SetPaused(paused bool) error {
	select {
	case <-mex.hasStarted:
		event := pauseEvent{isPaused: paused, err: make(chan error)}
		mex.pauseEvents <- event
		return <-event.err
	default:
		return fmt.Errorf("cannot pause the manual scheduler before it has started")
	}
}

// UpdateConfig validates the supplied config and updates it in real time. It is
// possible to update the configuration even when k6 is paused, either in the
// beginning (i.e. when running k6 with --paused) or in the middle of the script
// execution.
func (mex *ManualExecution) UpdateConfig(ctx context.Context, newConf interface{}) error {
	newManualConfig, ok := newConf.(ManualExecutionControlConfig)
	if !ok {
		return errors.New("invalid config type")
	}
	if errs := newManualConfig.Validate(); len(errs) != 0 {
		return fmt.Errorf("invalid confiuguration supplied: %s", lib.ConcatErrors(errs, ", "))
	}

	if newManualConfig.Duration != mex.startConfig.Duration {
		return fmt.Errorf("the manual scheduler duration cannot be changed")
	}
	if newManualConfig.MaxVUs.Int64 < mex.startConfig.MaxVUs.Int64 {
		// This limitation is because the manual execution scheduler is still a
		// scheduler that participates in the overall k6 scheduling. Thus, any
		// VUs that were explicitly specified by the user in the config may be
		// reused from or by other schedulers.
		return fmt.Errorf(
			"the new number of max VUs cannot be lower than the starting number of max VUs (%d)",
			mex.startConfig.MaxVUs.Int64,
		)
	}

	mex.configLock.Lock()
	select {
	case <-mex.hasStarted:
		mex.configLock.Unlock()
		event := updateConfigEvent{newConfig: newManualConfig, err: make(chan error)}
		mex.newControlConfigs <- event
		return <-event.err
	case <-ctx.Done():
		mex.configLock.Unlock()
		return ctx.Err()
	default:
		mex.currentControlConfig = newManualConfig
		mex.configLock.Unlock()
		return nil
	}
}

// This is a helper function that is used in run for non-infinite durations.
func (mex *ManualExecution) stopWhenDurationIsReached(ctx context.Context, duration time.Duration, cancel func()) {
	ctxDone := ctx.Done()
	checkInterval := time.NewTicker(100 * time.Millisecond)
	for {
		select {
		case <-ctxDone:
			checkInterval.Stop()
			return

		//TODO: something more optimized that sleeps for pauses?
		case <-checkInterval.C:
			if mex.executorState.GetCurrentTestRunDuration() >= duration {
				cancel()
				return
			}
		}
	}
}

// manualVUHandle is a wrapper around the vuHandle helper, used in the
// variable-looping-vus scheduler. Here, instead of using its getVU and returnVU
// methods to retrieve and return a VU from the global buffer, we use them to
// accurately update the local and global active VU counters and to ensure that
// the pausing and reducing VUs operations wait for VUs to fully finish
// executing their current iterations before returning.
type manualVUHandle struct {
	*vuHandle
	vu lib.VU
	wg *sync.WaitGroup

	// This is the cancel of the local context, used to kill its goroutine when
	// we reduce the number of MaxVUs, so that the Go GC can clean up the VU.
	cancelVU func()
}

func newManualVUHandle(
	parentCtx context.Context, state *lib.ExecutorState, localActiveVUsCount *int64, vu lib.VU, logger *logrus.Entry,
) *manualVUHandle {

	wg := sync.WaitGroup{}
	getVU := func() (lib.VU, error) {
		wg.Add(1)
		state.ModCurrentlyActiveVUsCount(+1)
		atomic.AddInt64(localActiveVUsCount, +1)
		return vu, nil
	}
	returnVU := func(_ lib.VU) {
		state.ModCurrentlyActiveVUsCount(-1)
		atomic.AddInt64(localActiveVUsCount, -1)
		wg.Done()
	}
	ctx, cancel := context.WithCancel(parentCtx)
	return &manualVUHandle{
		vuHandle: newStoppedVUHandle(ctx, getVU, returnVU, logger),
		vu:       vu,
		wg:       &wg,
		cancelVU: cancel,
	}
}

// Run constantly loops through as many iterations as possible on a variable
// dynamically controlled number of VUs either for the specified duration, or
// until the test is manually stopped.
//
//TODO: split this up? somehow... :/
func (mex *ManualExecution) Run(parentCtx context.Context, out chan<- stats.SampleContainer) (err error) {
	mex.configLock.RLock()
	// Safely get the current config - it's important that the close of the
	// hasStarted channel is inside of the lock, so that there are no data races
	// between it and the UpdateConfig() method.
	currentControlConfig := mex.currentControlConfig
	close(mex.hasStarted)
	mex.configLock.RUnlock()

	segment := mex.executorState.Options.ExecutionSegment
	duration := time.Duration(currentControlConfig.Duration.Duration)

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()
	if duration > 0 { // Only keep track of duration if it's not infinite
		go mex.stopWhenDurationIsReached(ctx, duration, cancel)
	}

	mex.logger.WithFields(
		logrus.Fields{"type": manualExecutionType, "duration": duration},
	).Debug("Starting scheduler run...")

	// Retrieve and initialize the (scaled) number of MaxVUs from the global VU
	// buffer that the user originally specified in the JS config.
	startMaxVUs := segment.Scale(mex.startConfig.MaxVUs.Int64)
	vuHandles := make([]*manualVUHandle, startMaxVUs)
	activeVUsCount := new(int64)
	runIteration := getIterationRunner(mex.executorState, mex.logger, out)
	for i := int64(0); i < startMaxVUs; i++ { // get the initial planned VUs from the common buffer
		vu, vuGetErr := mex.executorState.GetPlannedVU(mex.logger, false)
		if vuGetErr != nil {
			return vuGetErr
		}
		vuHandle := newManualVUHandle(
			parentCtx, mex.executorState, activeVUsCount, vu, mex.logger.WithField("vuNum", i),
		)
		go vuHandle.runLoopsIfPossible(runIteration)
		vuHandles[i] = vuHandle
	}

	// Keep track of the progress
	maxVUs := new(int64)
	*maxVUs = startMaxVUs
	progresFn := func() (float64, string) {
		spent := mex.executorState.GetCurrentTestRunDuration()
		progress := 0.0
		if duration > 0 {
			progress = math.Min(1, float64(spent)/float64(duration))
		}
		//TODO: simulate spinner for the other case or cycle 0-100?
		currentActiveVUs := atomic.LoadInt64(activeVUsCount)
		currentMaxVUs := atomic.LoadInt64(maxVUs)
		vusFmt := pb.GetFixedLengthIntFormat(currentMaxVUs)
		return progress, fmt.Sprintf(
			"currently "+vusFmt+" out of "+vusFmt+" active looping VUs, %s/%s", currentActiveVUs, currentMaxVUs,
			pb.GetFixedLengthDuration(spent, duration), duration,
		)
	}
	mex.progress.Modify(pb.WithProgress(progresFn))
	go trackProgress(parentCtx, ctx, ctx, mex, progresFn)

	currentlyPaused := false
	waitVUs := func(from, to int64) {
		for i := from; i < to; i++ {
			vuHandles[i].wg.Wait()
		}
	}
	handleConfigChange := func(oldControlConfig, newControlConfig ManualExecutionControlConfig) error {
		oldActiveVUs := segment.Scale(oldControlConfig.VUs.Int64)
		oldMaxVUs := segment.Scale(oldControlConfig.MaxVUs.Int64)
		newActiveVUs := segment.Scale(newControlConfig.VUs.Int64)
		newMaxVUs := segment.Scale(newControlConfig.MaxVUs.Int64)

		mex.logger.WithFields(logrus.Fields{
			"oldActiveVUs": oldActiveVUs, "oldMaxVUs": oldMaxVUs,
			"newActiveVUs": newActiveVUs, "newMaxVUs": newMaxVUs,
		}).Debug("Updating execution configuration...")

		for i := oldMaxVUs; i < newMaxVUs; i++ {
			vu, vuInitErr := mex.executorState.InitializeNewVU(ctx, mex.logger)
			if vuInitErr != nil {
				return vuInitErr
			}
			vuHandle := newManualVUHandle(
				ctx, mex.executorState, activeVUsCount, vu, mex.logger.WithField("vuNum", i),
			)
			go vuHandle.runLoopsIfPossible(runIteration)
			vuHandles = append(vuHandles, vuHandle)
		}

		if oldActiveVUs < newActiveVUs {
			for i := oldActiveVUs; i < newActiveVUs; i++ {

				if !currentlyPaused {
					vuHandles[i].start()
				}
			}
		} else {
			for i := newActiveVUs; i < oldActiveVUs; i++ {
				vuHandles[i].hardStop()
			}
			waitVUs(newActiveVUs, oldActiveVUs)
		}

		if oldMaxVUs > newMaxVUs {
			for i := newMaxVUs; i < oldMaxVUs; i++ {
				vuHandles[i].cancelVU()
				if i < startMaxVUs {
					// return the initial planned VUs to the common buffer
					mex.executorState.ReturnVU(vuHandles[i].vu, false)
				} else {
					mex.executorState.ModInitializedVUsCount(-1)
				}
				vuHandles[i] = nil
			}
			vuHandles = vuHandles[:newMaxVUs]
		}

		atomic.StoreInt64(maxVUs, newMaxVUs)
		return nil
	}

	err = handleConfigChange(ManualExecutionControlConfig{MaxVUs: mex.startConfig.MaxVUs}, currentControlConfig)
	if err != nil {
		return err
	}
	defer func() {
		err = handleConfigChange(currentControlConfig, ManualExecutionControlConfig{})
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case updateConfigEvent := <-mex.newControlConfigs:
			err := handleConfigChange(currentControlConfig, updateConfigEvent.newConfig)
			if err != nil {
				updateConfigEvent.err <- err
				return err
			}
			currentControlConfig = updateConfigEvent.newConfig
			mex.configLock.Lock()
			mex.currentControlConfig = updateConfigEvent.newConfig
			mex.configLock.Unlock()
			updateConfigEvent.err <- nil

		case pauseEvent := <-mex.pauseEvents:
			if pauseEvent.isPaused == currentlyPaused {
				pauseEvent.err <- nil
				continue
			}
			activeVUs := currentControlConfig.VUs.Int64
			if pauseEvent.isPaused {
				for i := int64(0); i < activeVUs; i++ {
					vuHandles[i].gracefulStop()
				}
				waitVUs(0, activeVUs)
			} else {
				for i := int64(0); i < activeVUs; i++ {
					vuHandles[i].start()
				}
			}
			currentlyPaused = pauseEvent.isPaused
			pauseEvent.err <- nil
		}
	}
}
