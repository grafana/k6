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

const perVUIterationsType = "per-vu-iterations"

func init() {
	lib.RegisterExecutorConfigType(perVUIterationsType, func(name string, rawJSON []byte) (lib.ExecutorConfig, error) {
		config := NewPerVUIterationsConfig(name)
		err := lib.StrictJSONUnmarshal(rawJSON, &config)
		return config, err
	})
}

// PerVUIterationsConfig stores the number of VUs iterations, as well as maxDuration settings
type PerVUIterationsConfig struct {
	BaseConfig
	VUs         null.Int           `json:"vus"`
	Iterations  null.Int           `json:"iterations"`
	MaxDuration types.NullDuration `json:"maxDuration"`
}

// NewPerVUIterationsConfig returns a PerVUIterationsConfig with default values
func NewPerVUIterationsConfig(name string) PerVUIterationsConfig {
	return PerVUIterationsConfig{
		BaseConfig:  NewBaseConfig(name, perVUIterationsType),
		VUs:         null.NewInt(1, false),
		Iterations:  null.NewInt(1, false),
		MaxDuration: types.NewNullDuration(10*time.Minute, false), // TODO: shorten?
	}
}

// Make sure we implement the lib.ExecutorConfig interface
var _ lib.ExecutorConfig = &PerVUIterationsConfig{}

// GetVUs returns the scaled VUs for the executor.
func (pvic PerVUIterationsConfig) GetVUs(et *lib.ExecutionTuple) int64 {
	return et.ScaleInt64(pvic.VUs.Int64)
}

// GetIterations returns the UNSCALED iteration count for the executor. It's
// important to note that scaling per-VU iteration executor affects only the
// number of VUs. If we also scaled the iterations, scaling would have quadratic
// effects instead of just linear.
func (pvic PerVUIterationsConfig) GetIterations() int64 {
	return pvic.Iterations.Int64
}

// GetDescription returns a human-readable description of the executor options
func (pvic PerVUIterationsConfig) GetDescription(et *lib.ExecutionTuple) string {
	return fmt.Sprintf("%d iterations for each of %d VUs%s",
		pvic.GetIterations(), pvic.GetVUs(et),
		pvic.getBaseInfo(fmt.Sprintf("maxDuration: %s", pvic.MaxDuration.Duration)))
}

// Validate makes sure all options are configured and valid
func (pvic PerVUIterationsConfig) Validate() []error {
	errors := pvic.BaseConfig.Validate()
	if pvic.VUs.Int64 <= 0 {
		errors = append(errors, fmt.Errorf("the number of VUs must be more than 0"))
	}

	if pvic.Iterations.Int64 <= 0 {
		errors = append(errors, fmt.Errorf("the number of iterations must be more than 0"))
	}

	if pvic.MaxDuration.TimeDuration() < minDuration {
		errors = append(errors, fmt.Errorf(
			"the maxDuration must be at least %s, but is %s", minDuration, pvic.MaxDuration,
		))
	}

	return errors
}

// GetExecutionRequirements returns the number of required VUs to run the
// executor for its whole duration (disregarding any startTime), including the
// maximum waiting time for any iterations to gracefully stop. This is used by
// the execution scheduler in its VU reservation calculations, so it knows how
// many VUs to pre-initialize.
func (pvic PerVUIterationsConfig) GetExecutionRequirements(et *lib.ExecutionTuple) []lib.ExecutionStep {
	return []lib.ExecutionStep{
		{
			TimeOffset: 0,
			PlannedVUs: uint64(pvic.GetVUs(et)), //nolint:gosec
		},
		{
			TimeOffset: pvic.MaxDuration.TimeDuration() + pvic.GracefulStop.TimeDuration(),
			PlannedVUs: 0,
		},
	}
}

// NewExecutor creates a new PerVUIterations executor
func (pvic PerVUIterationsConfig) NewExecutor(
	es *lib.ExecutionState, logger *logrus.Entry,
) (lib.Executor, error) {
	return PerVUIterations{
		BaseExecutor: NewBaseExecutor(pvic, es, logger),
		config:       pvic,
	}, nil
}

// HasWork reports whether there is any work to be done for the given execution segment.
func (pvic PerVUIterationsConfig) HasWork(et *lib.ExecutionTuple) bool {
	return pvic.GetVUs(et) > 0 && pvic.GetIterations() > 0
}

// PerVUIterations executes a specific number of iterations with each VU.
type PerVUIterations struct {
	*BaseExecutor
	config PerVUIterationsConfig
}

