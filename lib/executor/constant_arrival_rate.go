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
	"math/big"
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

const constantArrivalRateType = "constant-arrival-rate"

func init() {
	lib.RegisterExecutorConfigType(
		constantArrivalRateType,
		func(name string, rawJSON []byte) (lib.ExecutorConfig, error) {
			config := NewConstantArrivalRateConfig(name)
			err := lib.StrictJSONUnmarshal(rawJSON, &config)
			return config, err
		},
	)
}

// ConstantArrivalRateConfig stores config for the constant arrival-rate executor
type ConstantArrivalRateConfig struct {
	BaseConfig
	Rate     null.Int           `json:"rate"`
	TimeUnit types.NullDuration `json:"timeUnit"`
	Duration types.NullDuration `json:"duration"`

	// Initialize `PreAllocatedVUs` number of VUs, and if more than that are needed,
	// they will be dynamically allocated, until `MaxVUs` is reached, which is an
	// absolutely hard limit on the number of VUs the executor will use
	PreAllocatedVUs null.Int `json:"preAllocatedVUs"`
	MaxVUs          null.Int `json:"maxVUs"`
}

// NewConstantArrivalRateConfig returns a ConstantArrivalRateConfig with default values
func NewConstantArrivalRateConfig(name string) ConstantArrivalRateConfig {
	return ConstantArrivalRateConfig{
		BaseConfig: NewBaseConfig(name, constantArrivalRateType),
		TimeUnit:   types.NewNullDuration(1*time.Second, false),
	}
}

// Make sure we implement the lib.ExecutorConfig interface
var _ lib.ExecutorConfig = &ConstantArrivalRateConfig{}

// GetPreAllocatedVUs is just a helper method that returns the scaled pre-allocated VUs.
func (carc ConstantArrivalRateConfig) GetPreAllocatedVUs(et *lib.ExecutionTuple) int64 {
	return et.ScaleInt64(carc.PreAllocatedVUs.Int64)
}

// GetMaxVUs is just a helper method that returns the scaled max VUs.
func (carc ConstantArrivalRateConfig) GetMaxVUs(et *lib.ExecutionTuple) int64 {
	return et.ScaleInt64(carc.MaxVUs.Int64)
}

// GetDescription returns a human-readable description of the executor options
func (carc ConstantArrivalRateConfig) GetDescription(et *lib.ExecutionTuple) string {
	preAllocatedVUs, maxVUs := carc.GetPreAllocatedVUs(et), carc.GetMaxVUs(et)
	maxVUsRange := fmt.Sprintf("maxVUs: %d", preAllocatedVUs)
	if maxVUs > preAllocatedVUs {
		maxVUsRange += fmt.Sprintf("-%d", maxVUs)
	}

	timeUnit := time.Duration(carc.TimeUnit.Duration)
	var arrRatePerSec float64
	if maxVUs != 0 { // TODO: do something better?
		ratio := big.NewRat(maxVUs, carc.MaxVUs.Int64)
		arrRate := big.NewRat(carc.Rate.Int64, int64(timeUnit))
		arrRate.Mul(arrRate, ratio)
		arrRatePerSec, _ = getArrivalRatePerSec(arrRate).Float64()
	}

	return fmt.Sprintf("%.2f iterations/s for %s%s", arrRatePerSec, carc.Duration.Duration,
		carc.getBaseInfo(maxVUsRange))
}

// Validate makes sure all options are configured and valid
func (carc ConstantArrivalRateConfig) Validate() []error {
	errors := carc.BaseConfig.Validate()
	if !carc.Rate.Valid {
		errors = append(errors, fmt.Errorf("the iteration rate isn't specified"))
	} else if carc.Rate.Int64 <= 0 {
		errors = append(errors, fmt.Errorf("the iteration rate should be more than 0"))
	}

	if time.Duration(carc.TimeUnit.Duration) <= 0 {
		errors = append(errors, fmt.Errorf("the timeUnit should be more than 0"))
	}

	if !carc.Duration.Valid {
		errors = append(errors, fmt.Errorf("the duration is unspecified"))
	} else if time.Duration(carc.Duration.Duration) < minDuration {
		errors = append(errors, fmt.Errorf(
			"the duration should be at least %s, but is %s", minDuration, carc.Duration,
		))
	}

	if !carc.PreAllocatedVUs.Valid {
		errors = append(errors, fmt.Errorf("the number of preAllocatedVUs isn't specified"))
	} else if carc.PreAllocatedVUs.Int64 < 0 {
		errors = append(errors, fmt.Errorf("the number of preAllocatedVUs shouldn't be negative"))
	}

	if !carc.MaxVUs.Valid {
		errors = append(errors, fmt.Errorf("the number of maxVUs isn't specified"))
	} else if carc.MaxVUs.Int64 < carc.PreAllocatedVUs.Int64 {
		errors = append(errors, fmt.Errorf("maxVUs shouldn't be less than preAllocatedVUs"))
	}

	return errors
}

