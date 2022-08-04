package executor

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/ui/pb"
)

const rampingArrivalRateType = "ramping-arrival-rate"

func init() {
	lib.RegisterExecutorConfigType(
		rampingArrivalRateType,
		func(name string, rawJSON []byte) (lib.ExecutorConfig, error) {
			config := NewRampingArrivalRateConfig(name)
			err := lib.StrictJSONUnmarshal(rawJSON, &config)
			return config, err
		},
	)
}

// RampingArrivalRateConfig stores config for the ramping (i.e. variable)
// arrival-rate executor.
type RampingArrivalRateConfig struct {
	BaseConfig
	StartRate null.Int           `json:"startRate"`
	TimeUnit  types.NullDuration `json:"timeUnit"`
	Stages    []Stage            `json:"stages"`

	// Initialize `PreAllocatedVUs` number of VUs, and if more than that are needed,
	// they will be dynamically allocated, until `MaxVUs` is reached, which is an
	// absolutely hard limit on the number of VUs the executor will use
	PreAllocatedVUs null.Int `json:"preAllocatedVUs"`
	MaxVUs          null.Int `json:"maxVUs"`
}

// NewRampingArrivalRateConfig returns a RampingArrivalRateConfig with default values
func NewRampingArrivalRateConfig(name string) *RampingArrivalRateConfig {
	return &RampingArrivalRateConfig{
		BaseConfig: NewBaseConfig(name, rampingArrivalRateType),
		TimeUnit:   types.NewNullDuration(1*time.Second, false),
	}
}

// Make sure we implement the lib.ExecutorConfig interface
var _ lib.ExecutorConfig = &RampingArrivalRateConfig{}

// GetPreAllocatedVUs is just a helper method that returns the scaled pre-allocated VUs.
func (varc RampingArrivalRateConfig) GetPreAllocatedVUs(et *lib.ExecutionTuple) int64 {
	return et.ScaleInt64(varc.PreAllocatedVUs.Int64)
}

// GetMaxVUs is just a helper method that returns the scaled max VUs.
func (varc RampingArrivalRateConfig) GetMaxVUs(et *lib.ExecutionTuple) int64 {
	return et.ScaleInt64(varc.MaxVUs.Int64)
}

// GetDescription returns a human-readable description of the executor options
func (varc RampingArrivalRateConfig) GetDescription(et *lib.ExecutionTuple) string {
	// TODO: something better? always show iterations per second?
	maxVUsRange := fmt.Sprintf("maxVUs: %d", et.ScaleInt64(varc.PreAllocatedVUs.Int64))
	if varc.MaxVUs.Int64 > varc.PreAllocatedVUs.Int64 {
		maxVUsRange += fmt.Sprintf("-%d", et.ScaleInt64(varc.MaxVUs.Int64))
	}
	maxUnscaledRate := getStagesUnscaledMaxTarget(varc.StartRate.Int64, varc.Stages)
	maxArrRatePerSec, _ := getArrivalRatePerSec(
		getScaledArrivalRate(et.Segment, maxUnscaledRate, varc.TimeUnit.TimeDuration()),
	).Float64()

	return fmt.Sprintf("Up to %.2f iterations/s for %s over %d stages%s",
		maxArrRatePerSec, sumStagesDuration(varc.Stages),
		len(varc.Stages), varc.getBaseInfo(maxVUsRange))
}

// Validate makes sure all options are configured and valid
func (varc *RampingArrivalRateConfig) Validate() []error {
	errors := varc.BaseConfig.Validate()

	if varc.StartRate.Int64 < 0 {
		errors = append(errors, fmt.Errorf("the startRate value can't be negative"))
	}

	if varc.TimeUnit.TimeDuration() < 0 {
		errors = append(errors, fmt.Errorf("the timeUnit must be more than 0"))
	}

	errors = append(errors, validateStages(varc.Stages)...)

	if !varc.PreAllocatedVUs.Valid {
		errors = append(errors, fmt.Errorf("the number of preAllocatedVUs isn't specified"))
	} else if varc.PreAllocatedVUs.Int64 < 0 {
		errors = append(errors, fmt.Errorf("the number of preAllocatedVUs can't be negative"))
	}

	if !varc.MaxVUs.Valid {
		// TODO: don't change the config while validating
		varc.MaxVUs.Int64 = varc.PreAllocatedVUs.Int64
	} else if varc.MaxVUs.Int64 < varc.PreAllocatedVUs.Int64 {
		errors = append(errors, fmt.Errorf("maxVUs can't be less than preAllocatedVUs"))
	}

	return errors
}

