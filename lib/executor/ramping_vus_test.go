package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
)

func TestRampingVUsConfigValidation(t *testing.T) {
	t.Parallel()
	const maxConcurrentVUs = 100_000_000

	t.Run("no stages", func(t *testing.T) {
		t.Parallel()
		errs := NewRampingVUsConfig("default").Validate()
		require.NotEmpty(t, errs)
		assert.Contains(t, errs[0].Error(), "one stage has to be specified")
	})
	t.Run("basic 1 stage", func(t *testing.T) {
		t.Parallel()
		c := NewRampingVUsConfig("stage")
		c.Stages = []Stage{
			{Target: null.IntFrom(0), Duration: types.NullDurationFrom(12 * time.Second)},
		}
		errs := c.Validate()
		require.Empty(t, errs) // by default StartVUs is 1

		c.StartVUs = null.IntFrom(0)
		errs = c.Validate()
		require.NotEmpty(t, errs)
		assert.Contains(t, errs[0].Error(), "greater than 0")
	})

	t.Run("If startVUs are larger than maxConcurrentVUs, the validation should return an error", func(t *testing.T) {
		t.Parallel()

		c := NewRampingVUsConfig("stage")
		c.StartVUs = null.IntFrom(maxConcurrentVUs + 1)
		c.Stages = []Stage{
			{Target: null.IntFrom(0), Duration: types.NullDurationFrom(1 * time.Second)},
		}

		errs := c.Validate()
		require.NotEmpty(t, errs)
		assert.Contains(t, errs[0].Error(), "the startVUs exceed max limit of")
	})

	t.Run("For multiple VU values larger than maxConcurrentVUs, multiple errors are returned", func(t *testing.T) {
		t.Parallel()

		c := NewRampingVUsConfig("stage")
		c.StartVUs = null.IntFrom(maxConcurrentVUs + 1)
		c.Stages = []Stage{
			{Target: null.IntFrom(maxConcurrentVUs + 2), Duration: types.NullDurationFrom(1 * time.Second)},
		}

		errs := c.Validate()
		require.Equal(t, 2, len(errs))
		assert.Contains(t, errs[0].Error(), "the startVUs exceed max limit of")

		assert.Contains(t, errs[1].Error(), "target for stage 1 exceeds max limit of")
	})

	t.Run("VU values below maxConcurrentVUs will pass validation", func(t *testing.T) {
		t.Parallel()

		c := NewRampingVUsConfig("stage")
		c.StartVUs = null.IntFrom(0)
		c.Stages = []Stage{
			{Target: null.IntFrom(maxConcurrentVUs - 1), Duration: types.NullDurationFrom(1 * time.Second)},
		}

		errs := c.Validate()
		require.Empty(t, errs)
	})
}

func TestRampingVUsRun(t *testing.T) {
	t.Parallel()

	config := RampingVUsConfig{
		BaseConfig:       BaseConfig{GracefulStop: types.NullDurationFrom(0)},
		GracefulRampDown: types.NullDurationFrom(0),
		StartVUs:         null.IntFrom(5),
		Stages: []Stage{
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(5),
			},
			{
				Duration: types.NullDurationFrom(0),
				Target:   null.IntFrom(3),
			},
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(3),
			},
		},
	}

	var iterCount int64

	runner := simpleRunner(func(_ context.Context, _ *lib.State) error {
		// Sleeping for a weird duration somewhat offset from the
		// executor ticks to hopefully keep race conditions out of
		// our control from failing the test.
		time.Sleep(300 * time.Millisecond)
		atomic.AddInt64(&iterCount, 1)
		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, config)
	defer test.cancel()

	sampleTimes := []time.Duration{
		500 * time.Millisecond,
		1000 * time.Millisecond,
		900 * time.Millisecond,
	}

	errCh := make(chan error)
	go func() { errCh <- test.executor.Run(test.ctx, nil) }()

	result := make([]int64, len(sampleTimes))
	for i, d := range sampleTimes {
		time.Sleep(d)
		result[i] = test.state.GetCurrentlyActiveVUsCount()
	}

	require.NoError(t, <-errCh)

	assert.Equal(t, []int64{5, 3, 0}, result)
	assert.Equal(t, int64(29), atomic.LoadInt64(&iterCount))
}

func TestRampingVUsGracefulStopWaits(t *testing.T) {
	t.Parallel()

	config := RampingVUsConfig{
		BaseConfig: BaseConfig{GracefulStop: types.NullDurationFrom(time.Second)},
		StartVUs:   null.IntFrom(1),
		Stages: []Stage{
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(1),
			},
		},
	}

	var (
		started = make(chan struct{}) // the iteration started
		stopped = make(chan struct{}) // the iteration stopped
		stop    = make(chan struct{}) // the itearation should stop
	)

	runner := simpleRunner(func(ctx context.Context, _ *lib.State) error {
		close(started)
		defer close(stopped)
		select {
		case <-ctx.Done():
			t.Fatal("The iterations should've ended before the context")
		case <-stop:
		}
		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, config)
	defer test.cancel()

	errCh := make(chan error)
	go func() { errCh <- test.executor.Run(test.ctx, nil) }()

	<-started
	// 500 milliseconds more then the duration and 500 less then the gracefulStop
	time.Sleep(time.Millisecond * 1500)
	close(stop)
	<-stopped

	require.NoError(t, <-errCh)
}

func TestRampingVUsGracefulStopStops(t *testing.T) {
	t.Parallel()

	config := RampingVUsConfig{
		BaseConfig: BaseConfig{GracefulStop: types.NullDurationFrom(time.Second)},
		StartVUs:   null.IntFrom(1),
		Stages: []Stage{
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(1),
			},
		},
	}

	var (
		started = make(chan struct{}) // the iteration started
		stopped = make(chan struct{}) // the iteration stopped
		stop    = make(chan struct{}) // the itearation should stop
	)

	runner := simpleRunner(func(ctx context.Context, _ *lib.State) error {
		close(started)
		defer close(stopped)
		select {
		case <-ctx.Done():
		case <-stop:
			t.Fatal("The iterations shouldn't have ended before the context")
		}
		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, config)
	defer test.cancel()

	errCh := make(chan error)
	go func() { errCh <- test.executor.Run(test.ctx, nil) }()

	<-started
	// 500 milliseconds more then the gracefulStop + duration
	time.Sleep(time.Millisecond * 2500)
	close(stop)
	<-stopped

	require.NoError(t, <-errCh)
}

