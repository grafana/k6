package execution

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/ui/pb"
)

// A Scheduler is in charge of most of the test execution - initializing VUs and
// executors, running setup() and teardown(), and actually starting the
// executors for the different scenarios at the appropriate times.
type Scheduler struct {
	initProgress    *pb.ProgressBar
	executorConfigs []lib.ExecutorConfig // sorted by (startTime, ID)
	executors       []lib.Executor       // sorted by (startTime, ID), excludes executors with no work
	executionPlan   []lib.ExecutionStep
	maxDuration     time.Duration // cached value derived from the execution plan
	maxPossibleVUs  uint64        // cached value derived from the execution plan
	state           *lib.ExecutionState
}

// NewScheduler creates and returns a new Scheduler instance, without
// initializing it beyond the bare minimum. Specifically, it creates the needed
// executor instances and a lot of state placeholders, but it doesn't initialize
// the executors and it doesn't initialize or run VUs.
func NewScheduler(trs *lib.TestRunState) (*Scheduler, error) {
	options := trs.Options
	et, err := lib.NewExecutionTuple(options.ExecutionSegment, options.ExecutionSegmentSequence)
	if err != nil {
		return nil, err
	}
	executionPlan := options.Scenarios.GetFullExecutionRequirements(et)
	maxPlannedVUs := lib.GetMaxPlannedVUs(executionPlan)
	maxPossibleVUs := lib.GetMaxPossibleVUs(executionPlan)

	executionState := lib.NewExecutionState(trs, et, maxPlannedVUs, maxPossibleVUs)
	maxDuration, _ := lib.GetEndOffset(executionPlan) // we don't care if the end offset is final

	executorConfigs := options.Scenarios.GetSortedConfigs()
	executors := make([]lib.Executor, 0, len(executorConfigs))
	// Only take executors which have work.
	for _, sc := range executorConfigs {
		if !sc.HasWork(et) {
			trs.Logger.Warnf(
				"Executor '%s' is disabled for segment %s due to lack of work!",
				sc.GetName(), options.ExecutionSegment,
			)
			continue
		}
		s, err := sc.NewExecutor(executionState, trs.Logger.WithFields(logrus.Fields{
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

	return &Scheduler{
		initProgress:    pb.New(pb.WithConstLeft("Init")),
		executors:       executors,
		executorConfigs: executorConfigs,
		executionPlan:   executionPlan,
		maxDuration:     maxDuration,
		maxPossibleVUs:  maxPossibleVUs,
		state:           executionState,
	}, nil
}

// GetState returns a pointer to the execution state struct for the execution
// scheduler. It's guaranteed to be initialized and present, though see the
// documentation in lib/execution.go for caveats about its usage. The most
// important one is that none of the methods beyond the pause-related ones
// should be used for synchronization.
func (e *Scheduler) GetState() *lib.ExecutionState {
	return e.state
}

// GetExecutors returns the slice of configured executor instances which
// have work, sorted by their (startTime, name) in an ascending order.
func (e *Scheduler) GetExecutors() []lib.Executor {
	return e.executors
}

// GetExecutorConfigs returns the slice of all executor configs, sorted by
// their (startTime, name) in an ascending order.
func (e *Scheduler) GetExecutorConfigs() []lib.ExecutorConfig {
	return e.executorConfigs
}

// GetInitProgressBar returns the progress bar associated with the Init
// function. After the Init is done, it is "hijacked" to display real-time
// execution statistics as a text bar.
func (e *Scheduler) GetInitProgressBar() *pb.ProgressBar {
	return e.initProgress
}

// GetExecutionPlan is a helper method so users of the local execution scheduler
// don't have to calculate the execution plan again.
func (e *Scheduler) GetExecutionPlan() []lib.ExecutionStep {
	return e.executionPlan
}

// initVU is a helper method that's used to both initialize the planned VUs
// in the Init() method, and also passed to executors so they can initialize
// any unplanned VUs themselves.
func (e *Scheduler) initVU(
	ctx context.Context, samplesOut chan<- metrics.SampleContainer, logger logrus.FieldLogger,
) (lib.InitializedVU, error) {
	// Get the VU IDs here, so that the VUs are (mostly) ordered by their
	// number in the channel buffer
	vuIDLocal, vuIDGlobal := e.state.GetUniqueVUIdentifiers()
	vu, err := e.state.Test.Runner.NewVU(ctx, vuIDLocal, vuIDGlobal, samplesOut)
	if err != nil {
		return nil, errext.WithHint(err, fmt.Sprintf("error while initializing VU #%d", vuIDGlobal))
	}

	logger.Debugf("Initialized VU #%d", vuIDGlobal)
	return vu, nil
}

// getRunStats is a helper function that can be used as the execution
// scheduler's progressbar substitute (i.e. hijack).
func (e *Scheduler) getRunStats() string {
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

func (e *Scheduler) initVUsConcurrently(
	ctx context.Context, samplesOut chan<- metrics.SampleContainer, count uint64,
	concurrency int, logger logrus.FieldLogger,
) chan error {
	doneInits := make(chan error, count) // poor man's waitgroup with results
	limiter := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		go func() {
			for range limiter {
				newVU, err := e.initVU(ctx, samplesOut, logger)
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
				for skipVu := vuNum; skipVu < count; skipVu++ {
					// do not even start initializing the remaining VUs
					doneInits <- ctx.Err()
				}
				return
			}
		}
	}()

	return doneInits
}

func (e *Scheduler) emitVUsAndVUsMax(ctx context.Context, out chan<- metrics.SampleContainer) func() {
	e.state.Test.Logger.Debug("Starting emission of VUs and VUsMax metrics...")
	tags := e.state.Test.RunTags
	wg := &sync.WaitGroup{}
	wg.Add(1)

	emitMetrics := func() {
		t := time.Now()
		samples := metrics.ConnectedSamples{
			Samples: []metrics.Sample{
				{
					TimeSeries: metrics.TimeSeries{
						Metric: e.state.Test.BuiltinMetrics.VUs,
						Tags:   tags,
					},
					Time:  t,
					Value: float64(e.state.GetCurrentlyActiveVUsCount()),
				}, {
					TimeSeries: metrics.TimeSeries{
						Metric: e.state.Test.BuiltinMetrics.VUsMax,
						Tags:   tags,
					},
					Time:  t,
					Value: float64(e.state.GetInitializedVUsCount()),
				},
			},
			Tags: tags,
			Time: t,
		}
		metrics.PushIfNotDone(ctx, out, samples)
	}

	ticker := time.NewTicker(1 * time.Second)
	go func() {
		defer func() {
			ticker.Stop()
			e.state.Test.Logger.Debug("Metrics emission of VUs and VUsMax metrics stopped")
			wg.Done()
		}()

		for {
			select {
			case <-ticker.C:
				emitMetrics()
			case <-ctx.Done():
				return
			}
		}
	}()

	return wg.Wait
}

// initVUsAndExecutors concurrently initializes all of the planned VUs and then
// sequentially initializes all of the configured executors.
func (e *Scheduler) initVUsAndExecutors(ctx context.Context, samplesOut chan<- metrics.SampleContainer) (err error) {
	e.initProgress.Modify(pb.WithConstProgress(0, "Init VUs..."))

	logger := e.state.Test.Logger.WithField("phase", "execution-scheduler-init")
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

	var initErr error
	for vuNum := uint64(0); vuNum < vusToInitialize; vuNum++ {
		var err error
		select {
		case err = <-doneInits:
			if err == nil {
				atomic.AddUint64(initializedVUs, 1)
			}
		case <-ctx.Done():
			err = ctx.Err()
		}

		if err == nil || initErr != nil {
			// No error or a previous init error was already saved and we are
			// just waiting for VUs to finish aborting
			continue
		}

		logger.WithError(err).Debug("VU initialization returned with an error, aborting...")
		initErr = err
		cancel()
	}

	if initErr != nil {
		return initErr
	}

	e.state.SetInitVUFunc(func(ctx context.Context, logger *logrus.Entry) (lib.InitializedVU, error) {
		return e.initVU(ctx, samplesOut, logger)
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
func (e *Scheduler) runExecutor(
	runCtx context.Context, runResults chan<- error, engineOut chan<- metrics.SampleContainer, executor lib.Executor,
) {
	executorConfig := executor.GetConfig()
	executorStartTime := executorConfig.GetStartTime()
	executorLogger := e.state.Test.Logger.WithFields(logrus.Fields{
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
	err := executor.Run(runCtx, engineOut) // executor should handle context cancel itself
	if err == nil {
		executorLogger.Debugf("Executor finished successfully")
	} else {
		executorLogger.WithField("error", err).Errorf("Executor error")
	}
	runResults <- err
}

// Init concurrently initializes all of the planned VUs and then sequentially
// initializes all of the configured executors. It also starts the measurement
// and emission of the `vus` and `vus_max` metrics.
func (e *Scheduler) Init(
	runCtx context.Context, samplesOut chan<- metrics.SampleContainer,
) (stopVUEmission func(), initErr error) {
	logger := e.state.Test.Logger.WithField("phase", "execution-scheduler-init")

	execSchedRunCtx, execSchedRunCancel := context.WithCancel(runCtx)
	waitForVUsMetricPush := e.emitVUsAndVUsMax(execSchedRunCtx, samplesOut)
	stopVUEmission = func() {
		logger.Debugf("Stopping vus and vux_max metrics emission...")
		execSchedRunCancel()
		waitForVUsMetricPush()
	}

	defer func() {
		if interruptErr := GetCancelReasonIfTestAborted(runCtx); interruptErr != nil {
			logger.Debugf("The test run was interrupted, returning '%s' instead of '%s'", interruptErr, initErr)
			e.state.SetExecutionStatus(lib.ExecutionStatusInterrupted)
			initErr = interruptErr
		}
		if initErr != nil {
			stopVUEmission()
		}
	}()

	return stopVUEmission, e.initVUsAndExecutors(execSchedRunCtx, samplesOut)
}

// Run the Scheduler, funneling all generated metric samples through the supplied
// out channel.
//
//nolint:funlen
func (e *Scheduler) Run(globalCtx, runCtx context.Context, samplesOut chan<- metrics.SampleContainer) (runErr error) {
	logger := e.state.Test.Logger.WithField("phase", "execution-scheduler-run")

	defer func() {
		if interruptErr := GetCancelReasonIfTestAborted(runCtx); interruptErr != nil {
			logger.Debugf("The test run was interrupted, returning '%s' instead of '%s'", interruptErr, runErr)
			e.state.SetExecutionStatus(lib.ExecutionStatusInterrupted)
			runErr = interruptErr
		}
	}()

	e.initProgress.Modify(pb.WithConstLeft("Run"))
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

	e.initProgress.Modify(pb.WithConstProgress(1, "Starting test..."))
	e.state.MarkStarted()
	defer e.state.MarkEnded()
	e.initProgress.Modify(pb.WithConstProgress(1, "running"))

	executorsCount := len(e.executors)
	logger.WithFields(logrus.Fields{"executorsCount": executorsCount}).Debugf("Start of test run")

	runResults := make(chan error, executorsCount) // nil values are successful runs

	// TODO: get rid of this context, pass the e.state directly to VUs when they
	// are initialized by e.initVUsAndExecutors(). This will also give access to
	// its properties in their init context executions.
	withExecStateCtx := lib.WithExecutionState(runCtx, e.state)

	// Run setup() before any executors, if it's not disabled
	if !e.state.Test.Options.NoSetup.Bool {
		e.state.SetExecutionStatus(lib.ExecutionStatusSetup)
		e.initProgress.Modify(pb.WithConstProgress(1, "setup()"))
		if err := e.state.Test.Runner.Setup(withExecStateCtx, samplesOut); err != nil {
			logger.WithField("error", err).Debug("setup() aborted by error")
			return err
		}
	}
	e.initProgress.Modify(pb.WithHijack(e.getRunStats))

	// Start all executors at their particular startTime in a separate goroutine...
	logger.Debug("Start all executors...")
	e.state.SetExecutionStatus(lib.ExecutionStatusRunning)

	executorsRunCtx, executorsRunCancel := context.WithCancel(withExecStateCtx)
	defer executorsRunCancel()
	for _, exec := range e.executors {
		go e.runExecutor(executorsRunCtx, runResults, samplesOut, exec)
	}

	// Wait for all executors to finish
	var firstErr error
	for range e.executors {
		err := <-runResults
		if err != nil && firstErr == nil {
			logger.WithError(err).Debug("Executor returned with an error, cancelling test run...")
			firstErr = err
			executorsRunCancel()
		}
	}

	// Run teardown() after all executors are done, if it's not disabled
	if !e.state.Test.Options.NoTeardown.Bool {
		e.state.SetExecutionStatus(lib.ExecutionStatusTeardown)
		e.initProgress.Modify(pb.WithConstProgress(1, "teardown()"))

		// We run teardown() with the global context, so it isn't interrupted by
		// thresholds or test.abort() or even Ctrl+C (unless used twice).
		if err := e.state.Test.Runner.Teardown(globalCtx, samplesOut); err != nil {
			logger.WithField("error", err).Debug("teardown() aborted by error")
			return err
		}
	}

	return firstErr
}

// SetPaused pauses the test, or start/resumes it. To check if a test is paused,
// use GetState().IsPaused().
//
// Currently, any executor, so any test, can be started in a paused state. This
// will cause k6 to initialize all needed VUs, but it won't actually start the
// test. Later, the test can be started for real by resuming/unpausing it from
// the REST API.
//
// After a test is actually started, it may become impossible to pause it again.
// That is signaled by having SetPaused(true) return an error. The likely cause
// is that some of the executors for the test don't support pausing after the
// test has been started.
//
// IMPORTANT: Currently only the externally controlled executor can be paused
// and resumed multiple times in the middle of the test execution! Even then,
// "pausing" is a bit misleading, since k6 won't pause in the middle of the
// currently executing iterations. It will allow the currently in-progress
// iterations to finish, and it just won't start any new ones nor will it
// increment the value returned by GetCurrentTestRunDuration().
func (e *Scheduler) SetPaused(pause bool) error {
	if !e.state.HasStarted() && e.state.IsPaused() {
		if pause {
			return fmt.Errorf("execution is already paused")
		}
		e.state.Test.Logger.Debug("Starting execution")
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