// GetExecutionRequirements returns the number of required VUs to run the
// executor for its whole duration (disregarding any startTime), including the
// maximum waiting time for any iterations to gracefully stop. This is used by
// the execution scheduler in its VU reservation calculations, so it knows how
// many VUs to pre-initialize.
func (varc RampingArrivalRateConfig) GetExecutionRequirements(et *lib.ExecutionTuple) []lib.ExecutionStep {
	return []lib.ExecutionStep{
		{
			TimeOffset:      0,
			PlannedVUs:      uint64(et.ScaleInt64(varc.PreAllocatedVUs.Int64)),
			MaxUnplannedVUs: uint64(et.ScaleInt64(varc.MaxVUs.Int64 - varc.PreAllocatedVUs.Int64)),
		},
		{
			TimeOffset:      sumStagesDuration(varc.Stages) + varc.GracefulStop.TimeDuration(),
			PlannedVUs:      0,
			MaxUnplannedVUs: 0,
		},
	}
}

// NewExecutor creates a new RampingArrivalRate executor
func (varc RampingArrivalRateConfig) NewExecutor(
	es *lib.ExecutionState, logger *logrus.Entry,
) (lib.Executor, error) {
	return &RampingArrivalRate{
		BaseExecutor: NewBaseExecutor(&varc, es, logger),
		config:       varc,
	}, nil
}

// HasWork reports whether there is any work to be done for the given execution segment.
func (varc RampingArrivalRateConfig) HasWork(et *lib.ExecutionTuple) bool {
	return varc.GetMaxVUs(et) > 0
}

// RampingArrivalRate tries to execute a specific number of iterations for a
// specific period.
// TODO: combine with the ConstantArrivalRate?
type RampingArrivalRate struct {
	*BaseExecutor
	config RampingArrivalRateConfig
	et     *lib.ExecutionTuple
}

// Make sure we implement the lib.Executor interface.
var _ lib.Executor = &RampingArrivalRate{}

// Init values needed for the execution
func (varr *RampingArrivalRate) Init(ctx context.Context) error {
	// err should always be nil, because Init() won't be called for executors
	// with no work, as determined by their config's HasWork() method.
	et, err := varr.BaseExecutor.executionState.ExecutionTuple.GetNewExecutionTupleFromValue(varr.config.MaxVUs.Int64)
	varr.et = et
	varr.iterSegIndex = lib.NewSegmentedIndex(et)

	return err //nolint:wrapcheck
}

