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
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/stats"
	"go.k6.io/k6/ui/pb"
)

const rampingVUsType = "ramping-vus"

func init() {
	lib.RegisterExecutorConfigType(
		rampingVUsType,
		func(name string, rawJSON []byte) (lib.ExecutorConfig, error) {
			config := NewRampingVUsConfig(name)
			err := lib.StrictJSONUnmarshal(rawJSON, &config)
			return config, err
		},
	)
}

// Stage contains
type Stage struct {
	Duration types.NullDuration `json:"duration"`
	Target   null.Int           `json:"target"` // TODO: maybe rename this to endVUs? something else?
	// TODO: add a progression function?
}

// RampingVUsConfig stores the configuration for the stages executor
type RampingVUsConfig struct {
	BaseConfig
	StartVUs         null.Int           `json:"startVUs"`
	Stages           []Stage            `json:"stages"`
	GracefulRampDown types.NullDuration `json:"gracefulRampDown"`
}

// NewRampingVUsConfig returns a RampingVUsConfig with its default values
func NewRampingVUsConfig(name string) RampingVUsConfig {
	return RampingVUsConfig{
		BaseConfig:       NewBaseConfig(name, rampingVUsType),
		StartVUs:         null.NewInt(1, false),
		GracefulRampDown: types.NewNullDuration(30*time.Second, false),
	}
}

// Make sure we implement the lib.ExecutorConfig interface
var _ lib.ExecutorConfig = &RampingVUsConfig{}

// GetStartVUs is just a helper method that returns the scaled starting VUs.
func (vlvc RampingVUsConfig) GetStartVUs(et *lib.ExecutionTuple) int64 {
	return et.ScaleInt64(vlvc.StartVUs.Int64)
}

// GetGracefulRampDown is just a helper method that returns the graceful
// ramp-down period as a standard Go time.Duration value...
func (vlvc RampingVUsConfig) GetGracefulRampDown() time.Duration {
	return time.Duration(vlvc.GracefulRampDown.Duration)
}

// GetDescription returns a human-readable description of the executor options
func (vlvc RampingVUsConfig) GetDescription(et *lib.ExecutionTuple) string {
	maxVUs := et.ScaleInt64(getStagesUnscaledMaxTarget(vlvc.StartVUs.Int64, vlvc.Stages))
	return fmt.Sprintf("Up to %d looping VUs for %s over %d stages%s",
		maxVUs, sumStagesDuration(vlvc.Stages), len(vlvc.Stages),
		vlvc.getBaseInfo(fmt.Sprintf("gracefulRampDown: %s", vlvc.GetGracefulRampDown())))
}

// Validate makes sure all options are configured and valid
func (vlvc RampingVUsConfig) Validate() []error {
	errors := vlvc.BaseConfig.Validate()
	if vlvc.StartVUs.Int64 < 0 {
		errors = append(errors, fmt.Errorf("the number of start VUs shouldn't be negative"))
	}

	if getStagesUnscaledMaxTarget(vlvc.StartVUs.Int64, vlvc.Stages) <= 0 {
		errors = append(errors, fmt.Errorf("either startVUs or one of the stages' target value should be greater than 0"))
	}

	return append(errors, validateStages(vlvc.Stages)...)
}

