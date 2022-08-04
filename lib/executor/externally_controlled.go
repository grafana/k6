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
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/ui/pb"
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
		errors = append(errors, fmt.Errorf("the number of VUs can't be negative"))
	}

	if mecc.MaxVUs.Int64 < mecc.VUs.Int64 {
		errors = append(errors, fmt.Errorf(
			"the number of active VUs (%d) must be less than or equal to the number of maxVUs (%d)",
			mecc.VUs.Int64, mecc.MaxVUs.Int64,
		))
	}

	if !mecc.Duration.Valid {
		errors = append(errors, fmt.Errorf("the duration must be specified, for infinite duration use 0"))
	} else if mecc.Duration.TimeDuration() < 0 {
		errors = append(errors, fmt.Errorf(
			"the duration can't be negative, for infinite duration use 0",
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
		PlannedVUs:      uint64(et.ScaleInt64(mec.MaxVUs.Int64)), // user-configured, VUs to be pre-initialized
		MaxUnplannedVUs: 0,                                       // intentional, see function comment
	}

	maxDuration := mec.Duration.TimeDuration()
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

		// TODO: something saner and more optimized that sleeps for pauses and
		// doesn't depend on the global execution state?
		case <-checkInterval.C:
			elapsed := mex.executionState.GetCurrentTestRunDuration() - mex.config.StartTime.TimeDuration()
			if elapsed >= duration {
				cancel()
				return
			}
		}
	}
}

// manualVUHandle is a wrapper around the vuHandle helper, used in the
// ramping-vus executor. Here, instead of using its getVU and returnVU
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
		vuHandle: newStoppedVUHandle(ctx, getVU, returnVU,
			rs.executor.nextIterationCounters,
			&rs.executor.config.BaseConfig, logger),
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

	runIteration func(context.Context, lib.ActiveVU) bool // a helper closure function that runs a single iteration
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

func (rs *externallyControlledRunState) progressFn() (float64, []string) {
	// TODO: simulate spinner for the other case or cycle 0-100?
	currentActiveVUs := atomic.LoadInt64(rs.activeVUsCount)
	currentMaxVUs := atomic.LoadInt64(rs.maxVUs)
	vusFmt := pb.GetFixedLengthIntFormat(currentMaxVUs)
	progVUs := fmt.Sprintf(vusFmt+"/"+vusFmt+" VUs", currentActiveVUs, currentMaxVUs)

	right := []string{progVUs, rs.duration.String(), ""}

	// TODO: use a saner way to calculate the elapsed time, without relying on
	// the global execution state...
	elapsed := rs.executor.executionState.GetCurrentTestRunDuration() - rs.executor.config.StartTime.TimeDuration()
	if elapsed > rs.duration {
		return 1, right
	}

	progress := 0.0
	if rs.duration > 0 {
		progress = math.Min(1, float64(elapsed)/float64(rs.duration))
	}

	spentDuration := pb.GetFixedLengthDuration(elapsed, rs.duration)
	progDur := fmt.Sprintf("%s/%s", spentDuration, rs.duration)
	right[1] = progDur

	return progress, right
}

func (rs *externallyControlledRunState) handleConfigChange(oldCfg, newCfg ExternallyControlledConfigParams) error {
	executionState := rs.executor.executionState
	et := executionState.ExecutionTuple
	oldActiveVUs := et.ScaleInt64(oldCfg.VUs.Int64)
	oldMaxVUs := et.ScaleInt64(oldCfg.MaxVUs.Int64)
	newActiveVUs := et.ScaleInt64(newCfg.VUs.Int64)
	newMaxVUs := et.ScaleInt64(newCfg.MaxVUs.Int64)

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
				if err := rs.vuHandles[i].start(); err != nil {
					// TODO: maybe just log it ?
					return err
				}
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
			} else {
				executionState.ModInitializedVUsCount(-1)
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
//nolint:funlen,gocognit
func (mex *ExternallyControlled) Run(parentCtx context.Context, out chan<- metrics.SampleContainer) (err error) {
	mex.configLock.RLock()
	// Safely get the current config - it's important that the close of the
	// hasStarted channel is inside of the lock, so that there are no data races
	// between it and the UpdateConfig() method.
	currentControlConfig := mex.currentControlConfig
	close(mex.hasStarted)
	mex.configLock.RUnlock()

	ctx, cancel := context.WithCancel(parentCtx)
	waitOnProgressChannel := make(chan struct{})
	defer func() {
		cancel()
		<-waitOnProgressChannel
	}()

	duration := currentControlConfig.Duration.TimeDuration()
	if duration > 0 { // Only keep track of duration if it's not infinite
		go mex.stopWhenDurationIsReached(ctx, duration, cancel)
	}

	mex.logger.WithFields(
		logrus.Fields{"type": externallyControlledType, "duration": duration},
	).Debug("Starting executor run...")

	startMaxVUs := mex.executionState.ExecutionTuple.ScaleInt64(mex.config.MaxVUs.Int64)

	ss := &lib.ScenarioState{
		Name:      mex.config.Name,
		Executor:  mex.config.Type,
		StartTime: time.Now(),
	}
	ctx = lib.WithScenarioState(ctx, ss)

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
	ss.ProgressFn = runState.progressFn

	*runState.maxVUs = startMaxVUs
	if err = runState.retrieveStartMaxVUs(); err != nil {
		return err
	}

	mex.progress.Modify(pb.WithProgress(runState.progressFn)) // Keep track of the progress
	go func() {
		trackProgress(parentCtx, ctx, ctx, mex, runState.progressFn)
		close(waitOnProgressChannel)
	}()

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
					if err := runState.vuHandles[i].start(); err != nil {
						// TODO again ... just log it?
						pauseEvent.err <- err
						return err
					}
				}
			}
			runState.currentlyPaused = pauseEvent.isPaused
			pauseEvent.err <- nil
		}
	}
}
