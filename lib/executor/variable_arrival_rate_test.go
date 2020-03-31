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
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
)

func TestGetPlannedRateChanges0DurationStage(t *testing.T) {
	t.Parallel()
	var config = VariableArrivalRateConfig{
		TimeUnit:  types.NullDurationFrom(time.Second),
		StartRate: null.IntFrom(0),
		Stages: []Stage{
			{
				Duration: types.NullDurationFrom(0),
				Target:   null.IntFrom(50),
			},
			{
				Duration: types.NullDurationFrom(time.Minute),
				Target:   null.IntFrom(50),
			},
			{
				Duration: types.NullDurationFrom(0),
				Target:   null.IntFrom(100),
			},
			{
				Duration: types.NullDurationFrom(time.Minute),
				Target:   null.IntFrom(100),
			},
		},
	}
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	changes := config.getPlannedRateChanges(et)
	require.Equal(t, 2, len(changes))
	require.Equal(t, time.Duration(0), changes[0].timeOffset)
	require.Equal(t, types.NullDurationFrom(time.Millisecond*20), changes[0].tickerPeriod)

	require.Equal(t, time.Minute, changes[1].timeOffset)
	require.Equal(t, types.NullDurationFrom(time.Millisecond*10), changes[1].tickerPeriod)
}

// helper function to calculate the expected rate change at a given time
func calculateTickerPeriod(current, start, duration time.Duration, from, to int64) types.Duration {
	var coef = big.NewRat(
		(current - start).Nanoseconds(),
		duration.Nanoseconds(),
	)

	var oneRat = new(big.Rat).Mul(big.NewRat(from-to, 1), coef)
	oneRat = new(big.Rat).Sub(big.NewRat(from, 1), oneRat)
	oneRat = new(big.Rat).Mul(big.NewRat(int64(time.Second), 1), new(big.Rat).Inv(oneRat))
	return types.Duration(new(big.Int).Div(oneRat.Num(), oneRat.Denom()).Int64())
}

func TestGetPlannedRateChangesZeroDurationStart(t *testing.T) {
	// TODO: Make multiple of those tests
	t.Parallel()
	var config = VariableArrivalRateConfig{
		TimeUnit:  types.NullDurationFrom(time.Second),
		StartRate: null.IntFrom(0),
		Stages: []Stage{
			{
				Duration: types.NullDurationFrom(0),
				Target:   null.IntFrom(50),
			},
			{
				Duration: types.NullDurationFrom(time.Minute),
				Target:   null.IntFrom(50),
			},
			{
				Duration: types.NullDurationFrom(0),
				Target:   null.IntFrom(100),
			},
			{
				Duration: types.NullDurationFrom(time.Minute),
				Target:   null.IntFrom(100),
			},
			{
				Duration: types.NullDurationFrom(time.Minute),
				Target:   null.IntFrom(0),
			},
		},
	}

	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	changes := config.getPlannedRateChanges(et)
	var expectedTickerPeriod types.Duration
	for i, change := range changes {
		switch {
		case change.timeOffset == 0:
			expectedTickerPeriod = types.Duration(20 * time.Millisecond)
		case change.timeOffset == time.Minute*1:
			expectedTickerPeriod = types.Duration(10 * time.Millisecond)
		case change.timeOffset < time.Minute*3:
			expectedTickerPeriod = calculateTickerPeriod(change.timeOffset, 2*time.Minute, time.Minute, 100, 0)
		case change.timeOffset == time.Minute*3:
			expectedTickerPeriod = 0
		default:
			t.Fatalf("this shouldn't happen %d index %+v", i, change)
		}
		require.Equal(t, time.Duration(0),
			change.timeOffset%minIntervalBetweenRateAdjustments, "%d index %+v", i, change)
		require.Equal(t, change.tickerPeriod.Duration, expectedTickerPeriod, "%d index %+v", i, change)
	}
}