// getRawExecutionSteps calculates and returns as execution steps the number of
// actively running VUs the executor should have at every moment.
//
// It doesn't take into account graceful ramp-downs. It also doesn't deal with
// the end-of-executor drop to 0 VUs, whether graceful or not. These are
// handled by GetExecutionRequirements(), which internally uses this method and
// reserveVUsForGracefulRampDowns().
//
// The zeroEnd argument tells the method if we should artificially add a step
// with 0 VUs at offset sum(stages.duration), i.e. when the executor is
// supposed to end.
//
// It's also important to note how scaling works. Say, we ramp up from 0 to 10
// VUs over 10 seconds and then back to 0, and we want to split the execution in
// 2 equal segments (i.e. execution segments "0:0.5" and "0.5:1"). The original
// execution steps would look something like this:
//
// VUs  ^
//    10|          *
//     9|         ***
//     8|        *****
//     7|       *******
//     6|      *********
//     5|     ***********
//     4|    *************
//     3|   ***************
//     2|  *****************
//     1| *******************
//     0------------------------> time(s)
//       01234567890123456789012   (t%10)
//       00000000001111111111222   (t/10)
//
// The chart for one of the execution segments would look like this:
//
// VUs  ^
//     5|         XXX
//     4|       XXXXXXX
//     3|     XXXXXXXXXXX
//     2|   XXXXXXXXXXXXXXX
//     1| XXXXXXXXXXXXXXXXXXX
//     0------------------------> time(s)
//       01234567890123456789012   (t%10)
//       00000000001111111111222   (t/10)
//
// And the chart for the other execution segment would look like this:
//
// VUs  ^
//     5|          Y
//     4|        YYYYY
//     3|      YYYYYYYYY
//     2|    YYYYYYYYYYYYY
//     1|  YYYYYYYYYYYYYYYYY
//     0------------------------> time(s)
//       01234567890123456789012   (t%10)
//       00000000001111111111222   (t/10)
//
// Notice the time offsets and the slower ramping up and down. All of that is
// because the sum of the two execution segments has to produce exactly the
// original shape, as if the test ran on a single machine:
//
// VUs  ^
//    10|          Y
//     9|         XXX
//     8|        YYYYY
//     7|       XXXXXXX
//     6|      YYYYYYYYY
//     5|     XXXXXXXXXXX
//     4|    YYYYYYYYYYYYY
//     3|   XXXXXXXXXXXXXXX
//     2|  YYYYYYYYYYYYYYYYY
//     1| XXXXXXXXXXXXXXXXXXX
//     0------------------------> time(s)
//       01234567890123456789012   (t%10)
//       00000000001111111111222   (t/10)
//
// More information: https://github.com/k6io/k6/issues/997#issuecomment-484416866
func (vlvc RampingVUsConfig) getRawExecutionSteps(et *lib.ExecutionTuple, zeroEnd bool) []lib.ExecutionStep {
	var (
		timeTillEnd time.Duration
		fromVUs     = vlvc.StartVUs.Int64
		steps       = make([]lib.ExecutionStep, 0, vlvc.precalculateTheRequiredSteps(et, zeroEnd))
		index       = lib.NewSegmentedIndex(et)
	)

	// Reserve the scaled StartVUs at the beginning
	scaled, unscaled := index.GoTo(fromVUs)
	steps = append(steps, lib.ExecutionStep{TimeOffset: 0, PlannedVUs: uint64(scaled)})
	addStep := func(timeOffset time.Duration, plannedVUs uint64) {
		if steps[len(steps)-1].PlannedVUs != plannedVUs {
			steps = append(steps, lib.ExecutionStep{TimeOffset: timeOffset, PlannedVUs: plannedVUs})
		}
	}

	for _, stage := range vlvc.Stages {
		stageEndVUs := stage.Target.Int64
		stageDuration := time.Duration(stage.Duration.Duration)
		timeTillEnd += stageDuration

		stageVUDiff := stageEndVUs - fromVUs
		if stageVUDiff == 0 {
			continue
		}
		if stageDuration == 0 {
			scaled, unscaled = index.GoTo(stageEndVUs)
			addStep(timeTillEnd, uint64(scaled))
			fromVUs = stageEndVUs
			continue
		}

		// VU reservation for gracefully ramping down is handled as a
		// separate method: reserveVUsForGracefulRampDowns()
		if unscaled > stageEndVUs { // ramp down
			// here we don't want to emit for the equal to stageEndVUs as it doesn't go below it
			// it will just go to it
			for ; unscaled > stageEndVUs; scaled, unscaled = index.Prev() {
				addStep(
					// this is the time that we should go up 1 if we are ramping up
					// but we are ramping down so we should go 1 down, but because we want to not
					// stop VUs immediately we stop it on the next unscaled VU's time
					timeTillEnd-time.Duration(int64(stageDuration)*(stageEndVUs-unscaled+1)/stageVUDiff),
					uint64(scaled-1),
				)
			}
		} else {
			for ; unscaled <= stageEndVUs; scaled, unscaled = index.Next() {
				addStep(
					timeTillEnd-time.Duration(int64(stageDuration)*(stageEndVUs-unscaled)/stageVUDiff),
					uint64(scaled),
				)
			}
		}
		fromVUs = stageEndVUs
	}

	if zeroEnd && steps[len(steps)-1].PlannedVUs != 0 {
		// If the last PlannedVUs value wasn't 0, add a last step with 0
		steps = append(steps, lib.ExecutionStep{TimeOffset: timeTillEnd, PlannedVUs: 0})
	}
	return steps
}

func absInt64(a int64) int64 {
	if a < 0 {
		return -a
	}
	return a
}

