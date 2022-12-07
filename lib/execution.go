package lib

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// MaxTimeToWaitForPlannedVU specifies the maximum allowable time for an executor
// to wait for a planned VU to be retrieved from the ExecutionState.PlannedVUs
// buffer. If it's exceeded, k6 will emit a warning log message, since it either
// means that there's a bug in the k6 scheduling code, or that the machine is
// overloaded and the scheduling code suffers from delays.
//
// Critically, exceeding this time *doesn't* result in an aborted test or any
// test errors, and the executor will continue to try and borrow the VU
// (potentially resulting in further warnings). We likely should emit a k6
// metric about it in the future. TODO: emit a metric every time this is
// exceeded?
const MaxTimeToWaitForPlannedVU = 400 * time.Millisecond

// MaxRetriesGetPlannedVU how many times we should wait for
// MaxTimeToWaitForPlannedVU before we actually return an error.
const MaxRetriesGetPlannedVU = 5

// ExecutionStatus is used to mark the possible states of a test run at any
// given time in its execution, from its start to its finish.
//
//go:generate enumer -type=ExecutionStatus -trimprefix ExecutionStatus -output execution_status_gen.go
type ExecutionStatus uint32

// Possible execution status values
const (
	ExecutionStatusCreated ExecutionStatus = iota
	ExecutionStatusInitVUs
	ExecutionStatusInitExecutors
	ExecutionStatusInitDone
	ExecutionStatusPausedBeforeRun
	ExecutionStatusStarted
	ExecutionStatusSetup
	ExecutionStatusRunning
	ExecutionStatusTeardown
	ExecutionStatusEnded
	ExecutionStatusInterrupted
)

