package executor

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
)

func getTestRampingArrivalRateConfig() *RampingArrivalRateConfig {
	return &RampingArrivalRateConfig{
		BaseConfig: BaseConfig{GracefulStop: types.NullDurationFrom(1 * time.Second)},
		TimeUnit:   types.NullDurationFrom(time.Second),
		StartRate:  null.IntFrom(10),
		Stages: []Stage{
			{
				Duration: types.NullDurationFrom(time.Second * 1),
				Target:   null.IntFrom(10),
			},
			{
				Duration: types.NullDurationFrom(time.Second * 1),
				Target:   null.IntFrom(50),
			},
			{
				Duration: types.NullDurationFrom(time.Second * 1),
				Target:   null.IntFrom(50),
			},
		},
		PreAllocatedVUs: null.IntFrom(10),
		MaxVUs:          null.IntFrom(20),
	}
}

func TestRampingArrivalRateRunNotEnoughAllocatedVUsWarn(t *testing.T) {
	t.Parallel()

	runner := simpleRunner(func(_ context.Context, _ *lib.State) error {
		time.Sleep(time.Second)
		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, getTestRampingArrivalRateConfig())
	defer test.cancel()

	engineOut := make(chan metrics.SampleContainer, 1000)
	require.NoError(t, test.executor.Run(test.ctx, engineOut))
	entries := test.logHook.Drain()
	require.NotEmpty(t, entries)
	for _, entry := range entries {
		require.Equal(t,
			"Insufficient VUs, reached 20 active VUs and cannot initialize more",
			entry.Message)
		require.Equal(t, logrus.WarnLevel, entry.Level)
	}
}

func TestRampingArrivalRateRunCorrectRate(t *testing.T) {
	t.Parallel()
	var count int64
	runner := simpleRunner(func(_ context.Context, _ *lib.State) error {
		atomic.AddInt64(&count, 1)
		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, getTestRampingArrivalRateConfig())
	defer test.cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// check that we got around the amount of VU iterations as we would expect
		var currentCount int64

		time.Sleep(time.Second)
		currentCount = atomic.SwapInt64(&count, 0)
		assert.InDelta(t, 10, currentCount, 1)

		time.Sleep(time.Second)
		currentCount = atomic.SwapInt64(&count, 0)
		assert.InDelta(t, 30, currentCount, 2)

		time.Sleep(time.Second)
		currentCount = atomic.SwapInt64(&count, 0)
		assert.InDelta(t, 50, currentCount, 3)
	}()
	engineOut := make(chan metrics.SampleContainer, 1000)
	require.NoError(t, test.executor.Run(test.ctx, engineOut))
	wg.Wait()
	require.Empty(t, test.logHook.Drain())
}

func TestRampingArrivalRateRunUnplannedVUs(t *testing.T) {
	t.Parallel()

	config := &RampingArrivalRateConfig{
		TimeUnit: types.NullDurationFrom(time.Second),
		Stages: []Stage{
			{
				// the minus one makes it so only 9 iterations will be started instead of 10
				// as the 10th happens to be just at the end and sometimes doesn't get executed :(
				Duration: types.NullDurationFrom(time.Second*2 - 1),
				Target:   null.IntFrom(10),
			},
		},
		PreAllocatedVUs: null.IntFrom(1),
		MaxVUs:          null.IntFrom(3),
	}

	var count int64
	ch := make(chan struct{})  // closed when new unplannedVU is started and signal to get to next iterations
	ch2 := make(chan struct{}) // closed when a second iteration was started on an old VU in order to test it won't start a second unplanned VU in parallel or at all
	runner := simpleRunner(func(_ context.Context, _ *lib.State) error {
		cur := atomic.AddInt64(&count, 1)
		if cur == 1 {
			<-ch // wait to start again
		} else if cur == 2 {
			<-ch2 // wait to start again
		}

		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, config)
	defer test.cancel()

	engineOut := make(chan metrics.SampleContainer, 1000)
	test.state.SetInitVUFunc(func(ctx context.Context, _ *logrus.Entry) (lib.InitializedVU, error) {
		cur := atomic.LoadInt64(&count)
		require.Equal(t, cur, int64(1))
		time.Sleep(time.Second / 2)

		close(ch)
		time.Sleep(time.Millisecond * 150)

		cur = atomic.LoadInt64(&count)
		require.Equal(t, cur, int64(2))

		time.Sleep(time.Millisecond * 150)
		cur = atomic.LoadInt64(&count)
		require.Equal(t, cur, int64(2))

		close(ch2)
		time.Sleep(time.Millisecond * 200)
		cur = atomic.LoadInt64(&count)
		require.NotEqual(t, cur, int64(2))
		idl, idg := test.state.GetUniqueVUIdentifiers()
		return runner.NewVU(ctx, idl, idg, engineOut)
	})

	assert.NoError(t, test.executor.Run(test.ctx, engineOut))
	assert.Empty(t, test.logHook.Drain())

	droppedIters := sumMetricValues(engineOut, metrics.DroppedIterationsName)
	assert.Equal(t, count+int64(droppedIters), int64(9))
}

func TestRampingArrivalRateRunCorrectRateWithSlowRate(t *testing.T) {
	t.Parallel()

	config := &RampingArrivalRateConfig{
		TimeUnit: types.NullDurationFrom(time.Second),
		Stages: []Stage{
			{
				Duration: types.NullDurationFrom(time.Second * 2),
				Target:   null.IntFrom(10),
			},
		},
		PreAllocatedVUs: null.IntFrom(1),
		MaxVUs:          null.IntFrom(3),
	}

	var count int64
	ch := make(chan struct{}) // closed when new unplannedVU is started and signal to get to next iterations
	runner := simpleRunner(func(_ context.Context, _ *lib.State) error {
		cur := atomic.AddInt64(&count, 1)
		if cur == 1 {
			<-ch // wait to start again
		}

		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, config)
	defer test.cancel()

	engineOut := make(chan metrics.SampleContainer, 1000)
	test.state.SetInitVUFunc(func(ctx context.Context, _ *logrus.Entry) (lib.InitializedVU, error) {
		t.Log("init")
		cur := atomic.LoadInt64(&count)
		require.Equal(t, cur, int64(1))
		time.Sleep(time.Millisecond * 200)
		close(ch)
		time.Sleep(time.Millisecond * 200)
		cur = atomic.LoadInt64(&count)
		require.NotEqual(t, cur, int64(1))

		idl, idg := test.state.GetUniqueVUIdentifiers()
		return runner.NewVU(ctx, idl, idg, engineOut)
	})

	assert.NoError(t, test.executor.Run(test.ctx, engineOut))
	assert.Empty(t, test.logHook.Drain())
	assert.Equal(t, int64(0), test.state.GetCurrentlyActiveVUsCount())
	assert.Equal(t, int64(2), test.state.GetInitializedVUsCount())
}

func TestRampingArrivalRateRunGracefulStop(t *testing.T) {
	t.Parallel()

	config := &RampingArrivalRateConfig{
		TimeUnit: types.NullDurationFrom(1 * time.Second),
		Stages: []Stage{
			{
				Duration: types.NullDurationFrom(2 * time.Second),
				Target:   null.IntFrom(10),
			},
		},
		StartRate:       null.IntFrom(10),
		PreAllocatedVUs: null.IntFrom(10),
		MaxVUs:          null.IntFrom(10),
		BaseConfig: BaseConfig{
			GracefulStop: types.NullDurationFrom(5 * time.Second),
		},
	}

	runner := simpleRunner(func(_ context.Context, _ *lib.State) error {
		time.Sleep(5 * time.Second)
		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, config)
	defer test.cancel()

	engineOut := make(chan metrics.SampleContainer, 1000)
	defer close(engineOut)

	assert.NoError(t, test.executor.Run(test.ctx, engineOut))
	assert.Equal(t, int64(0), test.state.GetCurrentlyActiveVUsCount())
	assert.Equal(t, int64(10), test.state.GetInitializedVUsCount())
	assert.Equal(t, uint64(10), test.state.GetFullIterationCount())
}

func BenchmarkRampingArrivalRateRun(b *testing.B) {
	tests := []struct {
		prealloc null.Int
	}{
		{prealloc: null.IntFrom(10)},
		{prealloc: null.IntFrom(100)},
		{prealloc: null.IntFrom(1e3)},
		{prealloc: null.IntFrom(10e3)},
	}

	for _, tc := range tests {
		b.Run(fmt.Sprintf("VUs%d", tc.prealloc.ValueOrZero()), func(b *testing.B) {
			engineOut := make(chan metrics.SampleContainer, 1000)
			defer close(engineOut)
			go func() {
				for range engineOut { //nolint:revive // we want to discard samples
				}
			}()

			var count int64
			runner := simpleRunner(func(_ context.Context, _ *lib.State) error {
				atomic.AddInt64(&count, 1)
				return nil
			})

			testRunState := getTestRunState(b, lib.Options{}, runner)
			es := lib.NewExecutionState(
				testRunState, mustNewExecutionTuple(nil, nil),
				uint64(tc.prealloc.Int64), uint64(tc.prealloc.Int64), //nolint:gosec
			)

			// an high target to get the highest rate
			target := int64(1e9)

			ctx, cancel, executor, _ := setupExecutor(
				b, &RampingArrivalRateConfig{
					TimeUnit: types.NullDurationFrom(1 * time.Second),
					Stages: []Stage{
						{
							Duration: types.NullDurationFrom(0),
							Target:   null.IntFrom(target),
						},
						{
							Duration: types.NullDurationFrom(5 * time.Second),
							Target:   null.IntFrom(target),
						},
					},
					PreAllocatedVUs: tc.prealloc,
					MaxVUs:          tc.prealloc,
				}, es)
			defer cancel()

			b.ResetTimer()
			start := time.Now()

			err := executor.Run(ctx, engineOut)
			took := time.Since(start)
			assert.NoError(b, err)

			iterations := float64(atomic.LoadInt64(&count))
			b.ReportMetric(0, "ns/op")
			b.ReportMetric(iterations/took.Seconds(), "iterations/s")
		})
	}
}

func mustNewExecutionTuple(seg *lib.ExecutionSegment, seq *lib.ExecutionSegmentSequence) *lib.ExecutionTuple {
	et, err := lib.NewExecutionTuple(seg, seq)
	if err != nil {
		panic(err)
	}
	return et
}

func TestRampingArrivalRateCal(t *testing.T) {
	t.Parallel()

	var (
		defaultTimeUnit = time.Second
		getConfig       = func() RampingArrivalRateConfig {
			return RampingArrivalRateConfig{
				StartRate: null.IntFrom(0),
				Stages: []Stage{ // TODO make this even bigger and longer .. will need more time
					{
						Duration: types.NullDurationFrom(time.Second * 5),
						Target:   null.IntFrom(1),
					},
					{
						Duration: types.NullDurationFrom(time.Second * 1),
						Target:   null.IntFrom(1),
					},
					{
						Duration: types.NullDurationFrom(time.Second * 5),
						Target:   null.IntFrom(0),
					},
				},
			}
		}
	)

	testCases := []struct {
		expectedTimes []time.Duration
		et            *lib.ExecutionTuple
		timeUnit      time.Duration
	}{
		{
			expectedTimes: []time.Duration{time.Millisecond * 3162, time.Millisecond * 4472, time.Millisecond * 5500, time.Millisecond * 6527, time.Millisecond * 7837, time.Second * 11},
			et:            mustNewExecutionTuple(nil, nil),
		},
		{
			expectedTimes: []time.Duration{time.Millisecond * 4472, time.Millisecond * 7837},
			et:            mustNewExecutionTuple(newExecutionSegmentFromString("0:1/3"), nil),
		},
		{
			expectedTimes: []time.Duration{time.Millisecond * 4472, time.Millisecond * 7837},
			et:            mustNewExecutionTuple(newExecutionSegmentFromString("0:1/3"), newExecutionSegmentSequenceFromString("0,1/3,1")),
		},
		{
			expectedTimes: []time.Duration{time.Millisecond * 4472, time.Millisecond * 7837},
			et:            mustNewExecutionTuple(newExecutionSegmentFromString("1/3:2/3"), nil),
		},
		{
			expectedTimes: []time.Duration{time.Millisecond * 4472, time.Millisecond * 7837},
			et:            mustNewExecutionTuple(newExecutionSegmentFromString("2/3:1"), nil),
		},
		{
			expectedTimes: []time.Duration{time.Millisecond * 3162, time.Millisecond * 6527},
			et:            mustNewExecutionTuple(newExecutionSegmentFromString("0:1/3"), newExecutionSegmentSequenceFromString("0,1/3,2/3,1")),
		},
		{
			expectedTimes: []time.Duration{time.Millisecond * 4472, time.Millisecond * 7837},
			et:            mustNewExecutionTuple(newExecutionSegmentFromString("1/3:2/3"), newExecutionSegmentSequenceFromString("0,1/3,2/3,1")),
		},
		{
			expectedTimes: []time.Duration{time.Millisecond * 5500, time.Millisecond * 11000},
			et:            mustNewExecutionTuple(newExecutionSegmentFromString("2/3:1"), newExecutionSegmentSequenceFromString("0,1/3,2/3,1")),
		},
		{
			expectedTimes: []time.Duration{
				time.Millisecond * 1825, time.Millisecond * 2581, time.Millisecond * 3162, time.Millisecond * 3651, time.Millisecond * 4082, time.Millisecond * 4472,
				time.Millisecond * 4830, time.Millisecond * 5166, time.Millisecond * 5499, time.Millisecond * 5833, time.Millisecond * 6169, time.Millisecond * 6527,
				time.Millisecond * 6917, time.Millisecond * 7348, time.Millisecond * 7837, time.Millisecond * 8418, time.Millisecond * 9174, time.Millisecond * 10999,
			},
			et:       mustNewExecutionTuple(nil, nil),
			timeUnit: time.Second / 3, // three  times as fast
		},
		// TODO: extend more
	}

	for testNum, testCase := range testCases {
		et := testCase.et
		expectedTimes := testCase.expectedTimes
		config := getConfig()
		config.TimeUnit = types.NewNullDuration(testCase.timeUnit, true)
		if testCase.timeUnit == 0 {
			config.TimeUnit = types.NewNullDuration(defaultTimeUnit, true)
		}

		t.Run(fmt.Sprintf("testNum %d - %s timeunit %s", testNum, et, config.TimeUnit), func(t *testing.T) {
			t.Parallel()
			ch := make(chan time.Duration)
			go config.cal(et, ch)
			changes := make([]time.Duration, 0, len(expectedTimes))
			for c := range ch {
				changes = append(changes, c)
			}
			assert.Equal(t, len(expectedTimes), len(changes))
			for i, expectedTime := range expectedTimes {
				require.True(t, i < len(changes))
				change := changes[i]
				assert.InEpsilon(t, expectedTime, change, 0.001, "Expected around %s, got %s", expectedTime, change)
			}
		})
	}
}

func BenchmarkCal(b *testing.B) {
	for _, t := range []time.Duration{
		time.Second, time.Minute,
	} {
		t := t
		b.Run(t.String(), func(b *testing.B) {
			config := RampingArrivalRateConfig{
				TimeUnit:  types.NullDurationFrom(time.Second),
				StartRate: null.IntFrom(50),
				Stages: []Stage{
					{
						Duration: types.NullDurationFrom(t),
						Target:   null.IntFrom(49),
					},
					{
						Duration: types.NullDurationFrom(t),
						Target:   null.IntFrom(50),
					},
				},
			}
			et := mustNewExecutionTuple(nil, nil)

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					ch := make(chan time.Duration, 20)
					go config.cal(et, ch)
					for c := range ch {
						_ = c
					}
				}
			})
		})
	}
}

func BenchmarkCalRat(b *testing.B) {
	for _, t := range []time.Duration{
		time.Second, time.Minute,
	} {
		t := t
		b.Run(t.String(), func(b *testing.B) {
			config := RampingArrivalRateConfig{
				TimeUnit:  types.NullDurationFrom(time.Second),
				StartRate: null.IntFrom(50),
				Stages: []Stage{
					{
						Duration: types.NullDurationFrom(t),
						Target:   null.IntFrom(49),
					},
					{
						Duration: types.NullDurationFrom(t),
						Target:   null.IntFrom(50),
					},
				},
			}
			et := mustNewExecutionTuple(nil, nil)

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					ch := make(chan time.Duration, 20)
					go config.calRat(et, ch)
					for c := range ch {
						_ = c
					}
				}
			})
		})
	}
}