// cal calculates the  transtitions between stages and gives the next full value produced by the
// stages. In this explanation we are talking about events and in practice those events are starting
// of an iteration, but could really be anything that needs to occur at a constant or linear rate.
//
// The basic idea is that we make a graph with the X axis being time and the Y axis being
// events/s we know that the area of the figure between the graph and the X axis is equal to the
// amount of events done - we multiply time by events per time so we get events ...
// Mathematics :).
//
// Lets look at a simple example - lets say we start with 2 events and the first stage is 5
// seconds to 2 events/s and then we have a second stage for 5 second that goes up to 3 events
// (using small numbers because ... well it is easier :D). This will look something like:
//  ^
// 7|
// 6|
// 5|
// 4|
// 3|       ,-+
// 2|----+-'  |
// 1|    |    |
//  +----+----+---------------------------------->
//  0s   5s   10s
// TODO: bigger and more stages
//
// Now the question is when(where on the graph) does the first event happen? Well in this simple
// case it is easy it will be at 0.5 seconds as we are doing 2 events/s. If we want to know when
// event n will happen we need to calculate n = 2 * x, where x is the time it will happen, so we
// need to calculate x = n/2as we are interested in the time, x.
// So if we just had a constant function for each event n we can calculate n/2 and find out when
// it needs to start.
// As we can see though the graph changes as stages change. But we can calculate how many events
// each stage will have, again it is the area from the start of the stage to it's end and between
// the graph and the X axis. So in this case we know that the first stage will have 10 full events
// in it and no more or less. So we are trying to find out when the 12 event will happen the answer
// will be after the 5th second.
//
// The graph doesn't show this well but we are ramping up linearly (we could possibly add
// other ramping up/down functions later). So at 7.5 seconds for example we should be doing 2.5
// events/s. You could start slicing the graph constantly and in this way to represent the ramping
// up/down as a multiple constant functions, and you will get mostly okayish results. But here is
// where calculus comes into play. Calculus gives us a way of exactly calculate the area for any
// given function and linear ramp up/downs just happen to be pretty easy(actual math prove in
// https://github.com/k6io/k6/issues/1299#issuecomment-575661084).
//
// One tricky last point is what happens if stage only completes 9.8 events? Let's say that the
// first stage above was 4.9 seconds long 2 * 4.9 is 9.8, we have 9 events and .8 of an event, what
// do with do with that? Well the 10th even will happen in the next stage (if any) and will happen
// when the are from the start till time x is 0.2 (instead of 1) as 0.2 + 0.8 is 10. So the 12th for
// example will be when the area is 2.2 as 9.8+2.2. So we just carry this around.
//
// So in the end what calis doing is to get formulas which will tell it when
// a given event n in order will happen. It helps itself by knowing that in a given
// stage will do some given amount (the area of the stage) events and if we past that one we
// know we are not in that stage.
//
// The specific implementation here can only go forward and does incorporate
// the striping algorithm from the lib.ExecutionTuple for additional speed up but this could
// possibly be refactored if need for this arises.
func (varc RampingArrivalRateConfig) cal(et *lib.ExecutionTuple, ch chan<- time.Duration) {
	start, offsets, _ := et.GetStripedOffsets()
	li := -1
	// TODO: move this to a utility function, or directly what GetStripedOffsets uses once we see everywhere we will use it
	next := func() int64 {
		li++
		return offsets[li%len(offsets)]
	}
	defer close(ch) // TODO: maybe this is not a good design - closing a channel we get
	var (
		stageStart                   time.Duration
		timeUnit                     = float64(varc.TimeUnit.Duration)
		doneSoFar, endCount, to, dur float64
		from                         = float64(varc.StartRate.ValueOrZero()) / timeUnit
		// start .. starts at 0 but the algorithm works with area so we need to start from 1 not 0
		i = float64(start + 1)
	)

	for _, stage := range varc.Stages {
		to = float64(stage.Target.ValueOrZero()) / timeUnit
		dur = float64(stage.Duration.Duration)
		if from != to { // ramp up/down
			endCount += dur * ((to-from)/2 + from)
			for ; i <= endCount; i += float64(next()) {
				// TODO: try to twist this in a way to be able to get i (the only changing part)
				// somewhere where it is less in the middle of the equation
				x := (from*dur - noNegativeSqrt(dur*(from*from*dur+2*(i-doneSoFar)*(to-from)))) / (from - to)

				ch <- time.Duration(x) + stageStart
			}
		} else {
			endCount += dur * to
			for ; i <= endCount; i += float64(next()) {
				ch <- time.Duration((i-doneSoFar)/to) + stageStart
			}
		}
		doneSoFar = endCount
		from = to
		stageStart += stage.Duration.TimeDuration()
	}
}

// This is needed because, on some platforms (arm64), sometimes, even though we
// in *reality* don't get negative results due to the nature of how float64 is
// implemented, we get negative values (very close to the 0). This would get an
// sqrt which is *even* smaller and likely will have negligible effects on the
// final result.
//
// TODO: this is probably going to be less necessary if we do some kind of of
// optimization above and the operations with the float64 are more "accurate"
// even on arm platforms.
func noNegativeSqrt(f float64) float64 {
	if !math.Signbit(f) {
		return math.Sqrt(f)
	}

	return 0
}