// ExecutionState contains a few different things:
//   - Some convenience items, that are needed by all executors, like the
//     execution segment and the unique VU ID generator. By keeping those here,
//     we can just pass the ExecutionState to the different executors, instead of
//     individually passing them each item.
//   - Mutable counters that different executors modify and other parts of
//     k6 can read, e.g. for the vus and vus_max metrics k6 emits every second.
//   - Pausing controls and statistics.
//
// The counters and timestamps here are primarily meant to be used for
// information extraction and avoidance of ID collisions. Using many of the
// counters here for synchronization between VUs could result in HIDDEN data
// races, because the Go data race detector can't detect any data races
// involving atomics...
//
// The only functionality intended for synchronization is the one revolving
// around pausing, and uninitializedUnplannedVUs for restricting the number of
// unplanned VUs being initialized.
type ExecutionState struct {
	// A portal to the broader test run state, so the different executors have
	// access to the test options, built-in metrics, etc.. They will need to
	// access things like the current execution segment, the per-run metrics
	// tags, different metrics to emit, etc.
	//
	// Obviously, things here are not meant to be changed... They should be a
	// constant during the execution of a single test, but we can't easily
	// enforce that via the Go type system...
	Test *TestRunState

	ExecutionTuple *ExecutionTuple // TODO Rename, possibly move

	// vus is the shared channel buffer that contains all of the VUs that have
	// been initialized and aren't currently being used by a executor.
	//
	// It contains both pre-initialized (i.e. planned) VUs, as well as any
	// unplanned VUs. Planned VUs are initialized before a test begins, while
	// unplanned VUS can be initialized in the middle of the test run by a
	// executor and have been relinquished after it has finished working with
	// them. Usually, unplanned VUs are initialized by one of the arrival-rate
	// executors, after they have exhausted their PreAllocatedVUs. After the
	// executor is done with the VUs, it will put in this channel, so it could
	// potentially be reused by other executors further along in the test.
	//
	// Different executors cooperatively borrow VUs from here when they are
	// needed and return them when they are done with them. There's no central
	// enforcement of correctness, i.e. that a executor takes more VUs from
	// here than its execution plan has stipulated. The correctness guarantee
	// lies with the actual executors - bugs in one can affect others.
	//
	// That's why the field is private and we force executors to use the
	// GetPlannedVU(), GetUnplannedVU(), and ReturnVU() methods instead of work
	// directly with the channel. These methods will emit a warning or can even
	// return an error if retrieving a VU takes more than
	// MaxTimeToWaitForPlannedVU.
	vus chan InitializedVU

	// The segmented index used to generate unique local (current k6 instance)
	// and global (across k6 instances) VU IDs, starting from 1
	// (for backwards compatibility...).
	vuIDSegIndexMx *sync.Mutex
	vuIDSegIndex   *SegmentedIndex

	// TODO: add something similar, but for iterations? Currently, there isn't
	// a straightforward way to get a unique sequential identifier per iteration
	// in the context of a single k6 instance. Combining __VU and __ITER gives us
	// a unique identifier, but it's unwieldy and somewhat cumbersome.

	// Total number of currently initialized VUs. Generally equal to
	// the VU ID minus 1, since initializedVUs starts from 0 and is
	// incremented only after a VU is initialized, while the VU ID is
	// incremented before a VU is initialized. It should always be greater than
	// or equal to 0, but int64 is used for simplification of the used atomic
	// arithmetic operations.
	initializedVUs *int64

	// Total number of unplanned VUs we haven't initialized yet. It starts
	// being equal to GetMaxPossibleVUs(executionPlan)-GetMaxPlannedVUs(), and
	// may stay that way if no unplanned VUs are initialized. Once it reaches 0,
	// no more unplanned VUs can be initialized.
	uninitializedUnplannedVUs *int64

	// Injected when the execution scheduler's Init function is called, used for
	// initializing unplanned VUs.
	initVUFunc InitVUFunc

	// The number of VUs that are currently executing the test script. This also
	// includes any VUs that are in the process of gracefully winding down,
	// either at the end of the test, or when VUs are ramping down. It should
	// always be greater than or equal to 0, but int64 is used for
	// simplification of the used atomic arithmetic operations.
	activeVUs *int64

	// The total number of full (i.e uninterrupted) iterations that have been
	// completed so far.
	fullIterationsCount *uint64

	// The total number of iterations that have been interrupted during their
	// execution. The potential interruption causes vary - end of a specified
	// script `duration`, scaling down of VUs via `stages`, a user hitting
	// Ctrl+C, change of `vus` via the externally controlled executor's REST
	// API, etc.
	interruptedIterationsCount *uint64

	// A machine-readable indicator in which the current state of the test
	// execution is currently stored. Useful for the REST API and external
	// observability of the k6 test run progress.
	executionStatus *uint32

	// A nanosecond UNIX timestamp that is set when the test is actually
	// started. The default 0 value is used to denote that the test hasn't
	// started yet...
	startTime *int64

	// A nanosecond UNIX timestamp that is set when the test ends, either
	// by an early context cancel or at its regularly scheduled time.
	// The default 0 value is used to denote that the test hasn't ended yet.
	endTime *int64

	// Stuff related to pausing follows. Read the docs in ExecutionScheduler for
	// more information regarding how pausing works in k6.
	//
	// When we pause the execution in the middle of the test, we save the
	// current timestamp in currentPauseTime. When we resume the execution, we
	// set currentPauseTime back to 0 and we add the (time.Now() -
	// currentPauseTime) duration to totalPausedDuration (unless the test hasn't
	// started yet).
	//
	// Thus, the algorithm for GetCurrentTestRunDuration() is very
	// straightforward:
	//   - if the test hasn't started, return 0
	//   - set endTime to:
	//      - the current pauseTime, if not zero
	//      - time.Now() otherwise
	//   - return (endTime - startTime - totalPausedDuration)
	//
	// Quickly checking for IsPaused() just means comparing the currentPauseTime
	// with 0, a single atomic operation.
	//
	// But if we want to wait until a script resumes, or be notified of the
	// start/resume event from a channel (as part of a select{}), we have to
	// acquire the pauseStateLock, get the current resumeNotify instance,
	// release the lock and wait to read from resumeNotify (when it's closed by
	// Resume()).
	currentPauseTime    *int64
	pauseStateLock      sync.RWMutex
	totalPausedDuration time.Duration // only modified behind the lock
	resumeNotify        chan struct{}
}

// NewExecutionState initializes all of the pointers in the ExecutionState
// with zeros. It also makes sure that the initial state is unpaused, by
// setting resumeNotify to an already closed channel.
func NewExecutionState(
	testRunState *TestRunState, et *ExecutionTuple, maxPlannedVUs, maxPossibleVUs uint64,
) *ExecutionState {
	resumeNotify := make(chan struct{})
	close(resumeNotify) // By default the ExecutionState starts unpaused

	maxUnplannedUninitializedVUs := int64(maxPossibleVUs - maxPlannedVUs)

	segIdx := NewSegmentedIndex(et)
	return &ExecutionState{
		Test:           testRunState,
		ExecutionTuple: et,

		vus: make(chan InitializedVU, maxPossibleVUs),

		executionStatus:            new(uint32),
		vuIDSegIndexMx:             new(sync.Mutex),
		vuIDSegIndex:               segIdx,
		initializedVUs:             new(int64),
		uninitializedUnplannedVUs:  &maxUnplannedUninitializedVUs,
		activeVUs:                  new(int64),
		fullIterationsCount:        new(uint64),
		interruptedIterationsCount: new(uint64),
		startTime:                  new(int64),
		endTime:                    new(int64),
		currentPauseTime:           new(int64),
		pauseStateLock:             sync.RWMutex{},
		totalPausedDuration:        0, // Accessed only behind the pauseStateLock
		resumeNotify:               resumeNotify,
	}
}