func TestCompareCalImplementation(t *testing.T) {
	t.Parallel()
	// This test checks that the cal and calRat implementation get roughly similar numbers
	// in my experiment the difference is 1(nanosecond) in 7 case for the whole test
	// the duration is 1 second for each stage as calRat takes way longer - a longer better test can
	// be done when/if it's performance is improved
	config := RampingArrivalRateConfig{
		TimeUnit:  types.NullDurationFrom(time.Second),
		StartRate: null.IntFrom(0),
		Stages: []Stage{
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(200),
			},
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(200),
			},
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(2000),
			},
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(2000),
			},
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(300),
			},
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(300),
			},
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(1333),
			},
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(1334),
			},
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(1334),
			},
		},
	}

	et := mustNewExecutionTuple(nil, nil)
	chRat := make(chan time.Duration, 20)
	ch := make(chan time.Duration, 20)
	go config.calRat(et, chRat)
	go config.cal(et, ch)
	count := 0
	var diff int
	for c := range ch {
		count++
		cRat := <-chRat
		if !assert.InDelta(t, c, cRat, 1, "%d", count) {
			diff++
		}
	}
	require.Equal(t, 0, diff)
}

// calRat code here is just to check how accurate the cal implemenattion is
// there are no other tests for it so it depends on the test of cal that it is actually accurate :D