func TestRampingVUsGracefulRampDown(t *testing.T) {
	t.Parallel()

	config := RampingVUsConfig{
		BaseConfig:       BaseConfig{GracefulStop: types.NullDurationFrom(5 * time.Second)},
		StartVUs:         null.IntFrom(2),
		GracefulRampDown: types.NullDurationFrom(5 * time.Second),
		Stages: []Stage{
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(2),
			},
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(0),
			},
		},
	}

	var (
		started = make(chan struct{}) // the iteration started
		stopped = make(chan struct{}) // the iteration stopped
		stop    = make(chan struct{}) // the itearation should stop
	)

	runner := simpleRunner(func(ctx context.Context, state *lib.State) error {
		if state.VUID == 1 { // the first VU will wait here to do stuff
			close(started)
			defer close(stopped)
			select {
			case <-ctx.Done():
				t.Fatal("The iterations can't have ended before the context")
			case <-stop:
			}
		} else { // all other (1) VUs will just sleep long enough
			time.Sleep(2500 * time.Millisecond)
		}
		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, config)
	defer test.cancel()

	errCh := make(chan error)
	go func() { errCh <- test.executor.Run(test.ctx, nil) }()

	<-started
	// 500 milliseconds more then the gracefulRampDown + duration
	time.Sleep(2500 * time.Millisecond)
	close(stop)
	<-stopped

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(time.Second): // way too much time
		t.Fatal("Execution should've ended already")
	}
}