// GetUniqueVUIdentifiers returns the next unique VU IDs, both local (for the
// current instance, exposed as __VU) and global (across k6 instances, exposed
// in the k6/execution module). It starts from 1, for backwards compatibility.
func (es *ExecutionState) GetUniqueVUIdentifiers() (uint64, uint64) {
	es.vuIDSegIndexMx.Lock()
	defer es.vuIDSegIndexMx.Unlock()
	scaled, unscaled := es.vuIDSegIndex.Next()
	return uint64(scaled), uint64(unscaled)
}

// GetInitializedVUsCount returns the total number of currently initialized VUs.
//
// Important: this doesn't include any temporary/service VUs that are destroyed
// after they are used. These are created for the initial retrieval of the
// exported script options and for the execution of setup() and teardown()
//
// IMPORTANT: for UI/information purposes only, don't use for synchronization.
func (es *ExecutionState) GetInitializedVUsCount() int64 {
	return atomic.LoadInt64(es.initializedVUs)
}

// ModInitializedVUsCount changes the total number of currently initialized VUs.
//
// IMPORTANT: for UI/information purposes only, don't use for synchronization.
func (es *ExecutionState) ModInitializedVUsCount(mod int64) int64 {
	return atomic.AddInt64(es.initializedVUs, mod)
}

// GetCurrentlyActiveVUsCount returns the number of VUs that are currently
// executing the test script. This also includes any VUs that are in the process
// of gracefully winding down.
//
// IMPORTANT: for UI/information purposes only, don't use for synchronization.
func (es *ExecutionState) GetCurrentlyActiveVUsCount() int64 {
	return atomic.LoadInt64(es.activeVUs)
}

// ModCurrentlyActiveVUsCount changes the total number of currently active VUs.
//
// IMPORTANT: for UI/information purposes only, don't use for synchronization.
func (es *ExecutionState) ModCurrentlyActiveVUsCount(mod int64) int64 {
	return atomic.AddInt64(es.activeVUs, mod)
}

// GetFullIterationCount returns the total of full (i.e uninterrupted) iterations
// that have been completed so far.
//
// IMPORTANT: for UI/information purposes only, don't use for synchronization.
func (es *ExecutionState) GetFullIterationCount() uint64 {
	return atomic.LoadUint64(es.fullIterationsCount)
}

// AddFullIterations increments the number of full (i.e uninterrupted) iterations
// by the provided amount.
//
// IMPORTANT: for UI/information purposes only, don't use for synchronization.
func (es *ExecutionState) AddFullIterations(count uint64) uint64 {
	return atomic.AddUint64(es.fullIterationsCount, count)
}

// GetPartialIterationCount returns the total of partial (i.e interrupted)
// iterations that have been completed so far.
//
// IMPORTANT: for UI/information purposes only, don't use for synchronization.
func (es *ExecutionState) GetPartialIterationCount() uint64 {
	return atomic.LoadUint64(es.interruptedIterationsCount)
}

// AddInterruptedIterations increments the number of partial (i.e interrupted)
// iterations by the provided amount.
//
// IMPORTANT: for UI/information purposes only, don't use for synchronization.
func (es *ExecutionState) AddInterruptedIterations(count uint64) uint64 {
	return atomic.AddUint64(es.interruptedIterationsCount, count)
}

// SetExecutionStatus changes the current execution status to the supplied value
// and returns the current value.
func (es *ExecutionState) SetExecutionStatus(newStatus ExecutionStatus) (oldStatus ExecutionStatus) {
	return ExecutionStatus(atomic.SwapUint32(es.executionStatus, uint32(newStatus)))
}

// GetCurrentExecutionStatus returns the current execution status. Don't use
// this for synchronization unless you've made the k6 behavior somewhat
// predictable with options like --paused or --linger.
func (es *ExecutionState) GetCurrentExecutionStatus() ExecutionStatus {
	return ExecutionStatus(atomic.LoadUint32(es.executionStatus))
}