//nolint:gochecknoglobals
var two = big.NewRat(2, 1)

// from https://groups.google.com/forum/#!topic/golang-nuts/aIcDf8T-Png
func sqrtRat(x *big.Rat) *big.Rat {
	var z, a, b big.Rat
	var ns, ds big.Int
	ni, di := x.Num(), x.Denom()
	z.SetFrac(ns.Rsh(ni, uint(ni.BitLen())/2), ds.Rsh(di, uint(di.BitLen())/2)) //nolint:gosec
	for i := 10; i > 0; i-- {                                                   // TODO: better termination
		a.Sub(a.Mul(&z, &z), x)
		f, _ := a.Float64()
		if f == 0 {
			break
		}
		// fmt.Println(x, z, i)
		z.Sub(&z, b.Quo(&a, b.Mul(two, &z)))
	}
	return &z
}

// This implementation is just for reference and accuracy testing
func (varc RampingArrivalRateConfig) calRat(et *lib.ExecutionTuple, ch chan<- time.Duration) {
	defer close(ch)

	start, offsets, _ := et.GetStripedOffsets()
	li := -1
	next := func() int64 {
		li++
		return offsets[li%len(offsets)]
	}
	iRat := big.NewRat(start+1, 1)

	carry := big.NewRat(0, 1)
	doneSoFar := big.NewRat(0, 1)
	endCount := big.NewRat(0, 1)
	curr := varc.StartRate.ValueOrZero()
	var base time.Duration
	for _, stage := range varc.Stages {
		target := stage.Target.ValueOrZero()
		if target != curr {
			var (
				from = big.NewRat(curr, int64(time.Second))
				to   = big.NewRat(target, int64(time.Second))
				dur  = big.NewRat(stage.Duration.TimeDuration().Nanoseconds(), 1)
			)
			// precalcualations :)
			toMinusFrom := new(big.Rat).Sub(to, from)
			fromSquare := new(big.Rat).Mul(from, from)
			durMulSquare := new(big.Rat).Mul(dur, fromSquare)
			fromMulDur := new(big.Rat).Mul(from, dur)
			oneOverToMinusFrom := new(big.Rat).Inv(toMinusFrom)

			endCount.Add(endCount,
				new(big.Rat).Mul(
					dur,
					new(big.Rat).Add(new(big.Rat).Mul(toMinusFrom, big.NewRat(1, 2)), from)))
			for ; endCount.Cmp(iRat) >= 0; iRat.Add(iRat, big.NewRat(next(), 1)) {
				// even with all of this optimizations sqrtRat is taking so long this is still
				// extremely slow ... :(
				buf := new(big.Rat).Sub(iRat, doneSoFar)
				buf.Mul(buf, two)
				buf.Mul(buf, toMinusFrom)
				buf.Add(buf, durMulSquare)
				buf.Mul(buf, dur)
				buf.Sub(fromMulDur, sqrtRat(buf))
				buf.Mul(buf, oneOverToMinusFrom)

				r, _ := buf.Float64()
				ch <- base + time.Duration(-r) // the minus is because we don't deive by from-to but by to-from above
			}
		} else {
			step := big.NewRat(int64(time.Second), target)
			first := big.NewRat(0, 1)
			first.Sub(first, carry)
			endCount.Add(endCount, new(big.Rat).Mul(big.NewRat(target, 1), big.NewRat(stage.Duration.TimeDuration().Nanoseconds(), varc.TimeUnit.TimeDuration().Nanoseconds())))

			for ; endCount.Cmp(iRat) >= 0; iRat.Add(iRat, big.NewRat(next(), 1)) {
				res := new(big.Rat).Sub(iRat, doneSoFar) // this can get next added to it but will need to change the above for .. so
				r, _ := res.Mul(res, step).Float64()
				ch <- base + time.Duration(r)
				first.Add(first, step)
			}
		}
		doneSoFar.Set(endCount) // copy
		curr = target
		base += stage.Duration.TimeDuration()
	}
}

