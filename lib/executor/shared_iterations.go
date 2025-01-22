package executor

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/internal/ui/pb"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
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
		errors = append(errors, fmt.Errorf("the number of VUs must be more than 0"))
	}

	if sic.Iterations.Int64 < sic.VUs.Int64 {
		errors = append(errors, fmt.Errorf(
			"the number of iterations (%d) can't be less than the number of VUs (%d)",
			sic.Iterations.Int64, sic.VUs.Int64,
		))
	}

	if sic.MaxDuration.TimeDuration() < minDuration {
		errors = append(errors, fmt.Errorf(
			"the maxDuration must be at least %s, but is %s", minDuration, sic.MaxDuration,
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
			PlannedVUs: uint64(vus), //nolint:gosec
		},
		{
			TimeOffset: sic.MaxDuration.TimeDuration() + sic.GracefulStop.TimeDuration(),
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
func (si *SharedIterations) Init(_ context.Context) error {
	// err should always be nil, because Init() won't be called for executors
	// with no work, as determined by their config's HasWork() method.
	et, err := si.BaseExecutor.executionState.ExecutionTuple.GetNewExecutionTupleFromValue(si.config.VUs.Int64)
	si.et = et
	si.iterSegIndex = lib.NewSegmentedIndex(et)

	return err
}

// Run executes a specific total number of iterations, which are all shared by
// the configured VUs.
//
//nolint:funlen
func (si SharedIterations) Run(parentCtx context.Context, out chan<- metrics.SampleContainer) (err error) {
	numVUs := si.config.GetVUs(si.executionState.ExecutionTuple)
	iterations := si.et.ScaleInt64(si.config.Iterations.Int64)
	duration := si.config.MaxDuration.TimeDuration()
	gracefulStop := si.config.GetGracefulStop()

	waitOnProgressChannel := make(chan struct{})
	startTime, maxDurationCtx, regDurationCtx, cancel := getDurationContexts(parentCtx, duration, gracefulStop)
	defer func() {
		cancel()
		<-waitOnProgressChannel
	}()

	// Make sure the log and the progress bar have accurate information
	si.logger.WithFields(logrus.Fields{
		"vus": numVUs, "iterations": iterations, "maxDuration": duration, "type": si.config.GetType(),
	}).Debug("Starting executor run...")

	totalIters := uint64(iterations) //nolint:gosec
	doneIters := new(uint64)
	vusFmt := pb.GetFixedLengthIntFormat(numVUs)
	itersFmt := pb.GetFixedLengthIntFormat(iterations)
	progressFn := func() (float64, []string) {
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
	si.progress.Modify(pb.WithProgress(progressFn))
	maxDurationCtx = lib.WithScenarioState(maxDurationCtx, &lib.ScenarioState{
		Name:       si.config.Name,
		Executor:   si.config.Type,
		StartTime:  startTime,
		ProgressFn: progressFn,
	})
	go func() {
		trackProgress(parentCtx, maxDurationCtx, regDurationCtx, &si, progressFn)
		close(waitOnProgressChannel)
	}()

	var attemptedIters uint64

	// Actually schedule the VUs and iterations...
	activeVUs := &sync.WaitGroup{}
	defer func() {
		activeVUs.Wait()
		if attemptedIters < totalIters {
			metrics.PushIfNotDone(parentCtx, out, metrics.Sample{
				TimeSeries: metrics.TimeSeries{
					Metric: si.executionState.Test.BuiltinMetrics.DroppedIterations,
					Tags:   si.getMetricTags(nil),
				},
				Value: float64(totalIters - attemptedIters),
				Time:  time.Now(),
			})
		}
	}()

	regDurationDone := regDurationCtx.Done()
	runIteration := getIterationRunner(si.executionState, si.logger)

	returnVU := func(u lib.InitializedVU) {
		si.executionState.ReturnVU(u, true)
		activeVUs.Done()
	}

	handleVU := func(initVU lib.InitializedVU) {
		ctx, cancel := context.WithCancel(maxDurationCtx)
		defer cancel()

		activeVU := initVU.Activate(getVUActivationParams(
			ctx, si.config.BaseConfig, returnVU, si.nextIterationCounters))

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