// Run executes a variable number of iterations per second.
//
// TODO: Split this up and make an independent component that can be reused
// between the constant and ramping arrival rate executors - that way we can
// keep the complexity in one well-architected part (with short methods and few
// lambdas :D), while having both config frontends still be present for maximum
// UX benefits. Basically, keep the progress bars and scheduling (i.e. at what
// time should iteration X begin) different, but keep everyhing else the same.
// This will allow us to implement https://github.com/k6io/k6/issues/1386
// and things like all of the TODOs below in one place only.
//nolint:funlen,cyclop
func (varr RampingArrivalRate) Run(parentCtx context.Context, out chan<- metrics.SampleContainer) (err error) {
	segment := varr.executionState.ExecutionTuple.Segment
	gracefulStop := varr.config.GetGracefulStop()
	duration := sumStagesDuration(varr.config.Stages)
	preAllocatedVUs := varr.config.GetPreAllocatedVUs(varr.executionState.ExecutionTuple)
	maxVUs := varr.config.GetMaxVUs(varr.executionState.ExecutionTuple)

	// TODO: refactor and simplify
	timeUnit := varr.config.TimeUnit.TimeDuration()
	startArrivalRate := getScaledArrivalRate(segment, varr.config.StartRate.Int64, timeUnit)
	maxUnscaledRate := getStagesUnscaledMaxTarget(varr.config.StartRate.Int64, varr.config.Stages)
	maxArrivalRatePerSec, _ := getArrivalRatePerSec(getScaledArrivalRate(segment, maxUnscaledRate, timeUnit)).Float64()
	startTickerPeriod := getTickerPeriod(startArrivalRate)

	// Make sure the log and the progress bar have accurate information
	varr.logger.WithFields(logrus.Fields{
		"maxVUs": maxVUs, "preAllocatedVUs": preAllocatedVUs, "duration": duration, "numStages": len(varr.config.Stages),
		"startTickerPeriod": startTickerPeriod.Duration, "type": varr.config.GetType(),
	}).Debug("Starting executor run...")

	activeVUsWg := &sync.WaitGroup{}

	returnedVUs := make(chan struct{})
	waitOnProgressChannel := make(chan struct{})
	startTime, maxDurationCtx, regDurationCtx, cancel := getDurationContexts(parentCtx, duration, gracefulStop)

	vusPool := newActiveVUPool()

	defer func() {
		// Make sure all VUs aren't executing iterations anymore, for the cancel()
		// below to deactivate them.
		<-returnedVUs
		// first close the vusPool so we wait for the gracefulShutdown
		vusPool.Close()
		cancel()
		activeVUsWg.Wait()
		<-waitOnProgressChannel
	}()

	activeVUsCount := uint64(0)
	tickerPeriod := int64(startTickerPeriod.Duration)
	vusFmt := pb.GetFixedLengthIntFormat(maxVUs)
	itersFmt := pb.GetFixedLengthFloatFormat(maxArrivalRatePerSec, 2) + " iters/s"

	progressFn := func() (float64, []string) {
		currActiveVUs := atomic.LoadUint64(&activeVUsCount)
		currentTickerPeriod := atomic.LoadInt64(&tickerPeriod)
		progVUs := fmt.Sprintf(vusFmt+"/"+vusFmt+" VUs",
			vusPool.Running(), currActiveVUs)

		itersPerSec := 0.0
		if currentTickerPeriod > 0 {
			itersPerSec = float64(time.Second) / float64(currentTickerPeriod)
		}
		progIters := fmt.Sprintf(itersFmt, itersPerSec)

		right := []string{progVUs, duration.String(), progIters}

		spent := time.Since(startTime)
		if spent > duration {
			return 1, right
		}

		spentDuration := pb.GetFixedLengthDuration(spent, duration)
		progDur := fmt.Sprintf("%s/%s", spentDuration, duration)
		right[1] = progDur

		return math.Min(1, float64(spent)/float64(duration)), right
	}

	varr.progress.Modify(pb.WithProgress(progressFn))
	maxDurationCtx = lib.WithScenarioState(maxDurationCtx, &lib.ScenarioState{
		Name:       varr.config.Name,
		Executor:   varr.config.Type,
		StartTime:  startTime,
		ProgressFn: progressFn,
	})
	go func() {
		trackProgress(parentCtx, maxDurationCtx, regDurationCtx, &varr, progressFn)
		close(waitOnProgressChannel)
	}()

	returnVU := func(u lib.InitializedVU) {
		varr.executionState.ReturnVU(u, true)
		activeVUsWg.Done()
	}

	runIterationBasic := getIterationRunner(varr.executionState, varr.logger)

	activateVU := func(initVU lib.InitializedVU) lib.ActiveVU {
		activeVUsWg.Add(1)
		activeVU := initVU.Activate(
			getVUActivationParams(
				maxDurationCtx, varr.config.BaseConfig, returnVU,
				varr.nextIterationCounters))
		varr.executionState.ModCurrentlyActiveVUsCount(+1)
		atomic.AddUint64(&activeVUsCount, 1)

		vusPool.AddVU(maxDurationCtx, activeVU, runIterationBasic)
		return activeVU
	}

	remainingUnplannedVUs := maxVUs - preAllocatedVUs
	makeUnplannedVUCh := make(chan struct{})
	defer close(makeUnplannedVUCh)
	go func() {
		defer close(returnedVUs)

		for range makeUnplannedVUCh {
			varr.logger.Debug("Starting initialization of an unplanned VU...")
			initVU, err := varr.executionState.GetUnplannedVU(maxDurationCtx, varr.logger)
			if err != nil {
				// TODO figure out how to return it to the Run goroutine
				varr.logger.WithError(err).Error("Error while allocating unplanned VU")
			} else {
				varr.logger.Debug("The unplanned VU finished initializing successfully!")
				activateVU(initVU)
			}
		}
	}()

	// Get the pre-allocated VUs in the local buffer
	for i := int64(0); i < preAllocatedVUs; i++ {
		initVU, err := varr.executionState.GetPlannedVU(varr.logger, false)
		if err != nil {
			return err
		}
		activateVU(initVU)
	}

	regDurationDone := regDurationCtx.Done()
	timer := time.NewTimer(time.Hour)
	start := time.Now()
	ch := make(chan time.Duration, 10) // buffer 10 iteration times ahead
	var prevTime time.Duration
	shownWarning := false
	metricTags := varr.getMetricTags(nil)
	go varr.config.cal(varr.et, ch)
	droppedIterationMetric := varr.executionState.Test.BuiltinMetrics.DroppedIterations
	for nextTime := range ch {
		select {
		case <-regDurationDone:
			return nil
		default:
		}
		atomic.StoreInt64(&tickerPeriod, int64(nextTime-prevTime))
		prevTime = nextTime
		b := time.Until(start.Add(nextTime))
		if b > 0 { // TODO: have a minimal ?
			timer.Reset(b)
			select {
			case <-timer.C:
			case <-regDurationDone:
				return nil
			}
		}

		if vusPool.TryRunIteration() {
			continue
		}

		// Since there aren't any free VUs available, consider this iteration
		// dropped - we aren't going to try to recover it, but

		metrics.PushIfNotDone(parentCtx, out, droppedIterationMetric.Sample(time.Now(), metricTags, 1))

		// We'll try to start allocating another VU in the background,
		// non-blockingly, if we have remainingUnplannedVUs...
		if remainingUnplannedVUs == 0 {
			if !shownWarning {
				varr.logger.Warningf("Insufficient VUs, reached %d active VUs and cannot initialize more", maxVUs)
				shownWarning = true
			}
			continue
		}

		select {
		case makeUnplannedVUCh <- struct{}{}: // great!
			remainingUnplannedVUs--
		default: // we're already allocating a new VU
		}
	}
	return nil
}