func TestRampingArrivalRateGlobalIters(t *testing.T) {
	t.Parallel()

	config := &RampingArrivalRateConfig{
		BaseConfig:      BaseConfig{GracefulStop: types.NullDurationFrom(100 * time.Millisecond)},
		TimeUnit:        types.NullDurationFrom(950 * time.Millisecond),
		StartRate:       null.IntFrom(0),
		PreAllocatedVUs: null.IntFrom(4),
		MaxVUs:          null.IntFrom(5),
		Stages: []Stage{
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(20),
			},
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(0),
			},
		},
	}

	testCases := []struct {
		seq, seg string
		expIters []uint64
	}{
		{"0,1/4,3/4,1", "0:1/4", []uint64{1, 6, 11, 16}},
		{"0,1/4,3/4,1", "1/4:3/4", []uint64{0, 2, 4, 5, 7, 9, 10, 12, 14, 15, 17, 19, 20}},
		{"0,1/4,3/4,1", "3/4:1", []uint64{3, 8, 13, 18}},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("%s_%s", tc.seq, tc.seg), func(t *testing.T) {
			t.Parallel()

			gotIters := []uint64{}
			var mx sync.Mutex
			runner := simpleRunner(func(_ context.Context, state *lib.State) error {
				mx.Lock()
				gotIters = append(gotIters, state.GetScenarioGlobalVUIter())
				mx.Unlock()
				return nil
			})

			test := setupExecutorTest(t, tc.seg, tc.seq, lib.Options{}, runner, config)
			defer test.cancel()

			engineOut := make(chan metrics.SampleContainer, 100)
			require.NoError(t, test.executor.Run(test.ctx, engineOut))
			assert.Equal(t, tc.expIters, gotIters)
		})
	}
}