// This test aims to check whether the ramping VU executor interrupts
// hanging/remaining VUs after the graceful rampdown period finishes.
//
//	              Rampdown         Graceful Rampdown
//	              Stage (40ms)     (+30ms)
//	              [               ][              ]
//	    t 0---5---10---20---30---40---50---60---70
//	VU1       *..................................✔ (40+30=70ms)
//	VU2           *...................X            (20+30=50ms)
//
//	✔=Finishes,X=Interrupted,.=Sleeps
func TestRampingVUsHandleRemainingVUs(t *testing.T) {
	t.Parallel()

	const (
		maxVus                   = 2
		vuSleepDuration          = 65 * time.Millisecond // Each VU will sleep 65ms
		wantVuFinished    uint32 = 1                     // one VU should finish an iteration
		wantVuInterrupted uint32 = 1                     // one VU should be interrupted
	)

	cfg := RampingVUsConfig{
		BaseConfig: BaseConfig{
			// Extend the total test duration 50ms more
			//
			// test duration = sum(stages) + GracefulStop
			//
			// This could have been 30ms but increased it to 50ms
			// to prevent the test to become flaky.
			GracefulStop: types.NullDurationFrom(50 * time.Millisecond),
		},
		// Wait 30ms more for already started iterations
		// (Happens in the 2nd stage below: Graceful rampdown period)
		GracefulRampDown: types.NullDurationFrom(30 * time.Millisecond),
		// Total test duration is 50ms (excluding the GracefulRampdown period)
		Stages: []Stage{
			// Activate 2 VUs in 10ms
			{
				Duration: types.NullDurationFrom(10 * time.Millisecond),
				Target:   null.IntFrom(int64(maxVus)),
			},
			// Rampdown to 0 VUs in 40ms
			{
				Duration: types.NullDurationFrom(40 * time.Millisecond),
				Target:   null.IntFrom(int64(0)),
			},
		},
	}

	var (
		gotVuInterrupted uint32
		gotVuFinished    uint32
	)
	runner := simpleRunner(func(ctx context.Context, _ *lib.State) error {
		select {
		case <-time.After(vuSleepDuration):
			atomic.AddUint32(&gotVuFinished, 1)
		case <-ctx.Done():
			atomic.AddUint32(&gotVuInterrupted, 1)
		}
		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, cfg)
	defer test.cancel()

	// run the executor: this should finish in ~70ms
	// sum(stages) + GracefulRampDown
	require.NoError(t, test.executor.Run(test.ctx, nil))

	assert.Equal(t, wantVuInterrupted, atomic.LoadUint32(&gotVuInterrupted))
	assert.Equal(t, wantVuFinished, atomic.LoadUint32(&gotVuFinished))
}

// Ensure there's no wobble of VUs during graceful ramp-down, without segments.
// See https://github.com/k6io/k6/issues/1296
func TestRampingVUsRampDownNoWobble(t *testing.T) {
	t.Parallel()

	config := RampingVUsConfig{
		BaseConfig:       BaseConfig{GracefulStop: types.NullDurationFrom(0)},
		GracefulRampDown: types.NullDurationFrom(1 * time.Second),
		StartVUs:         null.IntFrom(0),
		Stages: []Stage{
			{
				Duration: types.NullDurationFrom(3 * time.Second),
				Target:   null.IntFrom(10),
			},
			{
				Duration: types.NullDurationFrom(2 * time.Second),
				Target:   null.IntFrom(0),
			},
		},
	}

	runner := simpleRunner(func(_ context.Context, _ *lib.State) error {
		time.Sleep(500 * time.Millisecond)
		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, config)
	defer test.cancel()

	sampleTimes := []time.Duration{
		100 * time.Millisecond,
		3000 * time.Millisecond,
	}
	const rampDownSampleTime = 50 * time.Millisecond
	rampDownSamples := int((config.Stages[len(config.Stages)-1].Duration.TimeDuration() + config.GracefulRampDown.TimeDuration()) / rampDownSampleTime)

	errCh := make(chan error)
	go func() { errCh <- test.executor.Run(test.ctx, nil) }()

	result := make([]int64, len(sampleTimes)+rampDownSamples)
	for i, d := range sampleTimes {
		time.Sleep(d)
		result[i] = test.state.GetCurrentlyActiveVUsCount()
	}

	// Sample ramp-down at a higher rate
	for i := len(sampleTimes); i < rampDownSamples; i++ {
		time.Sleep(rampDownSampleTime)
		result[i] = test.state.GetCurrentlyActiveVUsCount()
	}

	require.NoError(t, <-errCh)

	// Some baseline checks
	assert.Equal(t, int64(0), result[0])
	assert.Equal(t, int64(10), result[1])
	assert.Equal(t, int64(0), result[len(result)-1])

	vuChanges := []int64{result[2]}
	// Check ramp-down consistency
	for i := 3; i < len(result[2:]); i++ {
		if result[i] != result[i-1] {
			vuChanges = append(vuChanges, result[i])
		}
	}
	assert.Equal(t, []int64{10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0}, vuChanges)
}

func TestRampingVUsConfigExecutionPlanExample(t *testing.T) {
	t.Parallel()
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	conf := NewRampingVUsConfig("test")
	conf.StartVUs = null.IntFrom(4)
	conf.Stages = []Stage{
		{Target: null.IntFrom(6), Duration: types.NullDurationFrom(2 * time.Second)},
		{Target: null.IntFrom(1), Duration: types.NullDurationFrom(5 * time.Second)},
		{Target: null.IntFrom(5), Duration: types.NullDurationFrom(4 * time.Second)},
		{Target: null.IntFrom(1), Duration: types.NullDurationFrom(4 * time.Second)},
		{Target: null.IntFrom(4), Duration: types.NullDurationFrom(3 * time.Second)},
		{Target: null.IntFrom(4), Duration: types.NullDurationFrom(2 * time.Second)},
		{Target: null.IntFrom(1), Duration: types.NullDurationFrom(0 * time.Second)},
		{Target: null.IntFrom(1), Duration: types.NullDurationFrom(3 * time.Second)},
	}

	expRawStepsNoZeroEnd := []lib.ExecutionStep{
		{TimeOffset: 0 * time.Second, PlannedVUs: 4},
		{TimeOffset: 1 * time.Second, PlannedVUs: 5},
		{TimeOffset: 2 * time.Second, PlannedVUs: 6},
		{TimeOffset: 3 * time.Second, PlannedVUs: 5},
		{TimeOffset: 4 * time.Second, PlannedVUs: 4},
		{TimeOffset: 5 * time.Second, PlannedVUs: 3},
		{TimeOffset: 6 * time.Second, PlannedVUs: 2},
		{TimeOffset: 7 * time.Second, PlannedVUs: 1},
		{TimeOffset: 8 * time.Second, PlannedVUs: 2},
		{TimeOffset: 9 * time.Second, PlannedVUs: 3},
		{TimeOffset: 10 * time.Second, PlannedVUs: 4},
		{TimeOffset: 11 * time.Second, PlannedVUs: 5},
		{TimeOffset: 12 * time.Second, PlannedVUs: 4},
		{TimeOffset: 13 * time.Second, PlannedVUs: 3},
		{TimeOffset: 14 * time.Second, PlannedVUs: 2},
		{TimeOffset: 15 * time.Second, PlannedVUs: 1},
		{TimeOffset: 16 * time.Second, PlannedVUs: 2},
		{TimeOffset: 17 * time.Second, PlannedVUs: 3},
		{TimeOffset: 18 * time.Second, PlannedVUs: 4},
		{TimeOffset: 20 * time.Second, PlannedVUs: 1},
	}
	rawStepsNoZeroEnd := conf.getRawExecutionSteps(et, false)
	assert.Equal(t, expRawStepsNoZeroEnd, rawStepsNoZeroEnd)
	endOffset, isFinal := lib.GetEndOffset(rawStepsNoZeroEnd)
	assert.Equal(t, 20*time.Second, endOffset)
	assert.Equal(t, false, isFinal)

	rawStepsZeroEnd := conf.getRawExecutionSteps(et, true)
	assert.Equal(t,
		append(expRawStepsNoZeroEnd, lib.ExecutionStep{TimeOffset: 23 * time.Second, PlannedVUs: 0}),
		rawStepsZeroEnd,
	)
	endOffset, isFinal = lib.GetEndOffset(rawStepsZeroEnd)
	assert.Equal(t, 23*time.Second, endOffset)
	assert.Equal(t, true, isFinal)

	// GracefulStop and GracefulRampDown equal to the default 30 sec
	assert.Equal(t, []lib.ExecutionStep{
		{TimeOffset: 0 * time.Second, PlannedVUs: 4},
		{TimeOffset: 1 * time.Second, PlannedVUs: 5},
		{TimeOffset: 2 * time.Second, PlannedVUs: 6},
		{TimeOffset: 33 * time.Second, PlannedVUs: 5},
		{TimeOffset: 42 * time.Second, PlannedVUs: 4},
		{TimeOffset: 50 * time.Second, PlannedVUs: 1},
		{TimeOffset: 53 * time.Second, PlannedVUs: 0},
	}, conf.GetExecutionRequirements(et))

	// Try a longer GracefulStop than the GracefulRampDown
	conf.GracefulStop = types.NullDurationFrom(80 * time.Second)
	assert.Equal(t, []lib.ExecutionStep{
		{TimeOffset: 0 * time.Second, PlannedVUs: 4},
		{TimeOffset: 1 * time.Second, PlannedVUs: 5},
		{TimeOffset: 2 * time.Second, PlannedVUs: 6},
		{TimeOffset: 33 * time.Second, PlannedVUs: 5},
		{TimeOffset: 42 * time.Second, PlannedVUs: 4},
		{TimeOffset: 50 * time.Second, PlannedVUs: 1},
		{TimeOffset: 103 * time.Second, PlannedVUs: 0},
	}, conf.GetExecutionRequirements(et))

	// Try a much shorter GracefulStop than the GracefulRampDown
	conf.GracefulStop = types.NullDurationFrom(3 * time.Second)
	assert.Equal(t, []lib.ExecutionStep{
		{TimeOffset: 0 * time.Second, PlannedVUs: 4},
		{TimeOffset: 1 * time.Second, PlannedVUs: 5},
		{TimeOffset: 2 * time.Second, PlannedVUs: 6},
		{TimeOffset: 26 * time.Second, PlannedVUs: 0},
	}, conf.GetExecutionRequirements(et))

	// Try a zero GracefulStop
	conf.GracefulStop = types.NullDurationFrom(0 * time.Second)
	assert.Equal(t, []lib.ExecutionStep{
		{TimeOffset: 0 * time.Second, PlannedVUs: 4},
		{TimeOffset: 1 * time.Second, PlannedVUs: 5},
		{TimeOffset: 2 * time.Second, PlannedVUs: 6},
		{TimeOffset: 23 * time.Second, PlannedVUs: 0},
	}, conf.GetExecutionRequirements(et))

	// Try a zero GracefulStop and GracefulRampDown, i.e. raw steps with 0 end cap
	conf.GracefulRampDown = types.NullDurationFrom(0 * time.Second)
	assert.Equal(t, rawStepsZeroEnd, conf.GetExecutionRequirements(et))
}

func TestRampingVUsConfigExecutionPlanExampleOneThird(t *testing.T) {
	t.Parallel()
	et, err := lib.NewExecutionTuple(newExecutionSegmentFromString("0:1/3"), nil)
	require.NoError(t, err)
	conf := NewRampingVUsConfig("test")
	conf.StartVUs = null.IntFrom(4)
	conf.Stages = []Stage{
		{Target: null.IntFrom(6), Duration: types.NullDurationFrom(2 * time.Second)},
		{Target: null.IntFrom(1), Duration: types.NullDurationFrom(5 * time.Second)},
		{Target: null.IntFrom(5), Duration: types.NullDurationFrom(4 * time.Second)},
		{Target: null.IntFrom(1), Duration: types.NullDurationFrom(4 * time.Second)},
		{Target: null.IntFrom(4), Duration: types.NullDurationFrom(3 * time.Second)},
		{Target: null.IntFrom(4), Duration: types.NullDurationFrom(2 * time.Second)},
		{Target: null.IntFrom(1), Duration: types.NullDurationFrom(0 * time.Second)},
		{Target: null.IntFrom(1), Duration: types.NullDurationFrom(3 * time.Second)},
	}

	expRawStepsNoZeroEnd := []lib.ExecutionStep{
		{TimeOffset: 0 * time.Second, PlannedVUs: 1},
		{TimeOffset: 1 * time.Second, PlannedVUs: 2},
		{TimeOffset: 4 * time.Second, PlannedVUs: 1},
		{TimeOffset: 7 * time.Second, PlannedVUs: 0},
		{TimeOffset: 8 * time.Second, PlannedVUs: 1},
		{TimeOffset: 11 * time.Second, PlannedVUs: 2},
		{TimeOffset: 12 * time.Second, PlannedVUs: 1},
		{TimeOffset: 15 * time.Second, PlannedVUs: 0},
		{TimeOffset: 16 * time.Second, PlannedVUs: 1},
		{TimeOffset: 20 * time.Second, PlannedVUs: 0},
	}
	rawStepsNoZeroEnd := conf.getRawExecutionSteps(et, false)
	assert.Equal(t, expRawStepsNoZeroEnd, rawStepsNoZeroEnd)
	endOffset, isFinal := lib.GetEndOffset(rawStepsNoZeroEnd)
	assert.Equal(t, 20*time.Second, endOffset)
	assert.Equal(t, true, isFinal)

	rawStepsZeroEnd := conf.getRawExecutionSteps(et, true)
	assert.Equal(t, expRawStepsNoZeroEnd, rawStepsZeroEnd)
	endOffset, isFinal = lib.GetEndOffset(rawStepsZeroEnd)
	assert.Equal(t, 20*time.Second, endOffset)
	assert.Equal(t, true, isFinal)

	// GracefulStop and GracefulRampDown equal to the default 30 sec
	assert.Equal(t, []lib.ExecutionStep{
		{TimeOffset: 0 * time.Second, PlannedVUs: 1},
		{TimeOffset: 1 * time.Second, PlannedVUs: 2},
		{TimeOffset: 42 * time.Second, PlannedVUs: 1},
		{TimeOffset: 50 * time.Second, PlannedVUs: 0},
	}, conf.GetExecutionRequirements(et))

	// Try a longer GracefulStop than the GracefulRampDown
	conf.GracefulStop = types.NullDurationFrom(80 * time.Second)
	assert.Equal(t, []lib.ExecutionStep{
		{TimeOffset: 0 * time.Second, PlannedVUs: 1},
		{TimeOffset: 1 * time.Second, PlannedVUs: 2},
		{TimeOffset: 42 * time.Second, PlannedVUs: 1},
		{TimeOffset: 50 * time.Second, PlannedVUs: 0},
	}, conf.GetExecutionRequirements(et))

	// Try a much shorter GracefulStop than the GracefulRampDown
	conf.GracefulStop = types.NullDurationFrom(3 * time.Second)
	assert.Equal(t, []lib.ExecutionStep{
		{TimeOffset: 0 * time.Second, PlannedVUs: 1},
		{TimeOffset: 1 * time.Second, PlannedVUs: 2},
		{TimeOffset: 26 * time.Second, PlannedVUs: 0},
	}, conf.GetExecutionRequirements(et))

	// Try a zero GracefulStop
	conf.GracefulStop = types.NullDurationFrom(0 * time.Second)
	assert.Equal(t, []lib.ExecutionStep{
		{TimeOffset: 0 * time.Second, PlannedVUs: 1},
		{TimeOffset: 1 * time.Second, PlannedVUs: 2},
		{TimeOffset: 23 * time.Second, PlannedVUs: 0},
	}, conf.GetExecutionRequirements(et))

	// Try a zero GracefulStop and GracefulRampDown, i.e. raw steps with 0 end cap
	conf.GracefulRampDown = types.NullDurationFrom(0 * time.Second)
	assert.Equal(t, rawStepsZeroEnd, conf.GetExecutionRequirements(et))
}

func TestRampingVUsExecutionTupleTests(t *testing.T) {
	t.Parallel()

	conf := NewRampingVUsConfig("test")
	conf.StartVUs = null.IntFrom(4)
	conf.Stages = []Stage{
		{Target: null.IntFrom(6), Duration: types.NullDurationFrom(2 * time.Second)},
		{Target: null.IntFrom(1), Duration: types.NullDurationFrom(5 * time.Second)},
		{Target: null.IntFrom(5), Duration: types.NullDurationFrom(4 * time.Second)},
		{Target: null.IntFrom(1), Duration: types.NullDurationFrom(4 * time.Second)},
		{Target: null.IntFrom(4), Duration: types.NullDurationFrom(3 * time.Second)},
		{Target: null.IntFrom(4), Duration: types.NullDurationFrom(2 * time.Second)},
		{Target: null.IntFrom(1), Duration: types.NullDurationFrom(0 * time.Second)},
		{Target: null.IntFrom(1), Duration: types.NullDurationFrom(3 * time.Second)},
		{Target: null.IntFrom(5), Duration: types.NullDurationFrom(0 * time.Second)},
		{Target: null.IntFrom(5), Duration: types.NullDurationFrom(3 * time.Second)},
		{Target: null.IntFrom(0), Duration: types.NullDurationFrom(0 * time.Second)},
		{Target: null.IntFrom(2), Duration: types.NullDurationFrom(2 * time.Second)},
		{Target: null.IntFrom(0), Duration: types.NullDurationFrom(2 * time.Second)},
		{Target: null.IntFrom(4), Duration: types.NullDurationFrom(4 * time.Second)},
	}
	/*

			Graph of the above:
			^
		8	|
		7	|
		6	| +
		5	|/ \       +           +--+
		4	+   \     / \     +-+  |  |       *
		3	|    \   /   \   /  |  |  |      /
		2	|     \ /     \ /   |  |  | +   /
		1	|      +       +    +--+  |/ \ /
		0	+-------------------------+---+------------------------------>
		    01234567890123456789012345678901234567890

	*/

	testCases := []struct {
		expectedSteps []lib.ExecutionStep
		et            *lib.ExecutionTuple
	}{
		{
			et: mustNewExecutionTuple(nil, nil),
			expectedSteps: []lib.ExecutionStep{
				{TimeOffset: 0 * time.Second, PlannedVUs: 4},
				{TimeOffset: 1 * time.Second, PlannedVUs: 5},
				{TimeOffset: 2 * time.Second, PlannedVUs: 6},
				{TimeOffset: 3 * time.Second, PlannedVUs: 5},
				{TimeOffset: 4 * time.Second, PlannedVUs: 4},
				{TimeOffset: 5 * time.Second, PlannedVUs: 3},
				{TimeOffset: 6 * time.Second, PlannedVUs: 2},
				{TimeOffset: 7 * time.Second, PlannedVUs: 1},
				{TimeOffset: 8 * time.Second, PlannedVUs: 2},
				{TimeOffset: 9 * time.Second, PlannedVUs: 3},
				{TimeOffset: 10 * time.Second, PlannedVUs: 4},
				{TimeOffset: 11 * time.Second, PlannedVUs: 5},
				{TimeOffset: 12 * time.Second, PlannedVUs: 4},
				{TimeOffset: 13 * time.Second, PlannedVUs: 3},
				{TimeOffset: 14 * time.Second, PlannedVUs: 2},
				{TimeOffset: 15 * time.Second, PlannedVUs: 1},
				{TimeOffset: 16 * time.Second, PlannedVUs: 2},
				{TimeOffset: 17 * time.Second, PlannedVUs: 3},
				{TimeOffset: 18 * time.Second, PlannedVUs: 4},
				{TimeOffset: 20 * time.Second, PlannedVUs: 1},
				{TimeOffset: 23 * time.Second, PlannedVUs: 5},
				{TimeOffset: 26 * time.Second, PlannedVUs: 0},
				{TimeOffset: 27 * time.Second, PlannedVUs: 1},
				{TimeOffset: 28 * time.Second, PlannedVUs: 2},
				{TimeOffset: 29 * time.Second, PlannedVUs: 1},
				{TimeOffset: 30 * time.Second, PlannedVUs: 0},
				{TimeOffset: 31 * time.Second, PlannedVUs: 1},
				{TimeOffset: 32 * time.Second, PlannedVUs: 2},
				{TimeOffset: 33 * time.Second, PlannedVUs: 3},
				{TimeOffset: 34 * time.Second, PlannedVUs: 4},
			},
		},
		{
			et: mustNewExecutionTuple(newExecutionSegmentFromString("0:1/3"), nil),
			expectedSteps: []lib.ExecutionStep{
				{TimeOffset: 0 * time.Second, PlannedVUs: 1},
				{TimeOffset: 1 * time.Second, PlannedVUs: 2},
				{TimeOffset: 4 * time.Second, PlannedVUs: 1},
				{TimeOffset: 7 * time.Second, PlannedVUs: 0},
				{TimeOffset: 8 * time.Second, PlannedVUs: 1},
				{TimeOffset: 11 * time.Second, PlannedVUs: 2},
				{TimeOffset: 12 * time.Second, PlannedVUs: 1},
				{TimeOffset: 15 * time.Second, PlannedVUs: 0},
				{TimeOffset: 16 * time.Second, PlannedVUs: 1},
				{TimeOffset: 20 * time.Second, PlannedVUs: 0},
				{TimeOffset: 23 * time.Second, PlannedVUs: 2},
				{TimeOffset: 26 * time.Second, PlannedVUs: 0},
				{TimeOffset: 28 * time.Second, PlannedVUs: 1},
				{TimeOffset: 29 * time.Second, PlannedVUs: 0},
				{TimeOffset: 32 * time.Second, PlannedVUs: 1},
			},
		},
		{
			et: mustNewExecutionTuple(newExecutionSegmentFromString("0:1/3"), newExecutionSegmentSequenceFromString("0,1/3,1")),
			expectedSteps: []lib.ExecutionStep{
				{TimeOffset: 0 * time.Second, PlannedVUs: 1},
				{TimeOffset: 1 * time.Second, PlannedVUs: 2},
				{TimeOffset: 4 * time.Second, PlannedVUs: 1},
				{TimeOffset: 7 * time.Second, PlannedVUs: 0},
				{TimeOffset: 8 * time.Second, PlannedVUs: 1},
				{TimeOffset: 11 * time.Second, PlannedVUs: 2},
				{TimeOffset: 12 * time.Second, PlannedVUs: 1},
				{TimeOffset: 15 * time.Second, PlannedVUs: 0},
				{TimeOffset: 16 * time.Second, PlannedVUs: 1},
				{TimeOffset: 20 * time.Second, PlannedVUs: 0},
				{TimeOffset: 23 * time.Second, PlannedVUs: 2},
				{TimeOffset: 26 * time.Second, PlannedVUs: 0},
				{TimeOffset: 28 * time.Second, PlannedVUs: 1},
				{TimeOffset: 29 * time.Second, PlannedVUs: 0},
				{TimeOffset: 32 * time.Second, PlannedVUs: 1},
			},
		},
		{
			et: mustNewExecutionTuple(newExecutionSegmentFromString("1/3:2/3"), nil),
			expectedSteps: []lib.ExecutionStep{
				{TimeOffset: 0 * time.Second, PlannedVUs: 1},
				{TimeOffset: 1 * time.Second, PlannedVUs: 2},
				{TimeOffset: 4 * time.Second, PlannedVUs: 1},
				{TimeOffset: 7 * time.Second, PlannedVUs: 0},
				{TimeOffset: 8 * time.Second, PlannedVUs: 1},
				{TimeOffset: 11 * time.Second, PlannedVUs: 2},
				{TimeOffset: 12 * time.Second, PlannedVUs: 1},
				{TimeOffset: 15 * time.Second, PlannedVUs: 0},
				{TimeOffset: 16 * time.Second, PlannedVUs: 1},
				{TimeOffset: 20 * time.Second, PlannedVUs: 0},
				{TimeOffset: 23 * time.Second, PlannedVUs: 2},
				{TimeOffset: 26 * time.Second, PlannedVUs: 0},
				{TimeOffset: 28 * time.Second, PlannedVUs: 1},
				{TimeOffset: 29 * time.Second, PlannedVUs: 0},
				{TimeOffset: 32 * time.Second, PlannedVUs: 1},
			},
		},
		{
			et: mustNewExecutionTuple(newExecutionSegmentFromString("2/3:1"), nil),
			expectedSteps: []lib.ExecutionStep{
				{TimeOffset: 0 * time.Second, PlannedVUs: 1},
				{TimeOffset: 1 * time.Second, PlannedVUs: 2},
				{TimeOffset: 4 * time.Second, PlannedVUs: 1},
				{TimeOffset: 7 * time.Second, PlannedVUs: 0},
				{TimeOffset: 8 * time.Second, PlannedVUs: 1},
				{TimeOffset: 11 * time.Second, PlannedVUs: 2},
				{TimeOffset: 12 * time.Second, PlannedVUs: 1},
				{TimeOffset: 15 * time.Second, PlannedVUs: 0},
				{TimeOffset: 16 * time.Second, PlannedVUs: 1},
				{TimeOffset: 20 * time.Second, PlannedVUs: 0},
				{TimeOffset: 23 * time.Second, PlannedVUs: 2},
				{TimeOffset: 26 * time.Second, PlannedVUs: 0},
				{TimeOffset: 28 * time.Second, PlannedVUs: 1},
				{TimeOffset: 29 * time.Second, PlannedVUs: 0},
				{TimeOffset: 32 * time.Second, PlannedVUs: 1},
			},
		},
		{
			et: mustNewExecutionTuple(newExecutionSegmentFromString("0:1/3"), newExecutionSegmentSequenceFromString("0,1/3,2/3,1")),
			expectedSteps: []lib.ExecutionStep{
				{TimeOffset: 0 * time.Second, PlannedVUs: 2},
				{TimeOffset: 5 * time.Second, PlannedVUs: 1},
				{TimeOffset: 10 * time.Second, PlannedVUs: 2},
				{TimeOffset: 13 * time.Second, PlannedVUs: 1},
				{TimeOffset: 18 * time.Second, PlannedVUs: 2},
				{TimeOffset: 20 * time.Second, PlannedVUs: 1},
				{TimeOffset: 23 * time.Second, PlannedVUs: 2},
				{TimeOffset: 26 * time.Second, PlannedVUs: 0},
				{TimeOffset: 27 * time.Second, PlannedVUs: 1},
				{TimeOffset: 30 * time.Second, PlannedVUs: 0},
				{TimeOffset: 31 * time.Second, PlannedVUs: 1},
				{TimeOffset: 34 * time.Second, PlannedVUs: 2},
			},
		},
		{
			et: mustNewExecutionTuple(newExecutionSegmentFromString("1/3:2/3"), newExecutionSegmentSequenceFromString("0,1/3,2/3,1")),
			expectedSteps: []lib.ExecutionStep{
				{TimeOffset: 0 * time.Second, PlannedVUs: 1},
				{TimeOffset: 1 * time.Second, PlannedVUs: 2},
				{TimeOffset: 4 * time.Second, PlannedVUs: 1},
				{TimeOffset: 7 * time.Second, PlannedVUs: 0},
				{TimeOffset: 8 * time.Second, PlannedVUs: 1},
				{TimeOffset: 11 * time.Second, PlannedVUs: 2},
				{TimeOffset: 12 * time.Second, PlannedVUs: 1},
				{TimeOffset: 15 * time.Second, PlannedVUs: 0},
				{TimeOffset: 16 * time.Second, PlannedVUs: 1},
				{TimeOffset: 20 * time.Second, PlannedVUs: 0},
				{TimeOffset: 23 * time.Second, PlannedVUs: 2},
				{TimeOffset: 26 * time.Second, PlannedVUs: 0},
				{TimeOffset: 28 * time.Second, PlannedVUs: 1},
				{TimeOffset: 29 * time.Second, PlannedVUs: 0},
				{TimeOffset: 32 * time.Second, PlannedVUs: 1},
			},
		},
		{
			et: mustNewExecutionTuple(newExecutionSegmentFromString("2/3:1"), newExecutionSegmentSequenceFromString("0,1/3,2/3,1")),
			expectedSteps: []lib.ExecutionStep{
				{TimeOffset: 0 * time.Second, PlannedVUs: 1},
				{TimeOffset: 2 * time.Second, PlannedVUs: 2},
				{TimeOffset: 3 * time.Second, PlannedVUs: 1},
				{TimeOffset: 6 * time.Second, PlannedVUs: 0},
				{TimeOffset: 9 * time.Second, PlannedVUs: 1},
				{TimeOffset: 14 * time.Second, PlannedVUs: 0},
				{TimeOffset: 17 * time.Second, PlannedVUs: 1},
				{TimeOffset: 20 * time.Second, PlannedVUs: 0},
				{TimeOffset: 23 * time.Second, PlannedVUs: 1},
				{TimeOffset: 26 * time.Second, PlannedVUs: 0},
				{TimeOffset: 33 * time.Second, PlannedVUs: 1},
			},
		},
	}

	for _, testCase := range testCases {
		et := testCase.et
		expectedSteps := testCase.expectedSteps

		t.Run(et.String(), func(t *testing.T) {
			t.Parallel()
			rawStepsNoZeroEnd := conf.getRawExecutionSteps(et, false)
			assert.Equal(t, expectedSteps, rawStepsNoZeroEnd)
		})
	}
}

func TestRampingVUsGetRawExecutionStepsCornerCases(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		expectedSteps []lib.ExecutionStep
		et            *lib.ExecutionTuple
		stages        []Stage
		start         int64
	}{
		{
			name: "going up then down straight away",
			expectedSteps: []lib.ExecutionStep{
				{TimeOffset: 0 * time.Second, PlannedVUs: 2},
				{TimeOffset: 0 * time.Second, PlannedVUs: 5},
				{TimeOffset: 1 * time.Second, PlannedVUs: 4},
				{TimeOffset: 2 * time.Second, PlannedVUs: 3},
			},
			stages: []Stage{
				{Target: null.IntFrom(5), Duration: types.NullDurationFrom(0 * time.Second)},
				{Target: null.IntFrom(3), Duration: types.NullDurationFrom(2 * time.Second)},
			},
			start: 2,
		},
		{
			name: "jump up then go up again",
			expectedSteps: []lib.ExecutionStep{
				{TimeOffset: 0 * time.Second, PlannedVUs: 3},
				{TimeOffset: 1 * time.Second, PlannedVUs: 4},
				{TimeOffset: 2 * time.Second, PlannedVUs: 5},
			},
			stages: []Stage{
				{Target: null.IntFrom(5), Duration: types.NullDurationFrom(2 * time.Second)},
			},
			start: 3,
		},
		{
			name: "up down up down",
			expectedSteps: []lib.ExecutionStep{
				{TimeOffset: 0 * time.Second, PlannedVUs: 0},
				{TimeOffset: 1 * time.Second, PlannedVUs: 1},
				{TimeOffset: 2 * time.Second, PlannedVUs: 2},
				{TimeOffset: 3 * time.Second, PlannedVUs: 1},
				{TimeOffset: 4 * time.Second, PlannedVUs: 0},
				{TimeOffset: 5 * time.Second, PlannedVUs: 1},
				{TimeOffset: 6 * time.Second, PlannedVUs: 2},
				{TimeOffset: 7 * time.Second, PlannedVUs: 1},
				{TimeOffset: 8 * time.Second, PlannedVUs: 0},
			},
			stages: []Stage{
				{Target: null.IntFrom(2), Duration: types.NullDurationFrom(2 * time.Second)},
				{Target: null.IntFrom(0), Duration: types.NullDurationFrom(2 * time.Second)},
				{Target: null.IntFrom(2), Duration: types.NullDurationFrom(2 * time.Second)},
				{Target: null.IntFrom(0), Duration: types.NullDurationFrom(2 * time.Second)},
			},
		},
		{
			name: "up down up down in half",
			expectedSteps: []lib.ExecutionStep{
				{TimeOffset: 0 * time.Second, PlannedVUs: 0},
				{TimeOffset: 1 * time.Second, PlannedVUs: 1},
				{TimeOffset: 4 * time.Second, PlannedVUs: 0},
				{TimeOffset: 5 * time.Second, PlannedVUs: 1},
				{TimeOffset: 8 * time.Second, PlannedVUs: 0},
			},
			et: mustNewExecutionTuple(newExecutionSegmentFromString("0:1/2"), nil),
			stages: []Stage{
				{Target: null.IntFrom(2), Duration: types.NullDurationFrom(2 * time.Second)},
				{Target: null.IntFrom(0), Duration: types.NullDurationFrom(2 * time.Second)},
				{Target: null.IntFrom(2), Duration: types.NullDurationFrom(2 * time.Second)},
				{Target: null.IntFrom(0), Duration: types.NullDurationFrom(2 * time.Second)},
			},
		},
		{
			name: "up down up down in the other half",
			expectedSteps: []lib.ExecutionStep{
				{TimeOffset: 0 * time.Second, PlannedVUs: 0},
				{TimeOffset: 2 * time.Second, PlannedVUs: 1},
				{TimeOffset: 3 * time.Second, PlannedVUs: 0},
				{TimeOffset: 6 * time.Second, PlannedVUs: 1},
				{TimeOffset: 7 * time.Second, PlannedVUs: 0},
			},
			et: mustNewExecutionTuple(newExecutionSegmentFromString("1/2:1"), nil),
			stages: []Stage{
				{Target: null.IntFrom(2), Duration: types.NullDurationFrom(2 * time.Second)},
				{Target: null.IntFrom(0), Duration: types.NullDurationFrom(2 * time.Second)},
				{Target: null.IntFrom(2), Duration: types.NullDurationFrom(2 * time.Second)},
				{Target: null.IntFrom(0), Duration: types.NullDurationFrom(2 * time.Second)},
			},
		},
		{
			name: "up down up down in with nothing",
			expectedSteps: []lib.ExecutionStep{
				{TimeOffset: 0 * time.Second, PlannedVUs: 0},
			},
			et: mustNewExecutionTuple(newExecutionSegmentFromString("2/3:1"), newExecutionSegmentSequenceFromString("0,1/3,2/3,1")),
			stages: []Stage{
				{Target: null.IntFrom(2), Duration: types.NullDurationFrom(2 * time.Second)},
				{Target: null.IntFrom(0), Duration: types.NullDurationFrom(2 * time.Second)},
				{Target: null.IntFrom(2), Duration: types.NullDurationFrom(2 * time.Second)},
				{Target: null.IntFrom(0), Duration: types.NullDurationFrom(2 * time.Second)},
			},
		},
		{
			name: "up down up down in with funky sequence", // panics if there are no localIndex == 0 guards
			expectedSteps: []lib.ExecutionStep{
				{TimeOffset: 0 * time.Second, PlannedVUs: 0},
				{TimeOffset: 1 * time.Second, PlannedVUs: 1},
				{TimeOffset: 4 * time.Second, PlannedVUs: 0},
				{TimeOffset: 5 * time.Second, PlannedVUs: 1},
				{TimeOffset: 8 * time.Second, PlannedVUs: 0},
			},
			et: mustNewExecutionTuple(newExecutionSegmentFromString("0:1/3"), newExecutionSegmentSequenceFromString("0,1/3,1/2,2/3,1")),
			stages: []Stage{
				{Target: null.IntFrom(2), Duration: types.NullDurationFrom(2 * time.Second)},
				{Target: null.IntFrom(0), Duration: types.NullDurationFrom(2 * time.Second)},
				{Target: null.IntFrom(2), Duration: types.NullDurationFrom(2 * time.Second)},
				{Target: null.IntFrom(0), Duration: types.NullDurationFrom(2 * time.Second)},
			},
		},
		{
			name: "strange",
			expectedSteps: []lib.ExecutionStep{
				{TimeOffset: 0 * time.Second, PlannedVUs: 0},
				{TimeOffset: 1 * time.Second, PlannedVUs: 1},
				{TimeOffset: 5 * time.Second, PlannedVUs: 2},
				{TimeOffset: 8 * time.Second, PlannedVUs: 3},
				{TimeOffset: 11 * time.Second, PlannedVUs: 4},
				{TimeOffset: 15 * time.Second, PlannedVUs: 5},
				{TimeOffset: 18 * time.Second, PlannedVUs: 6},
				{TimeOffset: 23 * time.Second, PlannedVUs: 7},
				{TimeOffset: 35 * time.Second, PlannedVUs: 8},
				{TimeOffset: 44 * time.Second, PlannedVUs: 9},
			},
			et: mustNewExecutionTuple(newExecutionSegmentFromString("0:0.3"), newExecutionSegmentSequenceFromString("0,0.3,0.6,0.9,1")),
			stages: []Stage{
				{Target: null.IntFrom(20), Duration: types.NullDurationFrom(20 * time.Second)},
				{Target: null.IntFrom(30), Duration: types.NullDurationFrom(30 * time.Second)},
			},
		},
		{
			name: "more up and down",
			expectedSteps: []lib.ExecutionStep{
				{TimeOffset: 0 * time.Second, PlannedVUs: 0},
				{TimeOffset: 1 * time.Second, PlannedVUs: 1},
				{TimeOffset: 2 * time.Second, PlannedVUs: 2},
				{TimeOffset: 3 * time.Second, PlannedVUs: 3},
				{TimeOffset: 4 * time.Second, PlannedVUs: 4},
				{TimeOffset: 5 * time.Second, PlannedVUs: 5},
				{TimeOffset: 6 * time.Second, PlannedVUs: 4},
				{TimeOffset: 7 * time.Second, PlannedVUs: 3},
				{TimeOffset: 8 * time.Second, PlannedVUs: 2},
				{TimeOffset: 9 * time.Second, PlannedVUs: 1},
				{TimeOffset: 10 * time.Second, PlannedVUs: 0},
			},
			stages: []Stage{
				{Target: null.IntFrom(5), Duration: types.NullDurationFrom(5 * time.Second)},
				{Target: null.IntFrom(0), Duration: types.NullDurationFrom(5 * time.Second)},
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		conf := NewRampingVUsConfig("test")
		conf.StartVUs = null.IntFrom(testCase.start)
		conf.Stages = testCase.stages
		et := testCase.et
		if et == nil {
			et = mustNewExecutionTuple(nil, nil)
		}
		expectedSteps := testCase.expectedSteps

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			rawStepsNoZeroEnd := conf.getRawExecutionSteps(et, false)
			assert.Equal(t, expectedSteps, rawStepsNoZeroEnd)
		})
	}
}

func BenchmarkRampingVUsGetRawExecutionSteps(b *testing.B) {
	testCases := []struct {
		seq string
		seg string
	}{
		{},
		{seg: "0:1"},
		{seq: "0,0.3,0.5,0.6,0.7,0.8,0.9,1", seg: "0:0.3"},
		{seq: "0,0.1,0.2,0.3,0.4,0.5,0.6,0.7,0.8,0.9,1", seg: "0:0.1"},
		{seg: "2/5:4/5"},
		{seg: "2235/5213:4/5"}, // just wanted it to be ugly ;D
	}

	stageCases := []struct {
		name   string
		stages string
	}{
		{
			name:   "normal",
			stages: `[{"duration":"5m", "target":5000},{"duration":"5m", "target":5000},{"duration":"5m", "target":10000},{"duration":"5m", "target":10000}]`,
		}, {
			name: "rollercoaster",
			stages: `[{"duration":"5m", "target":5000},{"duration":"5m", "target":0},
				{"duration":"5m", "target":5000},{"duration":"5m", "target":0},
				{"duration":"5m", "target":5000},{"duration":"5m", "target":0},
				{"duration":"5m", "target":5000},{"duration":"5m", "target":0},
				{"duration":"5m", "target":5000},{"duration":"5m", "target":0},
				{"duration":"5m", "target":5000},{"duration":"5m", "target":0},
				{"duration":"5m", "target":5000},{"duration":"5m", "target":0},
				{"duration":"5m", "target":5000},{"duration":"5m", "target":0},
				{"duration":"5m", "target":5000},{"duration":"5m", "target":0},
				{"duration":"5m", "target":5000},{"duration":"5m", "target":0},
				{"duration":"5m", "target":5000},{"duration":"5m", "target":0}]`,
		},
	}
	for _, tc := range testCases {
		tc := tc
		b.Run(fmt.Sprintf("seq:%s;segment:%s", tc.seq, tc.seg), func(b *testing.B) {
			ess, err := lib.NewExecutionSegmentSequenceFromString(tc.seq)
			require.NoError(b, err)
			segment, err := lib.NewExecutionSegmentFromString(tc.seg)
			require.NoError(b, err)
			if tc.seg == "" {
				segment = nil // specifically for the optimization
			}
			et, err := lib.NewExecutionTuple(segment, &ess)
			require.NoError(b, err)
			for _, stageCase := range stageCases {
				var st []Stage
				require.NoError(b, json.Unmarshal([]byte(stageCase.stages), &st))
				vlvc := RampingVUsConfig{
					Stages: st,
				}
				b.Run(stageCase.name, func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						_ = vlvc.getRawExecutionSteps(et, false)
					}
				})
			}
		})
	}
}