// MarkStarted saves the current timestamp as the test start time.
//
// CAUTION: Calling MarkStarted() a second time for the same execution state will
// result in a panic!
func (es *ExecutionState) MarkStarted() {
	if !atomic.CompareAndSwapInt64(es.startTime, 0, time.Now().UnixNano()) {
		panic("the execution scheduler was started a second time")
	}
	es.SetExecutionStatus(ExecutionStatusStarted)
}

// MarkEnded saves the current timestamp as the test end time.
//
// CAUTION: Calling MarkEnded() a second time for the same execution state will
// result in a panic!
func (es *ExecutionState) MarkEnded() {
	if !atomic.CompareAndSwapInt64(es.endTime, 0, time.Now().UnixNano()) {
		panic("the execution scheduler was stopped a second time")
	}
	es.SetExecutionStatus(ExecutionStatusEnded)
}

// HasStarted returns true if the test has actually started executing.
// It will return false while a test is in the init phase, or if it has
// been initially paused. But if will return true if a test is paused
// midway through its execution (see above for details regarding the
// feasibility of that pausing for normal executors).
func (es *ExecutionState) HasStarted() bool {
	return atomic.LoadInt64(es.startTime) != 0
}

// HasEnded returns true if the test has finished executing. It will return
// false until MarkEnded() is called.
func (es *ExecutionState) HasEnded() bool {
	return atomic.LoadInt64(es.endTime) != 0
}

// IsPaused quickly returns whether the test is currently paused, by reading
// the atomic currentPauseTime timestamp
func (es *ExecutionState) IsPaused() bool {
	return atomic.LoadInt64(es.currentPauseTime) != 0
}

// GetCurrentTestRunDuration returns the duration for which the test has already
// ran. If the test hasn't started yet, that's 0. If it has started, but has
// been paused midway through, it will return the time up until the pause time.
// And if it's currently running, it will return the time since the start time.
//
// IMPORTANT: for UI/information purposes only, don't use for synchronization.
func (es *ExecutionState) GetCurrentTestRunDuration() time.Duration {
	startTime := atomic.LoadInt64(es.startTime)
	if startTime == 0 {
		// The test hasn't started yet
		return 0
	}

	es.pauseStateLock.RLock()
	endTime := atomic.LoadInt64(es.endTime)
	pausedDuration := es.totalPausedDuration
	es.pauseStateLock.RUnlock()

	if endTime == 0 {
		pauseTime := atomic.LoadInt64(es.currentPauseTime)
		if pauseTime != 0 {
			endTime = pauseTime
		} else {
			// The test isn't paused or finished, use the current time instead
			endTime = time.Now().UnixNano()
		}
	}

	return time.Duration(endTime-startTime) - pausedDuration
}

// Pause pauses the current execution. It acquires the lock, writes
// the current timestamp in currentPauseTime, and makes a new
// channel for resumeNotify.
// Pause can return an error if the test was already paused.
func (es *ExecutionState) Pause() error {
	es.pauseStateLock.Lock()
	defer es.pauseStateLock.Unlock()

	if !atomic.CompareAndSwapInt64(es.currentPauseTime, 0, time.Now().UnixNano()) {
		return errors.New("test execution was already paused")
	}
	es.resumeNotify = make(chan struct{})
	return nil
}

// Resume unpauses the test execution. Unless the test wasn't
// yet started, it calculates the duration between now and
// the old currentPauseTime and adds it to
// Resume will emit an error if the test wasn't paused.
func (es *ExecutionState) Resume() error {
	es.pauseStateLock.Lock()
	defer es.pauseStateLock.Unlock()

	currentPausedTime := atomic.SwapInt64(es.currentPauseTime, 0)
	if currentPausedTime == 0 {
		return errors.New("test execution wasn't paused")
	}

	// Check that it's not the pause before execution actually starts
	if atomic.LoadInt64(es.startTime) != 0 {
		es.totalPausedDuration += time.Duration(time.Now().UnixNano() - currentPausedTime)
	}

	close(es.resumeNotify)

	return nil
}

