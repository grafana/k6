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
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
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
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(lib.Options{}, et, 10, 50)
	ctx, cancel, executor, logHook := setupExecutor(
		t, getTestRampingArrivalRateConfig(), es,
		simpleRunner(func(ctx context.Context) error {
			time.Sleep(time.Second)
			return nil
		}),
	)
	defer cancel()
	engineOut := make(chan stats.SampleContainer, 1000)
	err = executor.Run(ctx, engineOut)
	require.NoError(t, err)
	entries := logHook.Drain()
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
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(lib.Options{}, et, 10, 50)
	ctx, cancel, executor, logHook := setupExecutor(
		t, getTestRampingArrivalRateConfig(), es,
		simpleRunner(func(ctx context.Context) error {
			atomic.AddInt64(&count, 1)
			return nil
		}),
	)
	defer cancel()
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
	engineOut := make(chan stats.SampleContainer, 1000)
	err = executor.Run(ctx, engineOut)
	wg.Wait()
	require.NoError(t, err)
	require.Empty(t, logHook.Drain())
}

func TestRampingArrivalRateRunUnplannedVUs(t *testing.T) {
	t.Parallel()
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(lib.Options{}, et, 1, 3)
	var count int64
	var ch = make(chan struct{})  // closed when new unplannedVU is started and signal to get to next iterations
	var ch2 = make(chan struct{}) // closed when a second iteration was started on an old VU in order to test it won't start a second unplanned VU in parallel or at all
	runner := simpleRunner(func(ctx context.Context) error {
		cur := atomic.AddInt64(&count, 1)
		if cur == 1 {
			<-ch // wait to start again
		} else if cur == 2 {
			<-ch2 // wait to start again
		}

		return nil
	})
	var ctx, cancel, executor, logHook = setupExecutor(
		t, &RampingArrivalRateConfig{
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
		},
		es, runner)
	defer cancel()
	engineOut := make(chan stats.SampleContainer, 1000)
	es.SetInitVUFunc(func(_ context.Context, logger *logrus.Entry) (lib.InitializedVU, error) {
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
		return runner.NewVU(int64(es.GetUniqueVUIdentifier()), engineOut)
	})
	err = executor.Run(ctx, engineOut)
	assert.NoError(t, err)
	assert.Empty(t, logHook.Drain())

	droppedIters := sumMetricValues(engineOut, metrics.DroppedIterations.Name)
	assert.Equal(t, count+int64(droppedIters), int64(9))
}

func TestRampingArrivalRateRunCorrectRateWithSlowRate(t *testing.T) {
	t.Parallel()
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(lib.Options{}, et, 1, 3)
	var count int64
	var ch = make(chan struct{}) // closed when new unplannedVU is started and signal to get to next iterations
	runner := simpleRunner(func(ctx context.Context) error {
		cur := atomic.AddInt64(&count, 1)
		if cur == 1 {
			<-ch // wait to start again
		}

		return nil
	})
	var ctx, cancel, executor, logHook = setupExecutor(
		t, &RampingArrivalRateConfig{
			TimeUnit: types.NullDurationFrom(time.Second),
			Stages: []Stage{
				{
					Duration: types.NullDurationFrom(time.Second * 2),
					Target:   null.IntFrom(10),
				},
			},
			PreAllocatedVUs: null.IntFrom(1),
			MaxVUs:          null.IntFrom(3),
		},
		es, runner)
	defer cancel()
	var engineOut = make(chan stats.SampleContainer, 1000)
	es.SetInitVUFunc(func(_ context.Context, logger *logrus.Entry) (lib.InitializedVU, error) {
		t.Log("init")
		cur := atomic.LoadInt64(&count)
		require.Equal(t, cur, int64(1))
		time.Sleep(time.Millisecond * 200)
		close(ch)
		time.Sleep(time.Millisecond * 200)
		cur = atomic.LoadInt64(&count)
		require.NotEqual(t, cur, int64(1))

		return runner.NewVU(int64(es.GetUniqueVUIdentifier()), engineOut)
	})
	err = executor.Run(ctx, engineOut)
	assert.NoError(t, err)
	assert.Empty(t, logHook.Drain())
	assert.Equal(t, int64(0), es.GetCurrentlyActiveVUsCount())
	assert.Equal(t, int64(2), es.GetInitializedVUsCount())
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
		config          = RampingArrivalRateConfig{
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

	for _, testCase := range testCases {
		et := testCase.et
		expectedTimes := testCase.expectedTimes
		config.TimeUnit = types.NewNullDuration(testCase.timeUnit, true)
		if testCase.timeUnit == 0 {
			config.TimeUnit = types.NewNullDuration(defaultTimeUnit, true)
		}

		t.Run(fmt.Sprintf("%s timeunit %s", et, config.TimeUnit), func(t *testing.T) {
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
				assert.InEpsilon(t, expectedTime, change, 0.001, "%s %s", expectedTime, change)
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
	z.SetFrac(ns.Rsh(ni, uint(ni.BitLen())/2), ds.Rsh(di, uint(di.BitLen())/2))
	for i := 10; i > 0; i-- { // TODO: better termination
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
				dur  = big.NewRat(time.Duration(stage.Duration.Duration).Nanoseconds(), 1)
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
			endCount.Add(endCount, new(big.Rat).Mul(big.NewRat(target, 1), big.NewRat(time.Duration(stage.Duration.Duration).Nanoseconds(), time.Duration(varc.TimeUnit.Duration).Nanoseconds())))

			for ; endCount.Cmp(iRat) >= 0; iRat.Add(iRat, big.NewRat(next(), 1)) {
				res := new(big.Rat).Sub(iRat, doneSoFar) // this can get next added to it but will need to change the above for .. so
				r, _ := res.Mul(res, step).Float64()
				ch <- base + time.Duration(r)
				first.Add(first, step)
			}
		}
		doneSoFar.Set(endCount) // copy
		curr = target
		base += time.Duration(stage.Duration.Duration)
	}
}