// activeVUPool controls the activeVUs
// executing the received requests for iterations.
type activeVUPool struct {
	iterations chan struct{}
	running    uint64
	wg         sync.WaitGroup
}

// newActiveVUPool returns an activeVUPool.
func newActiveVUPool() *activeVUPool {
	return &activeVUPool{
		iterations: make(chan struct{}),
	}
}

// TryRunIteration invokes a request to execute a new iteration.
// When there are no available VUs to process the request
// then false is returned.
func (p *activeVUPool) TryRunIteration() bool {
	select {
	case p.iterations <- struct{}{}:
		return true
	default:
		return false
	}
}

// Running returns the number of the currently running VUs.
func (p *activeVUPool) Running() uint64 {
	return atomic.LoadUint64(&p.running)
}

// AddVU adds the active VU to the pool of VUs for handling the incoming requests.
// When a new request is accepted the runfn function is executed.
func (p *activeVUPool) AddVU(ctx context.Context, avu lib.ActiveVU, runfn func(context.Context, lib.ActiveVU) bool) {
	p.wg.Add(1)
	ch := make(chan struct{})
	go func() {
		defer p.wg.Done()

		close(ch)
		for range p.iterations {
			atomic.AddUint64(&p.running, uint64(1))
			runfn(ctx, avu)
			atomic.AddUint64(&p.running, ^uint64(0))
		}
	}()
	<-ch
}

// Close stops the pool from accepting requests
// then it will wait for all on-going iterations to complete.
func (p *activeVUPool) Close() {
	close(p.iterations)
	p.wg.Wait()
}