func TestRampingArrivalRateCornerCase(t *testing.T) {
	t.Parallel()
	config := &RampingArrivalRateConfig{
		TimeUnit:  types.NullDurationFrom(time.Second),
		StartRate: null.IntFrom(1),
		Stages: []Stage{
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(1),
			},
		},
		MaxVUs: null.IntFrom(2),
	}

	et, err := lib.NewExecutionTuple(newExecutionSegmentFromString("1/5:2/5"), newExecutionSegmentSequenceFromString("0,1/5,2/5,1"))
	require.NoError(t, err)

	require.False(t, config.HasWork(et))
}

func TestRampingArrivalRateActiveVUs(t *testing.T) {
	t.Parallel()

	config := &RampingArrivalRateConfig{
		BaseConfig: BaseConfig{GracefulStop: types.NullDurationFrom(0 * time.Second)},
		TimeUnit:   types.NullDurationFrom(time.Second),
		StartRate:  null.IntFrom(10),
		Stages: []Stage{
			{
				Duration: types.NullDurationFrom(950 * time.Millisecond),
				Target:   null.IntFrom(20),
			},
		},
		PreAllocatedVUs: null.IntFrom(5),
		MaxVUs:          null.IntFrom(10),
	}

	var (
		running          int64
		getCurrActiveVUs func() int64
		runMx            sync.Mutex
	)

	runner := simpleRunner(func(ctx context.Context, _ *lib.State) error {
		runMx.Lock()
		running++
		assert.Equal(t, running, getCurrActiveVUs())
		runMx.Unlock()
		// Block the VU to cause the executor to schedule more
		<-ctx.Done()
		return nil
	})
	test := setupExecutorTest(t, "", "", lib.Options{}, runner, config)
	defer test.cancel()

	getCurrActiveVUs = test.state.GetCurrentlyActiveVUsCount

	engineOut := make(chan metrics.SampleContainer, 1000)
	require.NoError(t, test.executor.Run(test.ctx, engineOut))

	assert.GreaterOrEqual(t, running, int64(5))
	assert.LessOrEqual(t, running, int64(10))
}

