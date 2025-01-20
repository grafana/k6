package executor

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/sirupsen/logrus"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/internal/execution"
	"go.k6.io/k6/internal/ui/pb"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
)

const (
	// maxConcurrentVUs is an arbitrary limit for sanity checks.
	// It prevents running an exaggeratedly large number of concurrent VUs which may lead to an out-of-memory.
	maxConcurrentVUs int = 100_000_000
)

func sumStagesDuration(stages []Stage) (result time.Duration) {
	for _, s := range stages {
		result += s.Duration.TimeDuration()
	}
	return
}

func getStagesUnscaledMaxTarget(unscaledStartValue int64, stages []Stage) int64 {
	result := unscaledStartValue
	for _, s := range stages {
		if s.Target.Int64 > result {
			result = s.Target.Int64
		}
	}
	return result
}

// validateTargetShifts validates the VU Target shifts.
// It will append an error for any VU target that is larger than the maximum value allowed.
// Each Stage needs a Target value. The stages array can be empty. The Targes could be negative.
func validateTargetShifts(startVUs int64, stages []Stage) []error {
	var errors []error

	if startVUs > int64(maxConcurrentVUs) {
		errors = append(errors, fmt.Errorf(
			"the startVUs exceed max limit of %d", maxConcurrentVUs))
	}

	for i := 0; i < len(stages); i++ {
		if stages[i].Target.Int64 > int64(maxConcurrentVUs) {
			errors = append(errors, fmt.Errorf(
				"target for stage %d exceeds max limit of %d", i+1, maxConcurrentVUs))
		}
	}

	return errors
}

// A helper function to avoid code duplication
func validateStages(stages []Stage) []error {
	var errors []error
	if len(stages) == 0 {
		errors = append(errors, fmt.Errorf("at least one stage has to be specified"))
		return errors
	}

	for i, s := range stages {
		stageNum := i + 1
		if !s.Duration.Valid {
			errors = append(errors, fmt.Errorf("stage %d doesn't have a duration", stageNum))
		} else if s.Duration.Duration < 0 {
			errors = append(errors, fmt.Errorf("the duration for stage %d can't be negative", stageNum))
		}
		if !s.Target.Valid {
			errors = append(errors, fmt.Errorf("stage %d doesn't have a target", stageNum))
		} else if s.Target.Int64 < 0 {
			errors = append(errors, fmt.Errorf("the target for stage %d can't be negative", stageNum))
		}
	}
	return errors
}

// handleInterrupt returns true if err is InterruptError and if so it
// cancels the executor context passed with ctx.
func handleInterrupt(ctx context.Context, err error) bool {
	if err != nil {
		if errext.IsInterruptError(err) {
			execution.AbortTestRun(ctx, err)
			return true
		}
	}
	return false
}

// getIterationRunner is a helper function that returns an iteration executor
// closure. It takes care of updating the execution state statistics and
// warning messages. And returns whether a full iteration was finished or not
//
// TODO: emit the end-of-test iteration metrics here (https://github.com/k6io/k6/issues/1250)
func getIterationRunner(
	executionState *lib.ExecutionState, logger *logrus.Entry,
) func(context.Context, lib.ActiveVU) bool {
	return func(ctx context.Context, vu lib.ActiveVU) bool {
		err := vu.RunOnce()

		// TODO: track (non-ramp-down) errors from script iterations as a metric,
		// and have a default threshold that will abort the script when the error
		// rate exceeds a certain percentage

		select {
		case <-ctx.Done():
			// Don't log errors or emit iterations metrics from cancelled iterations
			executionState.AddInterruptedIterations(1)
			return false
		default:
			if err != nil {
				if handleInterrupt(ctx, err) {
					executionState.AddInterruptedIterations(1)
					return false
				}

				var exception errext.Exception
				if errors.As(err, &exception) {
					// TODO don't count this as a full iteration?
					logger.WithField("source", "stacktrace").Error(exception.StackTrace())
				} else {
					logger.Error(err.Error())
				}
				// TODO: investigate context cancelled errors
			}

			// TODO: move emission of end-of-iteration metrics here?
			executionState.AddFullIterations(1)
			return true
		}
	}
}

