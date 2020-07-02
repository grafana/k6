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

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/ui/pb"
)

const sharedIterationsType = "shared-iterations"

func init() {
	lib.RegisterExecutorConfigType(
		sharedIterationsType,
		func(name string, rawJSON []byte) (lib.ExecutorConfig, error) {
			config := NewSharedIterationsConfig(name)
			err := lib.StrictJSONUnmarshal(rawJSON, &config)
			return config, err
		},
	)
}

// SharedIterationsConfig stores the number of VUs iterations, as well as maxDuration settings
type SharedIterationsConfig struct {
	BaseConfig
	VUs         null.Int           `json:"vus"`
	Iterations  null.Int           `json:"iterations"`
	MaxDuration types.NullDuration `json:"maxDuration"`
}

// NewSharedIterationsConfig returns a SharedIterationsConfig with default values
func NewSharedIterationsConfig(name string) SharedIterationsConfig {
	return SharedIterationsConfig{
		BaseConfig:  NewBaseConfig(name, sharedIterationsType),
		VUs:         null.NewInt(1, false),
		Iterations:  null.NewInt(1, false),
		MaxDuration: types.NewNullDuration(10*time.Minute, false), // TODO: shorten?
	}
}

// Make sure we implement the lib.ExecutorConfig interface
var _ lib.ExecutorConfig = &SharedIterationsConfig{}

// GetVUs returns the scaled VUs for the executor.
func (sic SharedIterationsConfig) GetVUs(et *lib.ExecutionTuple) int64 {
	return et.ScaleInt64(sic.VUs.Int64)
}

// GetIterations returns the scaled iteration count for the executor.
func (sic SharedIterationsConfig) GetIterations(et *lib.ExecutionTuple) int64 {
	// TODO: Optimize this by probably changing the whole Config API
	newTuple, err := et.GetNewExecutionTupleFromValue(sic.VUs.Int64)
	if err != nil {
		return 0
	}
	return newTuple.ScaleInt64(sic.Iterations.Int64)
}

// GetDescription returns a human-readable description of the executor options
func (sic SharedIterationsConfig) GetDescription(et *lib.ExecutionTuple) string {
	return fmt.Sprintf("%d iterations shared among %d VUs%s",
		sic.GetIterations(et), sic.GetVUs(et),
		sic.getBaseInfo(fmt.Sprintf("maxDuration: %s", sic.MaxDuration.Duration)))
}

// Validate makes sure all options are configured and valid
func (sic SharedIterationsConfig) Validate() []error {
	errors := sic.BaseConfig.Validate()
	if sic.VUs.Int64 <= 0 {
		errors = append(errors, fmt.Errorf("the number of VUs should be more than 0"))
	}

	if sic.Iterations.Int64 < sic.VUs.Int64 {
		errors = append(errors, fmt.Errorf(
			"the number of iterations (%d) shouldn't be less than the number of VUs (%d)",
			sic.Iterations.Int64, sic.VUs.Int64,
		))
	}

	if time.Duration(sic.MaxDuration.Duration) < minDuration {
		errors = append(errors, fmt.Errorf(
			"the maxDuration should be at least %s, but is %s", minDuration, sic.MaxDuration,
		))
	}

	return errors
}

// GetExecutionRequirements returns the number of required VUs to run the
// executor for its whole duration (disregarding any startTime), including the
// maximum waiting time for any iterations to gracefully stop. This is used by
// the execution scheduler in its VU reservation calculations, so it knows how
// many VUs to pre-initialize.
func (sic SharedIterationsConfig) GetExecutionRequirements(et *lib.ExecutionTuple) []lib.ExecutionStep {
	vus := sic.GetVUs(et)
	if vus == 0 {
		return []lib.ExecutionStep{
			{
				TimeOffset: 0,
				PlannedVUs: 0,
			},
		}
	}

	return []lib.ExecutionStep{
		{
			TimeOffset: 0,
			PlannedVUs: uint64(vus),
		},
		{
			TimeOffset: time.Duration(sic.MaxDuration.Duration + sic.GracefulStop.Duration),
			PlannedVUs: 0,
		},
	}
}

// NewExecutor creates a new SharedIterations executor
func (sic SharedIterationsConfig) NewExecutor(
	es *lib.ExecutionState, logger *logrus.Entry,
) (lib.Executor, error) {
	return &SharedIterations{
		BaseExecutor: NewBaseExecutor(sic, es, logger),
		config:       sic,
	}, nil
}