// ResumeNotify returns a channel which will be closed (i.e. could
// be read from) as soon as the test execution is resumed.
//
// Since tests would likely be paused only rarely, unless you
// directly need to be notified via a channel that the test
// isn't paused or that it has resumed, it's probably a good
// idea to first use the IsPaused() method, since it will be much
// faster.
//
// And, since tests won't be paused most of the time, it's
// probably better to check for that like this:
//
//	if executionState.IsPaused() {
//	    <-executionState.ResumeNotify()
//	}
func (es *ExecutionState) ResumeNotify() <-chan struct{} {
	es.pauseStateLock.RLock()
	defer es.pauseStateLock.RUnlock()
	return es.resumeNotify
}

// GetPlannedVU tries to get a pre-initialized VU from the buffer channel. This
// shouldn't fail and should generally be an instantaneous action, but if it
// doesn't happen for MaxTimeToWaitForPlannedVU (for example, because the system
// is overloaded), a warning will be printed. If we reach that timeout more than
// MaxRetriesGetPlannedVU number of times, this function will return an error,
// since we either have a bug with some executor, or the machine is very, very
// overloaded.
//
// If modifyActiveVUCount is true, the method would also increment the counter
// for active VUs. In most cases, that's the desired behavior, but some
// executors might have to retrieve their reserved VUs without using them
// immediately - for example, the externally-controlled executor when the
// configured maxVUs number is greater than the configured starting VUs.
func (es *ExecutionState) GetPlannedVU(logger *logrus.Entry, modifyActiveVUCount bool) (InitializedVU, error) {
	for i := 1; i <= MaxRetriesGetPlannedVU; i++ {
		select {
		case vu := <-es.vus:
			if modifyActiveVUCount {
				es.ModCurrentlyActiveVUsCount(+1)
			}
			// TODO: set environment and exec
			return vu, nil
		case <-time.After(MaxTimeToWaitForPlannedVU):
			logger.Warnf("Could not get a VU from the buffer for %s", time.Duration(i)*MaxTimeToWaitForPlannedVU)
		}
	}
	return nil, fmt.Errorf(
		"could not get a VU from the buffer in %s",
		MaxRetriesGetPlannedVU*MaxTimeToWaitForPlannedVU,
	)
}

// SetInitVUFunc is called by the execution scheduler's init function, and it's
// used for setting the "constructor" function used for the initializing
// unplanned VUs.
//
// TODO: figure out a better dependency injection method?
func (es *ExecutionState) SetInitVUFunc(initVUFunc InitVUFunc) {
	es.initVUFunc = initVUFunc
}

// GetUnplannedVU checks if any unplanned VUs remain to be initialized, and if
// they do, it initializes one and returns it. If all unplanned VUs have already
// been initialized, it returns one from the global vus buffer, but doesn't
// automatically increment the active VUs counter in either case.
//
// IMPORTANT: GetUnplannedVU() doesn't do any checking if the requesting
// executor is actually allowed to have the VU at this particular time.
// Executors are trusted to correctly declare their needs (via their
// GetExecutionRequirements() methods) and then to never ask for more VUs than
// they have specified in those requirements.
func (es *ExecutionState) GetUnplannedVU(ctx context.Context, logger *logrus.Entry) (InitializedVU, error) {
	remVUs := atomic.AddInt64(es.uninitializedUnplannedVUs, -1)
	if remVUs < 0 {
		logger.Debug("Reusing a previously initialized unplanned VU")
		atomic.AddInt64(es.uninitializedUnplannedVUs, 1)
		return es.GetPlannedVU(logger, false)
	}

	logger.Debug("Initializing an unplanned VU, this may affect test results")
	return es.InitializeNewVU(ctx, logger)
}

// InitializeNewVU creates and returns a brand new VU, updating the relevant
// tracking counters.
func (es *ExecutionState) InitializeNewVU(ctx context.Context, logger *logrus.Entry) (InitializedVU, error) {
	if es.initVUFunc == nil {
		return nil, fmt.Errorf("initVUFunc wasn't set in the execution state")
	}
	newVU, err := es.initVUFunc(ctx, logger)
	if err != nil {
		return nil, err
	}
	es.ModInitializedVUsCount(+1)
	return newVU, err
}

// AddInitializedVU is a helper function that adds VUs into the buffer and
// increases the initialized VUs counter.
func (es *ExecutionState) AddInitializedVU(vu InitializedVU) {
	es.vus <- vu
	es.ModInitializedVUsCount(+1)
}

// ReturnVU is a helper function that puts VUs back into the buffer and
// decreases the active VUs counter.
func (es *ExecutionState) ReturnVU(vu InitializedVU, wasActive bool) {
	es.vus <- vu
	if wasActive {
		es.ModCurrentlyActiveVUsCount(-1)
	}
}
