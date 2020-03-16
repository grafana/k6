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
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	null "gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/ui/pb"
)

const variableLoopingVUsType = "variable-looping-vus"

// How often we can make VU adjustments when processing stages
// TODO: make configurable, in some bounds?
const minIntervalBetweenVUAdjustments = 100 * time.Millisecond

func init() {
	lib.RegisterExecutorConfigType(
		variableLoopingVUsType,
		func(name string, rawJSON []byte) (lib.ExecutorConfig, error) {
			config := NewVariableLoopingVUsConfig(name)
			err := lib.StrictJSONUnmarshal(rawJSON, &config)
			return config, err
		},
	)
}

// Stage contains
type Stage struct {
	Duration types.NullDuration `json:"duration"`
	Target   null.Int           `json:"target"` // TODO: maybe rename this to endVUs? something else?
	//TODO: add a progression function?
}

// VariableLoopingVUsConfig stores the configuration for the stages executor
type VariableLoopingVUsConfig struct {
	BaseConfig
	StartVUs         null.Int           `json:"startVUs"`
	Stages           []Stage            `json:"stages"`
	GracefulRampDown types.NullDuration `json:"gracefulRampDown"`
}

// NewVariableLoopingVUsConfig returns a VariableLoopingVUsConfig with its default values
func NewVariableLoopingVUsConfig(name string) VariableLoopingVUsConfig {
	return VariableLoopingVUsConfig{
		BaseConfig:       NewBaseConfig(name, variableLoopingVUsType),
		StartVUs:         null.NewInt(1, false),
		GracefulRampDown: types.NewNullDuration(30*time.Second, false),
	}
}

// Make sure we implement the lib.ExecutorConfig interface
var _ lib.ExecutorConfig = &VariableLoopingVUsConfig{}

// GetStartVUs is just a helper method that returns the scaled starting VUs.
func (vlvc VariableLoopingVUsConfig) GetStartVUs(et *lib.ExecutionTuple) int64 {
	return et.ES.Scale(vlvc.StartVUs.Int64)
}

// GetGracefulRampDown is just a helper method that returns the graceful
// ramp-down period as a standard Go time.Duration value...
func (vlvc VariableLoopingVUsConfig) GetGracefulRampDown() time.Duration {
	return time.Duration(vlvc.GracefulRampDown.Duration)
}

// GetDescription returns a human-readable description of the executor options
func (vlvc VariableLoopingVUsConfig) GetDescription(et *lib.ExecutionTuple) string {
	maxVUs := et.ES.Scale(getStagesUnscaledMaxTarget(vlvc.StartVUs.Int64, vlvc.Stages))
	return fmt.Sprintf("Up to %d looping VUs for %s over %d stages%s",
		maxVUs, sumStagesDuration(vlvc.Stages), len(vlvc.Stages),
		vlvc.getBaseInfo(fmt.Sprintf("gracefulRampDown: %s", vlvc.GetGracefulRampDown())))
}