func TestGetPlannedRateChanges(t *testing.T) {
	// TODO: Make multiple of those tests
	t.Parallel()
	var config = VariableArrivalRateConfig{
		TimeUnit:  types.NullDurationFrom(time.Second),
		StartRate: null.IntFrom(0),
		Stages: []Stage{
			{
				Duration: types.NullDurationFrom(2 * time.Minute),
				Target:   null.IntFrom(50),
			},
			{
				Duration: types.NullDurationFrom(time.Minute),
				Target:   null.IntFrom(50),
			},
			{
				Duration: types.NullDurationFrom(time.Minute),
				Target:   null.IntFrom(100),
			},
			{
				Duration: types.NullDurationFrom(0),
				Target:   null.IntFrom(200),
			},

			{
				Duration: types.NullDurationFrom(time.Second * 23),
				Target:   null.IntFrom(50),
			},
		},
	}

	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	changes := config.getPlannedRateChanges(et)
	var expectedTickerPeriod types.Duration
	for i, change := range changes {
		switch {
		case change.timeOffset <= time.Minute*2:
			expectedTickerPeriod = calculateTickerPeriod(change.timeOffset, 0, time.Minute*2, 0, 50)
		case change.timeOffset < time.Minute*4:
			expectedTickerPeriod = calculateTickerPeriod(change.timeOffset, time.Minute*3, time.Minute, 50, 100)
		case change.timeOffset == time.Minute*4:
			expectedTickerPeriod = types.Duration(5 * time.Millisecond)
		default:
			expectedTickerPeriod = calculateTickerPeriod(change.timeOffset, 4*time.Minute, 23*time.Second, 200, 50)
		}
		require.Equal(t, time.Duration(0),
			change.timeOffset%minIntervalBetweenRateAdjustments, "%d index %+v", i, change)
		require.Equal(t, change.tickerPeriod.Duration, expectedTickerPeriod, "%d index %+v", i, change)
	}
}

func BenchmarkGetPlannedRateChanges(b *testing.B) {
	var config = VariableArrivalRateConfig{
		TimeUnit:  types.NullDurationFrom(time.Second),
		StartRate: null.IntFrom(0),
		Stages: []Stage{
			{
				Duration: types.NullDurationFrom(5 * time.Minute),
				Target:   null.IntFrom(5000),
			},
			{
				Duration: types.NullDurationFrom(50 * time.Minute),
				Target:   null.IntFrom(5000),
			},
			{
				Duration: types.NullDurationFrom(5 * time.Minute),
				Target:   null.IntFrom(0),
			},
		},
	}

	b.RunParallel(func(pb *testing.PB) {
		et, err := lib.NewExecutionTuple(nil, nil)
		require.NoError(b, err)
		for pb.Next() {
			changes := config.getPlannedRateChanges(et)

			require.Equal(b, time.Duration(0),
				changes[0].timeOffset%minIntervalBetweenRateAdjustments, "%+v", changes[0])
		}
	})
}

func getTestVariableArrivalRateConfig() VariableArrivalRateConfig {
	return VariableArrivalRateConfig{
		TimeUnit:  types.NullDurationFrom(time.Second),
		StartRate: null.IntFrom(10),
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

func TestVariableArrivalRateRunNotEnoughAllocatedVUsWarn(t *testing.T) {
	t.Parallel()
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(lib.Options{}, et, 10, 50)
	var ctx, cancel, executor, logHook = setupExecutor(
		t, getTestVariableArrivalRateConfig(), es,
		simpleRunner(func(ctx context.Context) error {
			time.Sleep(time.Second)
			return nil
		}),
	)
	defer cancel()
	var engineOut = make(chan stats.SampleContainer, 1000)
	err = executor.Run(ctx, engineOut)
	require.NoError(t, err)
	entries := logHook.Drain()
	require.NotEmpty(t, entries)
	for _, entry := range entries {
		require.Equal(t,
			"Insufficient VUs, reached 20 active VUs and cannot allocate more",
			entry.Message)
		require.Equal(t, logrus.WarnLevel, entry.Level)
	}
}

func TestVariableArrivalRateRunCorrectRate(t *testing.T) {
	t.Parallel()
	var count int64
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(lib.Options{}, et, 10, 50)
	var ctx, cancel, executor, logHook = setupExecutor(
		t, getTestVariableArrivalRateConfig(), es,
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
		require.InDelta(t, 10, currentCount, 1)

		time.Sleep(time.Second)
		currentCount = atomic.SwapInt64(&count, 0)
		// this is highly dependant on minIntervalBetweenRateAdjustments
		// TODO find out why this isn't 30 and fix it
		require.InDelta(t, 23, currentCount, 2)

		time.Sleep(time.Second)
		currentCount = atomic.SwapInt64(&count, 0)
		require.InDelta(t, 50, currentCount, 2)
	}()
	var engineOut = make(chan stats.SampleContainer, 1000)
	err = executor.Run(ctx, engineOut)
	wg.Wait()
	require.NoError(t, err)
	require.Empty(t, logHook.Drain())
}