// GetExecutionRequirements returns the number of required VUs to run the
// executor for its whole duration (disregarding any startTime), including the
// maximum waiting time for any iterations to gracefully stop. This is used by
// the execution scheduler in its VU reservation calculations, so it knows how
// many VUs to pre-initialize.
func (carc ConstantArrivalRateConfig) GetExecutionRequirements(et *lib.ExecutionTuple) []lib.ExecutionStep {
	return []lib.ExecutionStep{
		{
			TimeOffset:      0,
			PlannedVUs:      uint64(et.ScaleInt64(carc.PreAllocatedVUs.Int64)),
			MaxUnplannedVUs: uint64(et.ScaleInt64(carc.MaxVUs.Int64) - et.ScaleInt64(carc.PreAllocatedVUs.Int64)),
		}, {
			TimeOffset:      time.Duration(carc.Duration.Duration + carc.GracefulStop.Duration),
			PlannedVUs:      0,
			MaxUnplannedVUs: 0,
		},
	}
}

// NewExecutor creates a new ConstantArrivalRate executor
func (carc ConstantArrivalRateConfig) NewExecutor(
	es *lib.ExecutionState, logger *logrus.Entry,
) (lib.Executor, error) {
	return &ConstantArrivalRate{
		BaseExecutor: NewBaseExecutor(carc, es, logger),
		config:       carc,
	}, nil
}

// HasWork reports whether there is any work to be done for the given execution segment.
func (carc ConstantArrivalRateConfig) HasWork(et *lib.ExecutionTuple) bool {
	return carc.GetMaxVUs(et) > 0
}

// ConstantArrivalRate tries to execute a specific number of iterations for a
// specific period.
type ConstantArrivalRate struct {
	*BaseExecutor
	config ConstantArrivalRateConfig
	et     *lib.ExecutionTuple
}

// Make sure we implement the lib.Executor interface.
var _ lib.Executor = &ConstantArrivalRate{}

// Init values needed for the execution
func (car *ConstantArrivalRate) Init(ctx context.Context) error {
	car.et = car.BaseExecutor.executionState.ExecutionTuple.GetNewExecutionTupleBasedOnValue(car.config.MaxVUs.Int64)
	return nil
}

