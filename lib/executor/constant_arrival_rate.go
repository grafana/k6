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
	"sync/atomic"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/ui/pb"
	"github.com/sirupsen/logrus"
	null "gopkg.in/guregu/null.v3"
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
func (carc ConstantArrivalRateConfig) GetPreAllocatedVUs(es *lib.ExecutionSegment) int64 {
	return es.Scale(carc.PreAllocatedVUs.Int64)
}

// GetMaxVUs is just a helper method that returns the scaled max VUs.
func (carc ConstantArrivalRateConfig) GetMaxVUs(es *lib.ExecutionSegment) int64 {
	return es.Scale(carc.MaxVUs.Int64)
}

// GetDescription returns a human-readable description of the executor options
func (carc ConstantArrivalRateConfig) GetDescription(es *lib.ExecutionSegment) string {
	preAllocatedVUs, maxVUs := carc.GetPreAllocatedVUs(es), carc.GetMaxVUs(es)
	maxVUsRange := fmt.Sprintf("maxVUs: %d", preAllocatedVUs)
	if maxVUs > preAllocatedVUs {
		maxVUsRange += fmt.Sprintf("-%d", maxVUs)
	}

	timeUnit := time.Duration(carc.TimeUnit.Duration)
	arrRate := getScaledArrivalRate(es, carc.Rate.Int64, timeUnit)
	arrRatePerSec, _ := getArrivalRatePerSec(arrRate).Float64()

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
func (carc ConstantArrivalRateConfig) GetExecutionRequirements(es *lib.ExecutionSegment) []lib.ExecutionStep {
	return []lib.ExecutionStep{
		{
			TimeOffset:      0,
			PlannedVUs:      uint64(es.Scale(carc.PreAllocatedVUs.Int64)),
			MaxUnplannedVUs: uint64(es.Scale(carc.MaxVUs.Int64 - carc.PreAllocatedVUs.Int64)),
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
	return ConstantArrivalRate{
		BaseExecutor: NewBaseExecutor(carc, es, logger),
		config:       carc,
	}, nil
}

// ConstantArrivalRate tries to execute a specific number of iterations for a
// specific period.
type ConstantArrivalRate struct {
	*BaseExecutor
	config ConstantArrivalRateConfig
}

// Make sure we implement the lib.Executor interface.
var _ lib.Executor = &ConstantArrivalRate{}

// Run executes a constant number of iterations per second.
//
// TODO: Reuse the variable arrival rate method?
func (car ConstantArrivalRate) Run(ctx context.Context, out chan<- stats.SampleContainer) (err error) { //nolint:funlen
	segment := car.executionState.Options.ExecutionSegment
	gracefulStop := car.config.GetGracefulStop()
	duration := time.Duration(car.config.Duration.Duration)
	preAllocatedVUs := car.config.GetPreAllocatedVUs(segment)
	maxVUs := car.config.GetMaxVUs(segment)

	arrivalRate := getScaledArrivalRate(segment, car.config.Rate.Int64, time.Duration(car.config.TimeUnit.Duration))
	tickerPeriod := time.Duration(getTickerPeriod(arrivalRate).Duration)
	arrivalRatePerSec, _ := getArrivalRatePerSec(arrivalRate).Float64()

	startTime, maxDurationCtx, regDurationCtx, cancel := getDurationContexts(ctx, duration, gracefulStop)
	defer cancel()
	ticker := time.NewTicker(tickerPeriod) // the rate can't be 0 because of the validation

	// Make sure the log and the progress bar have accurate information
	car.logger.WithFields(logrus.Fields{
		"maxVUs": maxVUs, "preAllocatedVUs": preAllocatedVUs, "duration": duration,
		"tickerPeriod": tickerPeriod, "type": car.config.GetType(),
	}).Debug("Starting executor run...")

	// Pre-allocate the VUs local shared buffer
	vus := make(chan lib.VU, maxVUs)

	initialisedVUs := uint64(0)
	// Make sure we put planned and unplanned VUs back in the global
	// buffer, and as an extra incentive, this replaces a waitgroup.
	defer func() {
		// no need for atomics, since initialisedVUs is mutated only in the select{}
		for i := uint64(0); i < initialisedVUs; i++ {
			car.executionState.ReturnVU(<-vus, true)
		}
	}()

	// Get the pre-allocated VUs in the local buffer
	for i := int64(0); i < preAllocatedVUs; i++ {
		vu, err := car.executionState.GetPlannedVU(car.logger, true)
		if err != nil {
			return err
		}
		initialisedVUs++
		vus <- vu
	}

	vusFmt := pb.GetFixedLengthIntFormat(maxVUs)
	fmtStr := pb.GetFixedLengthFloatFormat(arrivalRatePerSec, 2) +
		" iters/s, " + vusFmt + " out of " + vusFmt + " VUs active"

	progresFn := func() (float64, string) {
		spent := time.Since(startTime)
		currentInitialisedVUs := atomic.LoadUint64(&initialisedVUs)
		vusInBuffer := uint64(len(vus))
		return math.Min(1, float64(spent)/float64(duration)), fmt.Sprintf(fmtStr,
			arrivalRatePerSec, currentInitialisedVUs-vusInBuffer, currentInitialisedVUs,
		)
	}
	car.progress.Modify(pb.WithProgress(progresFn))
	go trackProgress(ctx, maxDurationCtx, regDurationCtx, car, progresFn)

	regDurationDone := regDurationCtx.Done()
	runIterationBasic := getIterationRunner(car.executionState, car.logger, out)
	runIteration := func(vu lib.VU) {
		runIterationBasic(maxDurationCtx, vu)
		vus <- vu
	}

	remainingUnplannedVUs := maxVUs - preAllocatedVUs
	for {
		select {
		case <-ticker.C:
			select {
			case vu := <-vus:
				// ideally, we get the VU from the buffer without any issues
				go runIteration(vu)
			default:
				if remainingUnplannedVUs == 0 {
					//TODO: emit an error metric?
					car.logger.Warningf("Insufficient VUs, reached %d active VUs and cannot allocate more", maxVUs)
					break
				}
				vu, err := car.executionState.GetUnplannedVU(maxDurationCtx, car.logger)
				if err != nil {
					return err
				}
				remainingUnplannedVUs--
				atomic.AddUint64(&initialisedVUs, 1)
				go runIteration(vu)
			}
		case <-regDurationDone:
			return nil
		}
	}
}