func (vlvc RampingVUsConfig) precalculateTheRequiredSteps(et *lib.ExecutionTuple, zeroEnd bool) int {
	p := et.ScaleInt64(vlvc.StartVUs.Int64)
	var result int64
	result++ // for the first one

	if zeroEnd {
		result++ // for the last one - this one can be more then needed
	}
	for _, stage := range vlvc.Stages {
		stageEndVUs := et.ScaleInt64(stage.Target.Int64)
		if stage.Duration.Duration == 0 {
			result++
		} else {
			result += absInt64(p - stageEndVUs)
		}
		p = stageEndVUs
	}
	return int(result)
}

// If the graceful ramp-downs are enabled, we need to reserve any VUs that may
// potentially have to finish running iterations when we're scaling their number
// down. This would prevent attempts from other executors to use them while the
// iterations are finishing up during their allotted gracefulRampDown periods.
//
// But we also need to be careful to not over-allocate more VUs than we actually
// need. We should never have more PlannedVUs than the max(startVUs,
// stage[n].target), even if we're quickly scaling VUs up and down multiple
// times, one after the other. In those cases, any previously reserved VUs
// finishing up interrupted iterations should be reused by the executor,
// instead of new ones being requested from the execution state.
//
// Here's an example with graceful ramp-down (i.e. "uninterruptible"
// iterations), where stars represent actively scheduled VUs and dots are used
// for VUs that are potentially finishing up iterations:
//
//
//      ^
//      |
// VUs 6|  *..............................
//     5| ***.......*..............................
//     4|*****.....***.....**..............................
//     3|******...*****...***..............................
//     2|*******.*******.****..............................
//     1|***********************..............................
//     0--------------------------------------------------------> time(s)
//       012345678901234567890123456789012345678901234567890123   (t%10)
//       000000000011111111112222222222333333333344444444445555   (t/10)
//
// We start with 4 VUs, scale to 6, scale down to 1, scale up to 5, scale down
// to 1 again, scale up to 4, back to 1, and finally back down to 0. If our
// gracefulStop timeout was 30s (the default), then we'll stay with 6 PlannedVUs
// until t=32 in the test above, and the actual executor could run until t=52.
// See TestRampingVUsConfigExecutionPlanExample() for the above example
// as a unit test.
//
// The algorithm we use below to reserve VUs so that ramping-down VUs can finish
// their last iterations is pretty simple. It just traverses the raw execution
// steps and whenever there's a scaling down of VUs, it prevents the number of
// VUs from decreasing for the configured gracefulRampDown period.
//
// Finishing up the test, i.e. making sure we have a step with 0 VUs at time
// executorEndOffset, is not handled here. Instead GetExecutionRequirements()
// takes care of that. But to make its job easier, this method won't add any
// steps with an offset that's greater or equal to executorEndOffset.
func (vlvc RampingVUsConfig) reserveVUsForGracefulRampDowns( //nolint:funlen
	rawSteps []lib.ExecutionStep, executorEndOffset time.Duration,
) []lib.ExecutionStep {
	rawStepsLen := len(rawSteps)
	gracefulRampDownPeriod := vlvc.GetGracefulRampDown()
	newSteps := []lib.ExecutionStep{}

	lastPlannedVUs := uint64(0)
	lastDownwardSlopeStepNum := 0
	for rawStepNum := 0; rawStepNum < rawStepsLen; rawStepNum++ {
		rawStep := rawSteps[rawStepNum]
		// Add the first step or any step where the number of planned VUs is
		// greater than the ones in the previous step. We don't need to worry
		// about reserving time for ramping-down VUs when the number of planned
		// VUs is growing. That's because the gracefulRampDown period is a fixed
		// value and any timeouts from early steps with fewer VUs will get
		// overshadowed by timeouts from latter steps with more VUs.
		if rawStepNum == 0 || rawStep.PlannedVUs > lastPlannedVUs {
			newSteps = append(newSteps, rawStep)
			lastPlannedVUs = rawStep.PlannedVUs
			continue
		}

		// We simply skip steps with the same number of planned VUs
		if rawStep.PlannedVUs == lastPlannedVUs {
			continue
		}

		// If we're here, we have a downward "slope" - the lastPlannedVUs are
		// more than the current rawStep's planned VUs. We're going to look
		// forward in time (up to gracefulRampDown) and inspect the rawSteps.
		// There are a 3 possibilities:
		//  - We find a new step within the gracefulRampDown period which has
		//    the same number of VUs or greater than lastPlannedVUs. Which
		//    means that we can just advance rawStepNum to that number and we
		//    don't need to worry about any of the raw steps in the middle!
		//    Both their planned VUs and their gracefulRampDown periods will
		//    be lower than what we're going to set from that new rawStep -
		//    we've basically found a new upward slope or equal value again.
		//  - We reach executorEndOffset, in which case we are done - we can't
		//    add any new steps, since those will be after the executor end
		//    offset.
		//  - We reach the end of the rawSteps, or we don't find any higher or
		//    equal steps to prevStep in the next gracefulRampDown period. So
		//    we'll simply add a new entry into newSteps with the values
		//    {timeOffsetWithTimeout, rawStep.PlannedVUs}, in which
		//    timeOffsetWithTimeout = (prevStep.TimeOffset + gracefulRampDown),
		//    after which we'll continue with traversing the following rawSteps.
		//
		//  In this last case, we can also keep track of the downward slope. We
		//  can save the last index we've seen, to optimize future lookaheads by
		//  starting them from the furhest downward slope step index we've seen so
		//  far. This can be done because the gracefulRampDown is constant, so
		//  when we're on a downward slope, raw steps will always fall in the
		//  gracefulRampDown "shadow" of their preceding steps on the slope.

		skippedToNewRawStep := false
		timeOffsetWithTimeout := rawStep.TimeOffset + gracefulRampDownPeriod

		advStepStart := rawStepNum + 1
		if lastDownwardSlopeStepNum > advStepStart {
			advStepStart = lastDownwardSlopeStepNum
		}

		wasRampingDown := true
		for advStepNum := advStepStart; advStepNum < rawStepsLen; advStepNum++ {
			advStep := rawSteps[advStepNum]
			if advStep.TimeOffset > timeOffsetWithTimeout {
				break
			}
			if advStep.PlannedVUs >= lastPlannedVUs {
				rawStepNum = advStepNum - 1
				skippedToNewRawStep = true
				break
			}
			if wasRampingDown {
				if rawSteps[advStepNum-1].PlannedVUs > advStep.PlannedVUs {
					// Still ramping down
					lastDownwardSlopeStepNum = advStepNum
				} else {
					// No longer ramping down
					wasRampingDown = false
				}
			}
		}

		// Nothing more to do here, found a new "slope" with equal or grater
		// PlannedVUs in the gracefulRampDownPeriod window, so we go to it.
		if skippedToNewRawStep {
			continue
		}

		// We've reached the absolute executor end offset, and we were already
		// on a downward "slope" (i.e. the previous planned VUs are more than
		// the current planned VUs), so nothing more we can do here.
		if timeOffsetWithTimeout >= executorEndOffset {
			break
		}

		newSteps = append(newSteps, lib.ExecutionStep{
			TimeOffset: timeOffsetWithTimeout,
			PlannedVUs: rawStep.PlannedVUs,
		})
		lastPlannedVUs = rawStep.PlannedVUs
	}

	return newSteps
}