// SharedIterations executes a specific total number of iterations, which are
// all shared by the configured VUs.
type SharedIterations struct {
	*BaseExecutor
	config SharedIterationsConfig
	et     *lib.ExecutionTuple
}

// Make sure we implement the lib.Executor interface.
var _ lib.Executor = &SharedIterations{}

// HasWork reports whether there is any work to be done for the given execution segment.
func (sic SharedIterationsConfig) HasWork(et *lib.ExecutionTuple) bool {
	return sic.GetVUs(et) > 0 && sic.GetIterations(et) > 0
}

// Init values needed for the execution
func (si *SharedIterations) Init(ctx context.Context) error {
	// err should always be nil, because Init() won't be called for executors
	// with no work, as determined by their config's HasWork() method.
	et, err := si.BaseExecutor.executionState.ExecutionTuple.GetNewExecutionTupleFromValue(si.config.VUs.Int64)
	si.et = et
	return err
}

// Run executes a specific total number of iterations, which are all shared by
// the configured VUs.
// nolint:funlen
func (si SharedIterations) Run(ctx context.Context, out chan<- stats.SampleContainer) (err error) {
	numVUs := si.config.GetVUs(si.executionState.ExecutionTuple)
	iterations := si.et.ScaleInt64(si.config.Iterations.Int64)
	duration := time.Duration(si.config.MaxDuration.Duration)
	gracefulStop := si.config.GetGracefulStop()

	startTime, maxDurationCtx, regDurationCtx, cancel := getDurationContexts(ctx, duration, gracefulStop)
	defer cancel()

	// Make sure the log and the progress bar have accurate information
	si.logger.WithFields(logrus.Fields{
		"vus": numVUs, "iterations": iterations, "maxDuration": duration, "type": si.config.GetType(),
	}).Debug("Starting executor run...")

	totalIters := uint64(iterations)
	doneIters := new(uint64)
	vusFmt := pb.GetFixedLengthIntFormat(numVUs)
	itersFmt := pb.GetFixedLengthIntFormat(int64(totalIters))
	progresFn := func() (float64, []string) {
		spent := time.Since(startTime)
		progVUs := fmt.Sprintf(vusFmt+" VUs", numVUs)
		currentDoneIters := atomic.LoadUint64(doneIters)
		progIters := fmt.Sprintf(itersFmt+"/"+itersFmt+" shared iters",
			currentDoneIters, totalIters)
		spentDuration := pb.GetFixedLengthDuration(spent, duration)
		progDur := fmt.Sprintf("%s/%s", spentDuration, duration)
		right := []string{progVUs, progDur, progIters}

		return float64(currentDoneIters) / float64(totalIters), right
	}
	si.progress.Modify(pb.WithProgress(progresFn))
	go trackProgress(ctx, maxDurationCtx, regDurationCtx, &si, progresFn)

	var attemptedIters uint64

	// Actually schedule the VUs and iterations...
	activeVUs := &sync.WaitGroup{}
	defer func() {
		activeVUs.Wait()
		if attemptedIters < totalIters {
			stats.PushIfNotDone(ctx, out, stats.Sample{
				Value: float64(totalIters - attemptedIters), Metric: metrics.DroppedIterations,
				Tags: si.getMetricTags(nil), Time: time.Now(),
			})
		}
	}()

	regDurationDone := regDurationCtx.Done()
	runIteration := getIterationRunner(si.executionState, si.logger)

	activationParams := getVUActivationParams(maxDurationCtx, si.config.BaseConfig,
		func(u lib.InitializedVU) {
			si.executionState.ReturnVU(u, true)
			activeVUs.Done()
		})
	handleVU := func(initVU lib.InitializedVU) {
		ctx, cancel := context.WithCancel(maxDurationCtx)
		defer cancel()

		newParams := *activationParams
		newParams.RunContext = ctx

		activeVU := initVU.Activate(&newParams)

		for {
			select {
			case <-regDurationDone:
				return // don't make more iterations
			default:
				// continue looping
			}

			attemptedIterNumber := atomic.AddUint64(&attemptedIters, 1)
			if attemptedIterNumber > totalIters {
				return
			}

			runIteration(maxDurationCtx, activeVU)
			atomic.AddUint64(doneIters, 1)
		}
	}

	for i := int64(0); i < numVUs; i++ {
		initVU, err := si.executionState.GetPlannedVU(si.logger, true)
		if err != nil {
			cancel()
			return err
		}
		activeVUs.Add(1)
		go handleVU(initVU)
	}

	return nil
}