// Validate makes sure all options are configured and valid
func (vlvc VariableLoopingVUsConfig) Validate() []error {
	errors := vlvc.BaseConfig.Validate()
	if vlvc.StartVUs.Int64 < 0 {
		errors = append(errors, fmt.Errorf("the number of start VUs shouldn't be negative"))
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
//     9|         YYY
//     8|        YYYYY
//     7|       YYYYYYY
//     6|      YYYYYYYYY
//     5|     YYYYXXXYYYY
//     4|    YYYXXXXXXXYYY
//     3|   YYXXXXXXXXXXXYY
//     2|  YXXXXXXXXXXXXXXXY
//     1| XXXXXXXXXXXXXXXXXXX
//     0------------------------> time(s)
//       01234567890123456789012   (t%10)
//       00000000001111111111222   (t/10)
//
// More information: https://github.com/loadimpact/k6/issues/997#issuecomment-484416866
//nolint:funlen
func (vlvc VariableLoopingVUsConfig) getRawExecutionSteps(es *lib.ExecutionSegment, zeroEnd bool) []lib.ExecutionStep {
	// For accurate results, calculations are done with the unscaled values, and
	// the values are scaled only before we add them to the steps result slice
	fromVUs := vlvc.StartVUs.Int64

	abs := func(n int64) int64 { // sigh...
		if n < 0 {
			return -n
		}
		return n
	}

	// Reserve the scaled StartVUs at the beginning
	prevScaledVUs := es.Scale(vlvc.StartVUs.Int64)
	steps := []lib.ExecutionStep{{TimeOffset: 0, PlannedVUs: uint64(prevScaledVUs)}}
	timeFromStart := time.Duration(0)
	totalDuration := time.Duration(0)

	for _, stage := range vlvc.Stages {
		stageEndVUs := stage.Target.Int64
		stageDuration := time.Duration(stage.Duration.Duration)
		totalDuration += stageDuration

		stageVUAbsDiff := abs(stageEndVUs - fromVUs)
		if stageVUAbsDiff == 0 {
			// We don't have to do anything but update the time offset
			// if the number of VUs wasn't changed in this stage
			timeFromStart += stageDuration
			continue
		}

		// Handle 0-duration stages, i.e. instant VU jumps
		if stageDuration == 0 {
			fromVUs = stageEndVUs
			prevScaledVUs = es.Scale(stageEndVUs)
			steps = append(steps, lib.ExecutionStep{
				TimeOffset: timeFromStart,
				PlannedVUs: uint64(prevScaledVUs),
			})
			continue
		}

		// For each stage, limit any VU adjustments between the previous
		// number of VUs and the stage's target to happen at most once
		// every minIntervalBetweenVUAdjustments. No floats or ratios,
		// since nanoseconds should be good enough for anyone... :)
		stepInterval := stageDuration / time.Duration(stageVUAbsDiff)
		if stepInterval < minIntervalBetweenVUAdjustments {
			stepInterval = minIntervalBetweenVUAdjustments
		}

		// Loop through the potential steps, adding an item to the
		// result only when there's a change in the number of VUs.
		//
		// IMPORTANT: we have to be very careful of rounding errors,
		// both from the step duration and from the VUs. It's especially
		// important that the scaling via the execution segment should
		// happen AFTER the rest of the calculations have been done and
		// we've rounded the global "global" number of VUs.
		for t := stepInterval; ; t += stepInterval { // Skip the first step, since we've already added that
			if time.Duration(abs(int64(stageDuration-t))) < minIntervalBetweenVUAdjustments {
				// Skip the last step of the stage, add it below to correct any minor clock skew
				break
			}
			stepGlobalVUs := fromVUs + int64(
				math.Round((float64(t)*float64(stageEndVUs-fromVUs))/float64(stageDuration)),
			)
			stepScaledVus := es.Scale(stepGlobalVUs)

			if stepScaledVus == prevScaledVUs {
				// only add steps when there's a change in the number of VUs
				continue
			}

			// VU reservation for gracefully ramping down is handled as a
			// separate method: reserveVUsForGracefulRampDowns()

			steps = append(steps, lib.ExecutionStep{
				TimeOffset: timeFromStart + t,
				PlannedVUs: uint64(stepScaledVus),
			})
			prevScaledVUs = stepScaledVus
		}

		fromVUs = stageEndVUs
		prevScaledVUs = es.Scale(stageEndVUs)
		timeFromStart += stageDuration
		steps = append(steps, lib.ExecutionStep{
			TimeOffset: timeFromStart,
			PlannedVUs: uint64(prevScaledVUs),
		})
	}

	if zeroEnd && steps[len(steps)-1].PlannedVUs != 0 {
		// If the last PlannedVUs value wasn't 0, add a last step with 0
		steps = append(steps, lib.ExecutionStep{TimeOffset: totalDuration, PlannedVUs: 0})
	}
	return steps
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
// See TestVariableLoopingVUsConfigExecutionPlanExample() for the above example
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
func (vlvc VariableLoopingVUsConfig) reserveVUsForGracefulRampDowns( //nolint:funlen
	rawSteps []lib.ExecutionStep, executorEndOffset time.Duration,
) []lib.ExecutionStep {
	rawStepsLen := len(rawSteps)
	gracefulRampDownPeriod := vlvc.GetGracefulRampDown()
	newSteps := []lib.ExecutionStep{}

	lastPlannedVUs := uint64(0)
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
		//    we'll simply try to add an entry into newSteps with the values
		//    {prevStep.TimeOffset + gracefulRampDown, rawStep.PlannedVUs} and
		//    we'll continue with traversing the following rawSteps.

		skippedToNewRawStep := false
		timeOffsetWithTimeout := rawStep.TimeOffset + gracefulRampDownPeriod

		for advStepNum := rawStepNum + 1; advStepNum < rawStepsLen; advStepNum++ {
			advStep := rawSteps[advStepNum]
			if advStep.TimeOffset > timeOffsetWithTimeout {
				break
			}
			if advStep.PlannedVUs >= lastPlannedVUs {
				rawStepNum = advStepNum - 1
				skippedToNewRawStep = true
				break
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
func (vlvc VariableLoopingVUsConfig) GetExecutionRequirements(et *lib.ExecutionTuple) []lib.ExecutionStep {
	steps := vlvc.getRawExecutionSteps(et.ES, false)

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

// NewExecutor creates a new VariableLoopingVUs executor
func (vlvc VariableLoopingVUsConfig) NewExecutor(es *lib.ExecutionState, logger *logrus.Entry) (lib.Executor, error) {
	return VariableLoopingVUs{
		BaseExecutor: NewBaseExecutor(vlvc, es, logger),
		config:       vlvc,
	}, nil
}

// HasWork reports whether there is any work to be done for the given execution segment.
func (vlvc VariableLoopingVUsConfig) HasWork(et *lib.ExecutionTuple) bool {
	return lib.GetMaxPlannedVUs(vlvc.GetExecutionRequirements(et)) > 0
}

// VariableLoopingVUs handles the old "stages" execution configuration - it
// loops iterations with a variable number of VUs for the sum of all of the
// specified stages' duration.
type VariableLoopingVUs struct {
	*BaseExecutor
	config VariableLoopingVUsConfig
}

// Make sure we implement the lib.Executor interface.
var _ lib.Executor = &VariableLoopingVUs{}

// Run constantly loops through as many iterations as possible on a variable
// number of VUs for the specified stages.
//
// TODO: split up? since this does a ton of things, unfortunately I can't think
// of a less complex way to implement it (besides the old "increment by 100ms
// and see what happens)... :/ so maybe see how it can be split?
// nolint:funlen,gocognit
func (vlv VariableLoopingVUs) Run(ctx context.Context, out chan<- stats.SampleContainer) (err error) {
	segment := vlv.executionState.Options.ExecutionSegment
	rawExecutionSteps := vlv.config.getRawExecutionSteps(segment, true)
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

	startTime, maxDurationCtx, regDurationCtx, cancel := getDurationContexts(ctx, regularDuration, gracefulStop)
	defer cancel()

	// Make sure the log and the progress bar have accurate information
	vlv.logger.WithFields(logrus.Fields{
		"type": vlv.config.GetType(), "startVUs": vlv.config.GetStartVUs(vlv.executionState.ExecutionTuple), "maxVUs": maxVUs,
		"duration": regularDuration, "numStages": len(vlv.config.Stages)},
	).Debug("Starting executor run...")

	activeVUsCount := new(int64)
	vusFmt := pb.GetFixedLengthIntFormat(int64(maxVUs))
	progresFn := func() (float64, []string) {
		spent := time.Since(startTime)
		currentlyActiveVUs := atomic.LoadInt64(activeVUsCount)
		vus := fmt.Sprintf(vusFmt+"/"+vusFmt+" VUs", currentlyActiveVUs, maxVUs)
		if spent > regularDuration {
			return 1, []string{vus, regularDuration.String()}
		}
		progVUs := fmt.Sprintf(vusFmt+"/"+vusFmt+" VUs", currentlyActiveVUs, maxVUs)
		progDur := fmt.Sprintf("%s/%s", pb.GetFixedLengthDuration(spent, regularDuration), regularDuration)
		return float64(spent) / float64(regularDuration), []string{progVUs, progDur}
	}
	vlv.progress.Modify(pb.WithProgress(progresFn))
	go trackProgress(ctx, maxDurationCtx, regDurationCtx, vlv, progresFn)

	// Actually schedule the VUs and iterations, likely the most complicated
	// executor among all of them...
	activeVUs := &sync.WaitGroup{}
	defer activeVUs.Wait()

	runIteration := getIterationRunner(vlv.executionState, vlv.logger)
	getVU := func() (lib.InitializedVU, error) {
		initVU, err := vlv.executionState.GetPlannedVU(vlv.logger, true)
		if err != nil {
			cancel()
			return nil, err
		}
		activeVUs.Add(1)
		atomic.AddInt64(activeVUsCount, 1)
		return initVU, nil
	}
	returnVU := func(initVU lib.InitializedVU) {
		activeVUs.Done()
		atomic.AddInt64(activeVUsCount, -1)
		vlv.executionState.ReturnVU(initVU, true)
	}

	vuHandles := make([]*vuHandle, maxVUs)
	for i := uint64(0); i < maxVUs; i++ {
		vuHandle := newStoppedVUHandle(maxDurationCtx, getVU, returnVU, vlv.logger.WithField("vuNum", i))
		go vuHandle.runLoopsIfPossible(runIteration)
		vuHandles[i] = vuHandle
	}

	rawStepEvents := lib.StreamExecutionSteps(ctx, startTime, rawExecutionSteps, true)
	gracefulLimitEvents := lib.StreamExecutionSteps(ctx, startTime, gracefulExecutionSteps, false)

	// 0 <= currentScheduledVUs <= currentMaxAllowedVUs <= maxVUs
	var currentScheduledVUs, currentMaxAllowedVUs uint64

	handleNewScheduledVUs := func(newScheduledVUs uint64) {
		if newScheduledVUs > currentScheduledVUs {
			for vuNum := currentScheduledVUs; vuNum < newScheduledVUs; vuNum++ {
				vuHandles[vuNum].start()
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

	handleAllRawSteps := func() bool {
		for {
			select {
			case step, ok := <-rawStepEvents:
				if !ok {
					return true
				}
				handleNewScheduledVUs(step.PlannedVUs)
			case step := <-gracefulLimitEvents:
				if step.PlannedVUs > currentMaxAllowedVUs {
					// Handle the case where a value is read from the
					// gracefulLimitEvents channel before rawStepEvents
					handleNewScheduledVUs(step.PlannedVUs)
				}
				handleNewMaxAllowedVUs(step.PlannedVUs)
			case <-ctx.Done():
				return false
			}
		}
	}

	if handleAllRawSteps() {
		// Handle any remaining graceful stops
		go func() {
			for {
				select {
				case step := <-gracefulLimitEvents:
					handleNewMaxAllowedVUs(step.PlannedVUs)
				case <-maxDurationCtx.Done():
					return
				}
			}
		}()
	}

	return nil
}