// GetExecutionRequirements very dynamically reserves exactly the number of
// required VUs for this executor at every moment of the test.
//
// If gracefulRampDown is specified, it will also be taken into account, and the
// number of needed VUs to handle that will also be reserved. See the
// documentation of reserveVUsForGracefulRampDowns() for more details.
//
// On the other hand, gracefulStop is handled here. To facilitate it, we'll
// ensure that the last execution step will have 0 VUs and will be at time
// offset (sum(stages.Duration)+gracefulStop). Any steps that would've been
// added after it will be ignored. Thus:
//   - gracefulStop can be less than gracefulRampDown and can cut the graceful
//     ramp-down periods of the last VUs short.
//   - gracefulRampDown can be more than gracefulStop:
//     - If the user manually ramped down VUs at the end of the test (i.e. the
//       last stage's target is 0), then this will have no effect.
//     - If the last stage's target is more than 0, the VUs at the end of the
//       executor's life will have more time to finish their last iterations.
func (vlvc RampingVUsConfig) GetExecutionRequirements(et *lib.ExecutionTuple) []lib.ExecutionStep {
	steps := vlvc.getRawExecutionSteps(et, false)

	executorEndOffset := sumStagesDuration(vlvc.Stages) + time.Duration(vlvc.GracefulStop.Duration)
	// Handle graceful ramp-downs, if we have them
	if vlvc.GracefulRampDown.Duration > 0 {
		steps = vlvc.reserveVUsForGracefulRampDowns(steps, executorEndOffset)
	}

	// If the last PlannedVUs value wasn't 0, add a last step with 0
	if steps[len(steps)-1].PlannedVUs != 0 {
		steps = append(steps, lib.ExecutionStep{TimeOffset: executorEndOffset, PlannedVUs: 0})
	}

	return steps
}