// Run executes a constant number of iterations per second.
//
// TODO: Reuse the variable arrival rate method?
func (car ConstantArrivalRate) Run(ctx context.Context, out chan<- stats.SampleContainer) (err error) { //nolint:funlen
	gracefulStop := car.config.GetGracefulStop()
	duration := time.Duration(car.config.Duration.Duration)
	preAllocatedVUs := car.config.GetPreAllocatedVUs(car.executionState.ExecutionTuple)
	maxVUs := car.config.GetMaxVUs(car.executionState.ExecutionTuple)
	// TODO: refactor and simplify
	arrivalRate := getScaledArrivalRate(car.et.ES, car.config.Rate.Int64, time.Duration(car.config.TimeUnit.Duration))
	tickerPeriod := time.Duration(getTickerPeriod(arrivalRate).Duration)
	arrivalRatePerSec, _ := getArrivalRatePerSec(arrivalRate).Float64()

	// Make sure the log and the progress bar have accurate information
	car.logger.WithFields(logrus.Fields{
		"maxVUs": maxVUs, "preAllocatedVUs": preAllocatedVUs, "duration": duration,
		"tickerPeriod": tickerPeriod, "type": car.config.GetType(),
	}).Debug("Starting executor run...")

	// Pre-allocate the VUs local shared buffer
	activeVUs := make(chan lib.ActiveVU, maxVUs)
	activeVUsCount := uint64(0)

	activeVUsWg := &sync.WaitGroup{}
	defer activeVUsWg.Wait()

	startTime, maxDurationCtx, regDurationCtx, cancel := getDurationContexts(ctx, duration, gracefulStop)
	defer cancel()

	// Make sure all VUs aren't executing iterations anymore, for the cancel()
	// above to deactivate them.
	defer func() {
		// activeVUsCount is modified only in the loop below, which is done here
		for i := uint64(0); i < activeVUsCount; i++ {
			<-activeVUs
		}
	}()

	activateVU := func(initVU lib.InitializedVU) lib.ActiveVU {
		activeVUsWg.Add(1)
		activeVU := initVU.Activate(&lib.VUActivationParams{
			RunContext: maxDurationCtx,
			DeactivateCallback: func() {
				car.executionState.ReturnVU(initVU, true)
				activeVUsWg.Done()
			},
		})
		car.executionState.ModCurrentlyActiveVUsCount(+1)
		atomic.AddUint64(&activeVUsCount, 1)
		return activeVU
	}

	// Get the pre-allocated VUs in the local buffer
	for i := int64(0); i < preAllocatedVUs; i++ {
		initVU, err := car.executionState.GetPlannedVU(car.logger, false)
		if err != nil {
			return err
		}
		activeVUs <- activateVU(initVU)
	}

	vusFmt := pb.GetFixedLengthIntFormat(maxVUs)
	progIters := fmt.Sprintf(
		pb.GetFixedLengthFloatFormat(arrivalRatePerSec, 0)+" iters/s", arrivalRatePerSec)
	progresFn := func() (float64, []string) {
		spent := time.Since(startTime)
		currActiveVUs := atomic.LoadUint64(&activeVUsCount)
		vusInBuffer := uint64(len(activeVUs))
		progVUs := fmt.Sprintf(vusFmt+"/"+vusFmt+" VUs",
			currActiveVUs-vusInBuffer, currActiveVUs)

		right := []string{progVUs, duration.String(), progIters}

		if spent > duration {
			return 1, right
		}

		spentDuration := pb.GetFixedLengthDuration(spent, duration)
		progDur := fmt.Sprintf("%s/%s", spentDuration, duration)
		right[1] = progDur

		return math.Min(1, float64(spent)/float64(duration)), right
	}
	car.progress.Modify(pb.WithProgress(progresFn))
	go trackProgress(ctx, maxDurationCtx, regDurationCtx, &car, progresFn)

	runIterationBasic := getIterationRunner(car.executionState, car.logger)
	runIteration := func(vu lib.ActiveVU) {
		runIterationBasic(maxDurationCtx, vu)
		activeVUs <- vu
	}

	remainingUnplannedVUs := maxVUs - preAllocatedVUs
	start, offsets, _ := car.et.GetStripedOffsets(car.et.ES)
	startTime = time.Now()
	timer := time.NewTimer(time.Hour * 24)
	// here the we need the not scaled one
	notScaledTickerPeriod := time.Duration(
		getTickerPeriod(
			big.NewRat(
				car.config.Rate.Int64,
				int64(time.Duration(car.config.TimeUnit.Duration)),
			)).Duration)

	for li, gi := 0, start; ; li, gi = li+1, gi+offsets[li%len(offsets)] {
		t := notScaledTickerPeriod*time.Duration(gi) - time.Since(startTime)
		timer.Reset(t)
		select {
		case <-timer.C:
			select {
			case vu := <-activeVUs:
				// ideally, we get the VU from the buffer without any issues
				go runIteration(vu)
			default:
				if remainingUnplannedVUs == 0 {
					// TODO: emit an error metric?
					car.logger.Warningf("Insufficient VUs, reached %d active VUs and cannot allocate more", maxVUs)
					break
				}
				initVU, err := car.executionState.GetUnplannedVU(maxDurationCtx, car.logger)
				if err != nil {
					return err
				}
				remainingUnplannedVUs--
				go runIteration(activateVU(initVU))
			}
		case <-regDurationCtx.Done():
			return nil
		}
	}
}
