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
	"fmt"
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

const sharedIterationsType = "shared-iterations"

func init() {
	lib.RegisterSchedulerConfigType(
		sharedIterationsType,
		func(name string, rawJSON []byte) (lib.SchedulerConfig, error) {
			config := NewSharedIterationsConfig(name)
			err := lib.StrictJSONUnmarshal(rawJSON, &config)
			return config, err
		},
	)
}

// SharedIteationsConfig stores the number of VUs iterations, as well as maxDuration settings
type SharedIteationsConfig struct {
	BaseConfig
	VUs         null.Int           `json:"vus"`
	Iterations  null.Int           `json:"iterations"`
	MaxDuration types.NullDuration `json:"maxDuration"`
}

// NewSharedIterationsConfig returns a SharedIteationsConfig with default values
func NewSharedIterationsConfig(name string) SharedIteationsConfig {
	return SharedIteationsConfig{
		BaseConfig:  NewBaseConfig(name, sharedIterationsType),
		VUs:         null.NewInt(1, false),
		Iterations:  null.NewInt(1, false),
		MaxDuration: types.NewNullDuration(10*time.Minute, false), //TODO: shorten?
	}
}

// Make sure we implement the lib.SchedulerConfig interface
var _ lib.SchedulerConfig = &SharedIteationsConfig{}

// GetVUs returns the scaled VUs for the scheduler.
func (sic SharedIteationsConfig) GetVUs(es *lib.ExecutionSegment) int64 {
	return es.Scale(sic.VUs.Int64)
}

// GetIterations returns the scaled iteration count for the scheduler.
func (sic SharedIteationsConfig) GetIterations(es *lib.ExecutionSegment) int64 {
	return es.Scale(sic.Iterations.Int64)
}

// GetDescription returns a human-readable description of the scheduler options
func (sic SharedIteationsConfig) GetDescription(es *lib.ExecutionSegment) string {
	return fmt.Sprintf("%d iterations shared among %d VUs%s",
		sic.GetIterations(es), sic.GetVUs(es),
		sic.getBaseInfo(fmt.Sprintf("maxDuration: %s", sic.MaxDuration.Duration)))
}

// Validate makes sure all options are configured and valid
func (sic SharedIteationsConfig) Validate() []error {
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

// GetExecutionRequirements just reserves the number of specified VUs for the
// whole duration of the scheduler, including the maximum waiting time for
// iterations to gracefully stop.
func (sic SharedIteationsConfig) GetExecutionRequirements(es *lib.ExecutionSegment) []lib.ExecutionStep {
	return []lib.ExecutionStep{
		{
			TimeOffset: 0,
			PlannedVUs: uint64(sic.GetVUs(es)),
		},
		{
			TimeOffset: time.Duration(sic.MaxDuration.Duration + sic.GracefulStop.Duration),
			PlannedVUs: 0,
		},
	}
}

// NewScheduler creates a new SharedIteations scheduler
func (sic SharedIteationsConfig) NewScheduler(
	es *lib.ExecutorState, logger *logrus.Entry) (lib.Scheduler, error) {

	return SharedIteations{
		BaseScheduler: NewBaseScheduler(sic, es, logger),
		config:        sic,
	}, nil
}

// SharedIteations executes a specific total number of iterations, which are
// all shared by the configured VUs.
type SharedIteations struct {
	*BaseScheduler
	config SharedIteationsConfig
}

// Make sure we implement the lib.Scheduler interface.
var _ lib.Scheduler = &PerVUIteations{}

// Run executes a specific total number of iterations, which are all shared by
// the configured VUs.
func (si SharedIteations) Run(ctx context.Context, out chan<- stats.SampleContainer) (err error) {
	segment := si.executorState.Options.ExecutionSegment
	numVUs := si.config.GetVUs(segment)
	iterations := si.config.GetIterations(segment)
	duration := time.Duration(si.config.MaxDuration.Duration)
	gracefulStop := si.config.GetGracefulStop()

	_, maxDurationCtx, regDurationCtx, cancel := getDurationContexts(ctx, duration, gracefulStop)
	defer cancel()

	// Make sure the log and the progress bar have accurate information
	si.logger.WithFields(logrus.Fields{
		"vus": numVUs, "iterations": iterations, "maxDuration": duration, "type": si.config.GetType(),
	}).Debug("Starting scheduler run...")

	totalIters := uint64(iterations)
	doneIters := new(uint64)
	fmtStr := pb.GetFixedLengthIntFormat(int64(totalIters)) + "/%d shared iters among %d VUs"
	progresFn := func() (float64, string) {
		currentDoneIters := atomic.LoadUint64(doneIters)
		return float64(currentDoneIters) / float64(totalIters), fmt.Sprintf(
			fmtStr, currentDoneIters, totalIters, numVUs,
		)
	}
	si.progress.Modify(pb.WithProgress(progresFn))
	go trackProgress(ctx, maxDurationCtx, regDurationCtx, si, progresFn)

	// Actually schedule the VUs and iterations...
	wg := sync.WaitGroup{}
	regDurationDone := regDurationCtx.Done()
	runIteration := getIterationRunner(si.executorState, si.logger, out)

	attemptedIters := new(uint64)
	handleVU := func(vu lib.VU) {
		defer si.executorState.ReturnVU(vu)
		defer wg.Done()

		for {
			attemptedIterNumber := atomic.AddUint64(attemptedIters, 1)
			if attemptedIterNumber > totalIters {
				return
			}

			runIteration(maxDurationCtx, vu)
			atomic.AddUint64(doneIters, 1)
			select {
			case <-regDurationDone:
				return // don't make more iterations
			default:
				// continue looping
			}
		}
	}

	for i := int64(0); i < numVUs; i++ {
		wg.Add(1)
		vu, err := si.executorState.GetPlannedVU(ctx, si.logger)
		if err != nil {
			return err
		}
		go handleVU(vu)
	}

	wg.Wait()
	return nil
}