// Make sure we implement the lib.Executor interface.
var _ lib.Executor = &PerVUIterations{}

// Run executes a specific number of iterations with each configured VU.
//
//nolint:funlen
func (pvi PerVUIterations) Run(parentCtx context.Context, out chan<- metrics.SampleContainer) (err error) {
	numVUs := pvi.config.GetVUs(pvi.executionState.ExecutionTuple)
	iterations := pvi.config.GetIterations()
	duration := pvi.config.MaxDuration.TimeDuration()
	gracefulStop := pvi.config.GetGracefulStop()

	waitOnProgressChannel := make(chan struct{})
	startTime, maxDurationCtx, regDurationCtx, cancel := getDurationContexts(parentCtx, duration, gracefulStop)
	defer func() {
		cancel()
		<-waitOnProgressChannel
	}()

	// Make sure the log and the progress bar have accurate information
	pvi.logger.WithFields(logrus.Fields{
		"vus": numVUs, "iterations": iterations, "maxDuration": duration, "type": pvi.config.GetType(),
	}).Debug("Starting executor run...")

	totalIters := numVUs * iterations
	doneIters := new(uint64)

	vusFmt := pb.GetFixedLengthIntFormat(numVUs)
	itersFmt := pb.GetFixedLengthIntFormat(totalIters)
	progressFn := func() (float64, []string) {
		spent := time.Since(startTime)
		progVUs := fmt.Sprintf(vusFmt+" VUs", numVUs)
		currentDoneIters := atomic.LoadUint64(doneIters)
		progIters := fmt.Sprintf(itersFmt+"/"+itersFmt+" iters, %d per VU",
			currentDoneIters, totalIters, iterations)
		right := []string{progVUs, duration.String(), progIters}
		if spent > duration {
			return 1, right
		}

		spentDuration := pb.GetFixedLengthDuration(spent, duration)
		progDur := fmt.Sprintf("%s/%s", spentDuration, duration)
		right[1] = progDur

		return float64(currentDoneIters) / float64(totalIters), right
	}
	pvi.progress.Modify(pb.WithProgress(progressFn))

	maxDurationCtx = lib.WithScenarioState(maxDurationCtx, &lib.ScenarioState{
		Name:       pvi.config.Name,
		Executor:   pvi.config.Type,
		StartTime:  startTime,
		ProgressFn: progressFn,
	})
	go func() {
		trackProgress(parentCtx, maxDurationCtx, regDurationCtx, pvi, progressFn)
		close(waitOnProgressChannel)
	}()

	handleVUsWG := &sync.WaitGroup{}
	defer handleVUsWG.Wait()
	// Actually schedule the VUs and iterations...
	activeVUs := &sync.WaitGroup{}
	defer activeVUs.Wait()

	regDurationDone := regDurationCtx.Done()
	runIteration := getIterationRunner(pvi.executionState, pvi.logger)

	returnVU := func(u lib.InitializedVU) {
		pvi.executionState.ReturnVU(u, true)
		activeVUs.Done()
	}

	droppedIterationMetric := pvi.executionState.Test.BuiltinMetrics.DroppedIterations
	handleVU := func(initVU lib.InitializedVU) {
		defer handleVUsWG.Done()
		ctx, cancel := context.WithCancel(maxDurationCtx)
		defer cancel()

		vuID := initVU.GetID()
		activeVU := initVU.Activate(
			getVUActivationParams(ctx, pvi.config.BaseConfig, returnVU,
				pvi.nextIterationCounters))

		for i := int64(0); i < iterations; i++ {
			select {
			case <-regDurationDone:
				metrics.PushIfNotDone(parentCtx, out, metrics.Sample{
					TimeSeries: metrics.TimeSeries{
						Metric: droppedIterationMetric,
						Tags:   pvi.getMetricTags(&vuID),
					},
					Time:  time.Now(),
					Value: float64(iterations - i),
				})
				return // don't make more iterations
			default:
				// continue looping
			}
			runIteration(maxDurationCtx, activeVU)
			atomic.AddUint64(doneIters, 1)
		}
	}

	for i := int64(0); i < numVUs; i++ {
		initializedVU, err := pvi.executionState.GetPlannedVU(pvi.logger, true)
		if err != nil {
			cancel()
			return err
		}
		activeVUs.Add(1)
		handleVUsWG.Add(1)
		go handleVU(initializedVU)
	}

	return nil
}
