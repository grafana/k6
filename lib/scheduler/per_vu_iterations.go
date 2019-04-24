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

const perVUIterationsType = "per-vu-iterations"

func init() {
	lib.RegisterSchedulerConfigType(perVUIterationsType, func(name string, rawJSON []byte) (lib.SchedulerConfig, error) {
		config := NewPerVUIterationsConfig(name)
		err := lib.StrictJSONUnmarshal(rawJSON, &config)
		return config, err
	})
}

// PerVUIteationsConfig stores the number of VUs iterations, as well as maxDuration settings
type PerVUIteationsConfig struct {
	BaseConfig
	VUs         null.Int           `json:"vus"`
	Iterations  null.Int           `json:"iterations"`
	MaxDuration types.NullDuration `json:"maxDuration"`
}

// NewPerVUIterationsConfig returns a PerVUIteationsConfig with default values
func NewPerVUIterationsConfig(name string) PerVUIteationsConfig {
	return PerVUIteationsConfig{
		BaseConfig:  NewBaseConfig(name, perVUIterationsType),
		VUs:         null.NewInt(1, false),
		Iterations:  null.NewInt(1, false),
		MaxDuration: types.NewNullDuration(10*time.Minute, false), //TODO: shorten?
	}
}

// Make sure we implement the lib.SchedulerConfig interface
var _ lib.SchedulerConfig = &PerVUIteationsConfig{}

// GetVUs returns the scaled VUs for the scheduler.
func (pvic PerVUIteationsConfig) GetVUs(es *lib.ExecutionSegment) int64 {
	return es.Scale(pvic.VUs.Int64)
}

// GetIterations returns the UNSCALED iteration count for the scheduler. It's
// important to note that scaling per-VU iteration scheduler affects only the
// number of VUs. If we also scaled the iterations, scaling would have quadratic
// effects instead of just linear.
func (pvic PerVUIteationsConfig) GetIterations() int64 {
	return pvic.Iterations.Int64
}

// GetDescription returns a human-readable description of the scheduler options
func (pvic PerVUIteationsConfig) GetDescription(es *lib.ExecutionSegment) string {
	return fmt.Sprintf("%d iterations for each of %d VUs%s",
		pvic.GetIterations(), pvic.GetVUs(es),
		pvic.getBaseInfo(fmt.Sprintf("maxDuration: %s", pvic.MaxDuration.Duration)))
}

// Validate makes sure all options are configured and valid
func (pvic PerVUIteationsConfig) Validate() []error {
	errors := pvic.BaseConfig.Validate()
	if pvic.VUs.Int64 <= 0 {
		errors = append(errors, fmt.Errorf("the number of VUs should be more than 0"))
	}

	if pvic.Iterations.Int64 <= 0 {
		errors = append(errors, fmt.Errorf("the number of iterations should be more than 0"))
	}

	if time.Duration(pvic.MaxDuration.Duration) < minDuration {
		errors = append(errors, fmt.Errorf(
			"the maxDuration should be at least %s, but is %s", minDuration, pvic.MaxDuration,
		))
	}

	return errors
}

// GetExecutionRequirements just reserves the number of specified VUs for the
// whole duration of the scheduler, including the maximum waiting time for
// iterations to gracefully stop.
func (pvic PerVUIteationsConfig) GetExecutionRequirements(es *lib.ExecutionSegment) []lib.ExecutionStep {
	return []lib.ExecutionStep{
		{
			TimeOffset: 0,
			PlannedVUs: uint64(pvic.GetVUs(es)),
		},
		{
			TimeOffset: time.Duration(pvic.MaxDuration.Duration + pvic.GracefulStop.Duration),
			PlannedVUs: 0,
		},
	}
}

// NewScheduler creates a new PerVUIteations scheduler
func (pvic PerVUIteationsConfig) NewScheduler(
	es *lib.ExecutorState, logger *logrus.Entry) (lib.Scheduler, error) {

	return PerVUIteations{
		BaseScheduler: NewBaseScheduler(pvic, es, logger),
		config:        pvic,
	}, nil
}

// PerVUIteations executes a specific number of iterations with each VU.
type PerVUIteations struct {
	*BaseScheduler
	config PerVUIteationsConfig
}

// Make sure we implement the lib.Scheduler interface.
var _ lib.Scheduler = &PerVUIteations{}

// Run executes a specific number of iterations with each confugured VU.
func (pvi PerVUIteations) Run(ctx context.Context, out chan<- stats.SampleContainer) (err error) {
	segment := pvi.executorState.Options.ExecutionSegment
	numVUs := pvi.config.GetVUs(segment)
	iterations := pvi.config.GetIterations()
	duration := time.Duration(pvi.config.MaxDuration.Duration)
	gracefulStop := pvi.config.GetGracefulStop()

	_, maxDurationCtx, regDurationCtx, cancel := getDurationContexts(ctx, duration, gracefulStop)
	defer cancel()

	// Make sure the log and the progress bar have accurate information
	pvi.logger.WithFields(logrus.Fields{
		"vus": numVUs, "iterations": iterations, "maxDuration": duration, "type": pvi.config.GetType(),
	}).Debug("Starting scheduler run...")

	totalIters := uint64(numVUs * iterations)
	doneIters := new(uint64)
	fmtStr := pb.GetFixedLengthIntFormat(int64(totalIters)) + "/%d iters, %d from each of %d VUs"
	progresFn := func() (float64, string) {
		currentDoneIters := atomic.LoadUint64(doneIters)
		return float64(currentDoneIters) / float64(totalIters), fmt.Sprintf(
			fmtStr, currentDoneIters, totalIters, iterations, numVUs,
		)
	}
	pvi.progress.Modify(pb.WithProgress(progresFn))
	go trackProgress(ctx, maxDurationCtx, regDurationCtx, pvi, progresFn)

	// Actually schedule the VUs and iterations...
	activeVUs := &sync.WaitGroup{}
	defer activeVUs.Wait()

	regDurationDone := regDurationCtx.Done()
	runIteration := getIterationRunner(pvi.executorState, pvi.logger, out)

	handleVU := func(vu lib.VU) {
		defer pvi.executorState.ReturnVU(vu)
		defer activeVUs.Done()

		for i := int64(0); i < iterations; i++ {
			select {
			case <-regDurationDone:
				return // don't make more iterations
			default:
				// continue looping
			}
			runIteration(maxDurationCtx, vu)
			atomic.AddUint64(doneIters, 1)
		}
	}

	for i := int64(0); i < numVUs; i++ {
		vu, err := pvi.executorState.GetPlannedVU(pvi.logger)
		if err != nil {
			cancel()
			return err
		}
		activeVUs.Add(1)
		go handleVU(vu)
	}

	return nil
}