// getDurationContexts is used to create sub-contexts that can restrict an
// executor to only run for its allotted time.
//
// If the executor doesn't have a graceful stop period for iterations, then
// both returned sub-contexts will be the same one, with a timeout equal to
// the supplied regular executor duration.
//
// But if a graceful stop is enabled, then the first returned context (and the
// cancel func) will be for the "outer" sub-context. Its timeout will include
// both the regular duration and the specified graceful stop period. The second
// context will be a sub-context of the first one and its timeout will include
// only the regular duration.
//
// In either case, the usage of these contexts should be like this:
//   - As long as the regDurationCtx isn't done, new iterations can be started.
//   - After regDurationCtx is done, no new iterations should be started; every
//     VU that finishes an iteration from now on can be returned to the buffer
//     pool in the ExecutionState struct.
//   - After maxDurationCtx is done, any VUs with iterations will be
//     interrupted by the context's closing and will be returned to the buffer.
//   - If you want to interrupt the execution of all VUs prematurely (e.g. there
//     was an error or something like that), trigger maxDurationCancel().
//   - If the whole test is aborted, the parent context will be cancelled, so
//     that will also cancel these contexts, thus the "general abort" case is
//     handled transparently.
func getDurationContexts(parentCtx context.Context, regularDuration, gracefulStop time.Duration) (
	startTime time.Time, maxDurationCtx, regDurationCtx context.Context, maxDurationCancel func(),
) {
	startTime = time.Now()
	maxEndTime := startTime.Add(regularDuration + gracefulStop)

	maxDurationCtx, maxDurationCancel = context.WithDeadline(parentCtx, maxEndTime)
	if gracefulStop == 0 {
		return startTime, maxDurationCtx, maxDurationCtx, maxDurationCancel
	}
	regDurationCtx, _ = context.WithDeadline(maxDurationCtx, startTime.Add(regularDuration)) //nolint:govet
	return startTime, maxDurationCtx, regDurationCtx, maxDurationCancel
}

// trackProgress is a helper function that monitors certain end-events in an
// executor and updates its progressbar accordingly.
func trackProgress(
	parentCtx, maxDurationCtx, regDurationCtx context.Context,
	exec lib.Executor, snapshot func() (float64, []string),
) {
	progressBar := exec.GetProgress()
	logger := exec.GetLogger()

	<-regDurationCtx.Done() // Wait for the regular context to be over
	gracefulStop := exec.GetConfig().GetGracefulStop()
	if parentCtx.Err() == nil && gracefulStop > 0 {
		p, right := snapshot()
		logger.WithField("gracefulStop", gracefulStop).Debug(
			"Regular duration is done, waiting for iterations to gracefully finish",
		)
		progressBar.Modify(
			pb.WithStatus(pb.Stopping),
			pb.WithConstProgress(p, right...),
		)
	}

	<-maxDurationCtx.Done()
	p, right := snapshot()
	constProg := pb.WithConstProgress(p, right...)
	select {
	case <-parentCtx.Done():
		progressBar.Modify(pb.WithStatus(pb.Interrupted), constProg)
	default:
		status := pb.WithStatus(pb.Done)
		if p < 1 {
			status = pb.WithStatus(pb.Interrupted)
		}
		progressBar.Modify(status, constProg)
	}
}

// getScaledArrivalRate returns a rational number containing the scaled value of
// the given rate over the given period. This should generally be the first
// function that's called, before we do any calculations with the users-supplied
// rates in the arrival-rate executors.
func getScaledArrivalRate(es *lib.ExecutionSegment, rate int64, period time.Duration) *big.Rat {
	return es.InPlaceScaleRat(big.NewRat(rate, int64(period)))
}

// getTickerPeriod is just a helper function that returns the ticker interval
// we need for given arrival-rate parameters.
//
// It's possible for this function to return a zero duration (i.e. valid=false)
// and 0 isn't a valid ticker period. This happens so we don't divide by 0 when
// the arrival-rate period is 0. This case has to be handled separately.
func getTickerPeriod(scaledArrivalRate *big.Rat) types.NullDuration {
	if scaledArrivalRate.Sign() == 0 {
		return types.NewNullDuration(0, false)
	}
	// Basically, the ticker rate is time.Duration(1/arrivalRate). Considering
	// that time.Duration is represented as int64 nanoseconds, no meaningful
	// precision is likely to be lost here...
	result, _ := new(big.Rat).Inv(scaledArrivalRate).Float64()
	return types.NewNullDuration(time.Duration(result), true)
}

// getArrivalRatePerSec returns the iterations per second rate.
func getArrivalRatePerSec(scaledArrivalRate *big.Rat) *big.Rat {
	perSecRate := big.NewRat(int64(time.Second), 1)
	return perSecRate.Mul(perSecRate, scaledArrivalRate)
}

// TODO: Refactor this, maybe move all scenario things to an embedded struct?
func getVUActivationParams(
	ctx context.Context, conf BaseConfig, deactivateCallback func(lib.InitializedVU),
	nextIterationCounters func() (uint64, uint64),
) *lib.VUActivationParams {
	return &lib.VUActivationParams{
		RunContext:               ctx,
		Scenario:                 conf.Name,
		Exec:                     conf.GetExec(),
		Env:                      conf.GetEnv(),
		Tags:                     conf.GetTags(),
		DeactivateCallback:       deactivateCallback,
		GetNextIterationCounters: nextIterationCounters,
	}
}