func TestRampingArrivalRateActiveVUs_GetExecutionRequirements(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		preAllocatedVUs    int64
		maxVUs             int64
		segment            string
		sequence           string
		expPlannedVUs      uint64
		expMaxUnplannedVUs uint64
	}{
		"Segmented/Odd":     {preAllocatedVUs: 1, maxVUs: 4000, segment: "0:1/4", sequence: "0,1/4,1/2,3/4,1", expPlannedVUs: 1, expMaxUnplannedVUs: 999},
		"Segmented/Even":    {preAllocatedVUs: 100, maxVUs: 4000, segment: "0:1/4", sequence: "0,1/4,1/2,3/4,1", expPlannedVUs: 25, expMaxUnplannedVUs: 975},
		"NotSegmented/Odd":  {preAllocatedVUs: 1, maxVUs: 4000, segment: "0:1", sequence: "0,1", expPlannedVUs: 1, expMaxUnplannedVUs: 3999},
		"NotSegmented/Even": {preAllocatedVUs: 100, maxVUs: 4000, segment: "0:1", sequence: "0,1", expPlannedVUs: 100, expMaxUnplannedVUs: 3900},
	}

	for name, tc := range tcs {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			config := &RampingArrivalRateConfig{
				PreAllocatedVUs: null.IntFrom(tc.preAllocatedVUs),
				MaxVUs:          null.IntFrom(tc.maxVUs),
			}

			et, err := lib.NewExecutionTuple(newExecutionSegmentFromString(tc.segment), newExecutionSegmentSequenceFromString(tc.sequence))
			require.NoError(t, err)

			exp := []lib.ExecutionStep{{PlannedVUs: tc.expPlannedVUs, MaxUnplannedVUs: tc.expMaxUnplannedVUs}, {}}
			require.Equal(t, exp, config.GetExecutionRequirements(et))
		})
	}
}