// TODO: delete in favor of lib.generateRandomSequence() after
// https://github.com/k6io/k6/issues/1302 is done (can't import now due to
// import loops...)
func generateRandomSequence(t testing.TB, n, m int64, r *rand.Rand) lib.ExecutionSegmentSequence {
	var err error
	ess := lib.ExecutionSegmentSequence(make([]*lib.ExecutionSegment, n))
	numerators := make([]int64, n)
	var denominator int64
	for i := int64(0); i < n; i++ {
		numerators[i] = 1 + r.Int63n(m)
		denominator += numerators[i]
	}
	from := big.NewRat(0, 1)
	for i := int64(0); i < n; i++ {
		to := new(big.Rat).Add(big.NewRat(numerators[i], denominator), from)
		ess[i], err = lib.NewExecutionSegment(from, to)
		require.NoError(t, err)
		from = to
	}

	return ess
}

func TestSumRandomSegmentSequenceMatchesNoSegment(t *testing.T) {
	t.Parallel()

	const (
		numTests         = 10
		maxStages        = 10
		minStageDuration = 1 * time.Second
		maxStageDuration = 10 * time.Minute
		maxVUs           = 300
		segmentSeqMaxLen = 15
		maxNumerator     = 300
	)
	getTestConfig := func(r *rand.Rand, name string) RampingVUsConfig {
		stagesCount := 1 + r.Int31n(maxStages)
		stages := make([]Stage, stagesCount)
		for s := int32(0); s < stagesCount; s++ {
			dur := (minStageDuration + time.Duration(r.Int63n(int64(maxStageDuration-minStageDuration)))).Round(time.Second)
			stages[s] = Stage{Duration: types.NullDurationFrom(dur), Target: null.IntFrom(r.Int63n(maxVUs))}
		}

		c := NewRampingVUsConfig(name)
		c.GracefulRampDown = types.NullDurationFrom(0)
		c.GracefulStop = types.NullDurationFrom(0)
		c.StartVUs = null.IntFrom(r.Int63n(maxVUs))
		c.Stages = stages
		return c
	}

	subtractChildSteps := func(t *testing.T, parent, child []lib.ExecutionStep) {
		t.Logf("subtractChildSteps()")
		for _, step := range child {
			t.Logf("	child planned VUs for time offset %s: %d", step.TimeOffset, step.PlannedVUs)
		}
		sub := uint64(0)
		ci := 0
		for pi, p := range parent {
			// We iterate over all parent steps and match them to child steps.
			// Once we have a match, we remove the child step's plannedVUs from
			// the parent steps until a new match, when we adjust the subtracted
			// amount again.
			if p.TimeOffset > child[ci].TimeOffset && ci != len(child)-1 {
				t.Errorf("ERR Could not match child offset %s with any parent time offset", child[ci].TimeOffset)
			}
			if p.TimeOffset == child[ci].TimeOffset {
				t.Logf("Setting sub to %d at t=%s", child[ci].PlannedVUs, child[ci].TimeOffset)
				sub = child[ci].PlannedVUs
				if ci != len(child)-1 {
					ci++
				}
			}
			t.Logf("Subtracting %d VUs (out of %d) at t=%s", sub, p.PlannedVUs, p.TimeOffset)
			parent[pi].PlannedVUs -= sub
		}
	}

	for i := 0; i < numTests; i++ {
		name := fmt.Sprintf("random%02d", i)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			seed := time.Now().UnixNano()
			r := rand.New(rand.NewSource(seed)) //nolint:gosec
			t.Logf("Random source seeded with %d\n", seed)
			c := getTestConfig(r, name)
			ranSeqLen := 2 + r.Int63n(segmentSeqMaxLen-1)
			t.Logf("Config: %#v, ranSeqLen: %d", c, ranSeqLen)
			randomSequence := generateRandomSequence(t, ranSeqLen, maxNumerator, r)
			t.Logf("Random sequence: %s", randomSequence)
			fullSeg, err := lib.NewExecutionTuple(nil, nil)
			require.NoError(t, err)
			fullRawSteps := c.getRawExecutionSteps(fullSeg, false)

			for _, step := range fullRawSteps {
				t.Logf("original planned VUs for time offset %s: %d", step.TimeOffset, step.PlannedVUs)
			}

			for s := 0; s < len(randomSequence); s++ {
				et, err := lib.NewExecutionTuple(randomSequence[s], &randomSequence)
				require.NoError(t, err)
				segRawSteps := c.getRawExecutionSteps(et, false)
				subtractChildSteps(t, fullRawSteps, segRawSteps)
			}

			for _, step := range fullRawSteps {
				if step.PlannedVUs != 0 {
					t.Errorf("ERR Remaining planned VUs for time offset %s are not 0 but %d", step.TimeOffset, step.PlannedVUs)
				}
			}
		})
	}
}