// NewExecutor creates a new RampingVUs executor
func (vlvc RampingVUsConfig) NewExecutor(es *lib.ExecutionState, logger *logrus.Entry) (lib.Executor, error) {
	return RampingVUs{
		BaseExecutor: NewBaseExecutor(vlvc, es, logger),
		config:       vlvc,
	}, nil
}

// HasWork reports whether there is any work to be done for the given execution segment.
func (vlvc RampingVUsConfig) HasWork(et *lib.ExecutionTuple) bool {
	return lib.GetMaxPlannedVUs(vlvc.GetExecutionRequirements(et)) > 0
}

// RampingVUs handles the old "stages" execution configuration - it loops
// iterations with a variable number of VUs for the sum of all of the specified
// stages' duration.
type RampingVUs struct {
	*BaseExecutor
	config RampingVUsConfig
}

// Make sure we implement the lib.Executor interface.
var _ lib.Executor = &RampingVUs{}

// Run constantly loops through as many iterations as possible on a variable
// number of VUs for the specified stages.
//
// TODO: split up? since this does a ton of things, unfortunately I can't think
// of a less complex way to implement it (besides the old "increment by 100ms
// and see what happens)... :/ so maybe see how it can be split?
// nolint:funlen,gocognit,cyclop
func (vlv RampingVUs) Run(
	parentCtx context.Context, out chan<- stats.SampleContainer, _ *metrics.BuiltinMetrics,
) (err error) {
	rawExecutionSteps := vlv.config.getRawExecutionSteps(vlv.executionState.ExecutionTuple, true)
	regularDuration, isFinal := lib.GetEndOffset(rawExecutionSteps)
	if !isFinal {
		return fmt.Errorf("%s expected raw end offset at %s to be final", vlv.config.GetName(), regularDuration)
	}

	gracefulExecutionSteps := vlv.config.GetExecutionRequirements(vlv.executionState.ExecutionTuple)
	maxDuration, isFinal := lib.GetEndOffset(gracefulExecutionSteps)
	if !isFinal {
		return fmt.Errorf("%s expected graceful end offset at %s to be final", vlv.config.GetName(), maxDuration)
	}
	maxVUs := lib.GetMaxPlannedVUs(gracefulExecutionSteps)
	gracefulStop := maxDuration - regularDuration

	startTime, maxDurationCtx, regDurationCtx, cancel := getDurationContexts(parentCtx, regularDuration, gracefulStop)
	defer cancel()

	activeVUs := &sync.WaitGroup{}
	defer activeVUs.Wait()

	// Make sure the log and the progress bar have accurate information
	vlv.logger.WithFields(logrus.Fields{
		"type": vlv.config.GetType(), "startVUs": vlv.config.GetStartVUs(vlv.executionState.ExecutionTuple), "maxVUs": maxVUs,
		"duration": regularDuration, "numStages": len(vlv.config.Stages),
	},
	).Debug("Starting executor run...")

	activeVUsCount := new(int64)
	vusFmt := pb.GetFixedLengthIntFormat(int64(maxVUs))
	regularDurationStr := pb.GetFixedLengthDuration(regularDuration, regularDuration)
	progressFn := func() (float64, []string) {
		spent := time.Since(startTime)
		currentlyActiveVUs := atomic.LoadInt64(activeVUsCount)
		vus := fmt.Sprintf(vusFmt+"/"+vusFmt+" VUs", currentlyActiveVUs, maxVUs)
		if spent > regularDuration {
			return 1, []string{vus, regularDuration.String()}
		}
		progVUs := fmt.Sprintf(vusFmt+"/"+vusFmt+" VUs", currentlyActiveVUs, maxVUs)
		progDur := pb.GetFixedLengthDuration(spent, regularDuration) + "/" + regularDurationStr
		return float64(spent) / float64(regularDuration), []string{progVUs, progDur}
	}
	vlv.progress.Modify(pb.WithProgress(progressFn))
	go trackProgress(parentCtx, maxDurationCtx, regDurationCtx, vlv, progressFn)

	// Actually schedule the VUs and iterations, likely the most complicated
	// executor among all of them...
	runIteration := getIterationRunner(vlv.executionState, vlv.logger)
	getVU := func() (lib.InitializedVU, error) {
		initVU, err := vlv.executionState.GetPlannedVU(vlv.logger, false)
		if err != nil {
			vlv.logger.WithError(err).Error("Cannot get a VU from the buffer")
			cancel()
		} else {
			activeVUs.Add(1)
			atomic.AddInt64(activeVUsCount, 1)
			vlv.executionState.ModCurrentlyActiveVUsCount(+1)
		}
		return initVU, err
	}
	returnVU := func(initVU lib.InitializedVU) {
		vlv.executionState.ReturnVU(initVU, false)
		atomic.AddInt64(activeVUsCount, -1)
		activeVUs.Done()
		vlv.executionState.ModCurrentlyActiveVUsCount(-1)
	}

	maxDurationCtx = lib.WithScenarioState(maxDurationCtx, &lib.ScenarioState{
		Name:       vlv.config.Name,
		Executor:   vlv.config.Type,
		StartTime:  startTime,
		ProgressFn: progressFn,
	})

	vuHandles := make([]*vuHandle, maxVUs)
	for i := uint64(0); i < maxVUs; i++ {
		vuHandle := newStoppedVUHandle(
			maxDurationCtx, getVU, returnVU, vlv.nextIterationCounters,
			&vlv.config.BaseConfig, vlv.logger.WithField("vuNum", i))
		go vuHandle.runLoopsIfPossible(runIteration)
		vuHandles[i] = vuHandle
	}

	// 0 <= currentScheduledVUs <= currentMaxAllowedVUs <= maxVUs
	var currentScheduledVUs, currentMaxAllowedVUs uint64

	handleNewScheduledVUs := func(newScheduledVUs uint64) {
		if newScheduledVUs > currentScheduledVUs {
			for vuNum := currentScheduledVUs; vuNum < newScheduledVUs; vuNum++ {
				_ = vuHandles[vuNum].start() // TODO handle error
			}
		} else {
			for vuNum := newScheduledVUs; vuNum < currentScheduledVUs; vuNum++ {
				vuHandles[vuNum].gracefulStop()
			}
		}
		currentScheduledVUs = newScheduledVUs
	}

	handleNewMaxAllowedVUs := func(newMaxAllowedVUs uint64) {
		if newMaxAllowedVUs < currentMaxAllowedVUs {
			for vuNum := newMaxAllowedVUs; vuNum < currentMaxAllowedVUs; vuNum++ {
				vuHandles[vuNum].hardStop()
			}
		}
		currentMaxAllowedVUs = newMaxAllowedVUs
	}

	wait := waiter(parentCtx, startTime)
	// iterate over rawExecutionSteps and gracefulExecutionSteps in order by TimeOffset
	// giving rawExecutionSteps precedence.
	// we stop iterating once rawExecutionSteps are over as we need to run the remaining
	// gracefulExecutionSteps concurrently while waiting for VUs to stop in order to not wait until
	// the end of gracefulStop timeouts
	i, j := 0, 0
	for i != len(rawExecutionSteps) {
		if rawExecutionSteps[i].TimeOffset > gracefulExecutionSteps[j].TimeOffset {
			if wait(gracefulExecutionSteps[j].TimeOffset) {
				return
			}
			handleNewMaxAllowedVUs(gracefulExecutionSteps[j].PlannedVUs)
			j++
		} else {
			if wait(rawExecutionSteps[i].TimeOffset) {
				return
			}
			handleNewScheduledVUs(rawExecutionSteps[i].PlannedVUs)
			i++
		}
	}

	go func() { // iterate over the remaining gracefulExecutionSteps
		for _, step := range gracefulExecutionSteps[j:] {
			if wait(step.TimeOffset) {
				return
			}
			handleNewMaxAllowedVUs(step.PlannedVUs)
		}
	}()

	return nil
}

// waiter returns a function that will sleep/wait for the required time since the startTime and then
// return. If the context was done before that it will return true otherwise it will return false
// TODO use elsewhere
// TODO set startTime here?
// TODO move it to a struct type or something and benchmark if that makes a difference
func waiter(ctx context.Context, startTime time.Time) func(offset time.Duration) bool {
	timer := time.NewTimer(time.Hour * 24)
	return func(offset time.Duration) bool {
		offsetDiff := offset - time.Since(startTime)
		if offsetDiff > 0 { // wait until time of event arrives // TODO have a mininum
			timer.Reset(offsetDiff)
			select {
			case <-ctx.Done():
				return true // exit if context is cancelled
			case <-timer.C:
				// now we do a step
			}
		}
		return false
	}
}
