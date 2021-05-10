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
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/stats"
	"go.k6.io/k6/ui/pb"
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
func NewConstantArrivalRateConfig(name string) *ConstantArrivalRateConfig {
	return &ConstantArrivalRateConfig{
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
func (carc *ConstantArrivalRateConfig) Validate() []error {
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
		// TODO: don't change the config while validating
		carc.MaxVUs.Int64 = carc.PreAllocatedVUs.Int64
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
		BaseExecutor: NewBaseExecutor(&carc, es, logger),
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
	// err should always be nil, because Init() won't be called for executors
	// with no work, as determined by their config's HasWork() method.
	et, err := car.BaseExecutor.executionState.ExecutionTuple.GetNewExecutionTupleFromValue(car.config.MaxVUs.Int64)
	car.et = et
	return err
}

// Run executes a constant number of iterations per second.
//
// TODO: Split this up and make an independent component that can be reused
// between the constant and ramping arrival rate executors - that way we can
// keep the complexity in one well-architected part (with short methods and few
// lambdas :D), while having both config frontends still be present for maximum
// UX benefits. Basically, keep the progress bars and scheduling (i.e. at what
// time should iteration X begin) different, but keep everything else the same.
// This will allow us to implement https://github.com/loadimpact/k6/issues/1386
// and things like all of the TODOs below in one place only.
//nolint:funlen
func (car ConstantArrivalRate) Run(parentCtx context.Context, out chan<- stats.SampleContainer) (err error) {
	gracefulStop := car.config.GetGracefulStop()
	duration := time.Duration(car.config.Duration.Duration)
	preAllocatedVUs := car.config.GetPreAllocatedVUs(car.executionState.ExecutionTuple)
	maxVUs := car.config.GetMaxVUs(car.executionState.ExecutionTuple)
	// TODO: refactor and simplify
	arrivalRate := getScaledArrivalRate(car.et.Segment, car.config.Rate.Int64, time.Duration(car.config.TimeUnit.Duration))
	tickerPeriod := time.Duration(getTickerPeriod(arrivalRate).Duration)
	arrivalRatePerSec, _ := getArrivalRatePerSec(arrivalRate).Float64()

	// Make sure the log and the progress bar have accurate information
	car.logger.WithFields(logrus.Fields{
		"maxVUs": maxVUs, "preAllocatedVUs": preAllocatedVUs, "duration": duration,
		"tickerPeriod": tickerPeriod, "type": car.config.GetType(),
	}).Debug("Starting executor run...")

	activeVUsWg := &sync.WaitGroup{}

	returnedVUs := make(chan struct{})
	startTime, maxDurationCtx, regDurationCtx, cancel := getDurationContexts(parentCtx, duration, gracefulStop)

	defer func() {
		// Make sure all VUs aren't executing iterations anymore, for the cancel()
		// below to deactivate them.
		<-returnedVUs
		cancel()
		activeVUsWg.Wait()
	}()
	activeVUs := make(chan lib.ActiveVU, maxVUs)
	activeVUsCount := uint64(0)

	returnVU := func(u lib.InitializedVU) {
		car.executionState.ReturnVU(u, true)
		activeVUsWg.Done()
	}
	activateVU := func(initVU lib.InitializedVU) lib.ActiveVU {
		activeVUsWg.Add(1)
		activeVU := initVU.Activate(getVUActivationParams(maxDurationCtx, car.config.BaseConfig, returnVU))
		car.executionState.ModCurrentlyActiveVUsCount(+1)
		atomic.AddUint64(&activeVUsCount, 1)
		return activeVU
	}

	remainingUnplannedVUs := maxVUs - preAllocatedVUs
	makeUnplannedVUCh := make(chan struct{})
	defer close(makeUnplannedVUCh)
	go func() {
		defer close(returnedVUs)
		defer func() {
			// this is done here as to not have an unplannedVU in the middle of initialization when
			// starting to return activeVUs
			for i := uint64(0); i < atomic.LoadUint64(&activeVUsCount); i++ {
				<-activeVUs
			}
		}()
		for range makeUnplannedVUCh {
			car.logger.Debug("Starting initialization of an unplanned VU...")
			initVU, err := car.executionState.GetUnplannedVU(maxDurationCtx, car.logger)
			if err != nil {
				// TODO figure out how to return it to the Run goroutine
				car.logger.WithError(err).Error("Error while allocating unplanned VU")
			} else {
				car.logger.Debug("The unplanned VU finished initializing successfully!")
				activeVUs <- activateVU(initVU)
			}
		}
	}()

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
	progressFn := func() (float64, []string) {
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
	car.progress.Modify(pb.WithProgress(progressFn))
	go trackProgress(parentCtx, maxDurationCtx, regDurationCtx, &car, progressFn)

	runIterationBasic := getIterationRunner(car.executionState, car.logger)
	runIteration := func(vu lib.ActiveVU) {
		runIterationBasic(maxDurationCtx, vu)
		activeVUs <- vu
	}

	start, offsets, _ := car.et.GetStripedOffsets()
	timer := time.NewTimer(time.Hour * 24)
	// here the we need the not scaled one
	notScaledTickerPeriod := time.Duration(
		getTickerPeriod(
			big.NewRat(
				car.config.Rate.Int64,
				int64(time.Duration(car.config.TimeUnit.Duration)),
			)).Duration)

	shownWarning := false
	metricTags := car.getMetricTags(nil)
	for li, gi := 0, start; ; li, gi = li+1, gi+offsets[li%len(offsets)] {
		t := notScaledTickerPeriod*time.Duration(gi) - time.Since(startTime)
		timer.Reset(t)
		select {
		case <-timer.C:
			select {
			case vu := <-activeVUs: // ideally, we get the VU from the buffer without any issues
				go runIteration(vu) //TODO: refactor so we dont spin up a goroutine for each iteration
				continue
			default: // no free VUs currently available
			}

			// Since there aren't any free VUs available, consider this iteration
			// dropped - we aren't going to try to recover it, but

			stats.PushIfNotDone(parentCtx, out, stats.Sample{
				Value: 1, Metric: metrics.DroppedIterations,
				Tags: metricTags, Time: time.Now(),
			})

			// We'll try to start allocating another VU in the background,
			// non-blockingly, if we have remainingUnplannedVUs...
			if remainingUnplannedVUs == 0 {
				if !shownWarning {
					car.logger.Warningf("Insufficient VUs, reached %d active VUs and cannot initialize more", maxVUs)
					shownWarning = true
				}
				continue
			}

			select {
			case makeUnplannedVUCh <- struct{}{}: // great!
				remainingUnplannedVUs--
			default: // we're already allocating a new VU
			}

		case <-regDurationCtx.Done():
			return nil
		}
	}
}
