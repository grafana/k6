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

func newExecutionSegmentFromString(str string) *lib.ExecutionSegment {
	r, err := lib.NewExecutionSegmentFromString(str)
	if err != nil {
		panic(err)
	}
	return r
}

func newExecutionSegmentSequenceFromString(str string) *lib.ExecutionSegmentSequence {
	r, err := lib.NewExecutionSegmentSequenceFromString(str)
	if err != nil {
		panic(err)
	}
	return &r
}

func getTestConstantArrivalRateConfig() *ConstantArrivalRateConfig {
	return &ConstantArrivalRateConfig{
		BaseConfig:      BaseConfig{GracefulStop: types.NullDurationFrom(1 * time.Second)},
		TimeUnit:        types.NullDurationFrom(time.Second),
		Rate:            null.IntFrom(50),
		Duration:        types.NullDurationFrom(5 * time.Second),
		PreAllocatedVUs: null.IntFrom(10),
		MaxVUs:          null.IntFrom(20),
	}
}

func TestConstantArrivalRateRunNotEnoughAllocatedVUsWarn(t *testing.T) {
	t.Parallel()
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(lib.Options{}, et, 10, 50)
	ctx, cancel, executor, logHook := setupExecutor(
		t, getTestConstantArrivalRateConfig(), es,
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

func TestConstantArrivalRateRunCorrectRate(t *testing.T) {
	t.Parallel()
	var count int64
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(lib.Options{}, et, 10, 50)
	ctx, cancel, executor, logHook := setupExecutor(
		t, getTestConstantArrivalRateConfig(), es,
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

		for i := 0; i < 5; i++ {
			time.Sleep(time.Second)
			currentCount = atomic.SwapInt64(&count, 0)
			require.InDelta(t, 50, currentCount, 1)
		}
	}()
	engineOut := make(chan stats.SampleContainer, 1000)
	err = executor.Run(ctx, engineOut)
	wg.Wait()
	require.NoError(t, err)
	require.Empty(t, logHook.Drain())
}

func TestConstantArrivalRateRunCorrectTiming(t *testing.T) {
	tests := []struct {
		segment  *lib.ExecutionSegment
		sequence *lib.ExecutionSegmentSequence
		start    time.Duration
		steps    []int64
	}{
		{
			segment: newExecutionSegmentFromString("0:1/3"),
			start:   time.Millisecond * 20,
			steps:   []int64{40, 60, 60, 60, 60, 60, 60},
		},
		{
			segment: newExecutionSegmentFromString("1/3:2/3"),
			start:   time.Millisecond * 20,
			steps:   []int64{60, 60, 60, 60, 60, 60, 40},
		},
		{
			segment: newExecutionSegmentFromString("2/3:1"),
			start:   time.Millisecond * 20,
			steps:   []int64{40, 60, 60, 60, 60, 60, 60},
		},
		{
			segment: newExecutionSegmentFromString("1/6:3/6"),
			start:   time.Millisecond * 20,
			steps:   []int64{40, 80, 40, 80, 40, 80, 40},
		},
		{
			segment:  newExecutionSegmentFromString("1/6:3/6"),
			sequence: newExecutionSegmentSequenceFromString("1/6,3/6"),
			start:    time.Millisecond * 20,
			steps:    []int64{40, 80, 40, 80, 40, 80, 40},
		},
		// sequences
		{
			segment:  newExecutionSegmentFromString("0:1/3"),
			sequence: newExecutionSegmentSequenceFromString("0,1/3,2/3,1"),
			start:    time.Millisecond * 0,
			steps:    []int64{60, 60, 60, 60, 60, 60, 40},
		},
		{
			segment:  newExecutionSegmentFromString("1/3:2/3"),
			sequence: newExecutionSegmentSequenceFromString("0,1/3,2/3,1"),
			start:    time.Millisecond * 20,
			steps:    []int64{60, 60, 60, 60, 60, 60, 40},
		},
		{
			segment:  newExecutionSegmentFromString("2/3:1"),
			sequence: newExecutionSegmentSequenceFromString("0,1/3,2/3,1"),
			start:    time.Millisecond * 40,
			steps:    []int64{60, 60, 60, 60, 60, 100},
		},
	}
	for _, test := range tests {
		test := test

		t.Run(fmt.Sprintf("segment %s sequence %s", test.segment, test.sequence), func(t *testing.T) {
			t.Parallel()
			et, err := lib.NewExecutionTuple(test.segment, test.sequence)
			require.NoError(t, err)
			es := lib.NewExecutionState(lib.Options{
				ExecutionSegment:         test.segment,
				ExecutionSegmentSequence: test.sequence,
			}, et, 10, 50)
			var count int64
			seconds := 2
			config := getTestConstantArrivalRateConfig()
			config.Duration.Duration = types.Duration(time.Second * time.Duration(seconds))
			newET, err := es.ExecutionTuple.GetNewExecutionTupleFromValue(config.MaxVUs.Int64)
			require.NoError(t, err)
			rateScaled := newET.ScaleInt64(config.Rate.Int64)
			startTime := time.Now()
			expectedTimeInt64 := int64(test.start)
			ctx, cancel, executor, logHook := setupExecutor(
				t, config, es,
				simpleRunner(func(ctx context.Context) error {
					current := atomic.AddInt64(&count, 1)

					expectedTime := test.start
					if current != 1 {
						expectedTime = time.Duration(atomic.AddInt64(&expectedTimeInt64,
							int64(time.Millisecond)*test.steps[(current-2)%int64(len(test.steps))]))
					}
					assert.WithinDuration(t,
						startTime.Add(expectedTime),
						time.Now(),
						time.Millisecond*10,
						"%d expectedTime %s", current, expectedTime,
					)

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

				for i := 0; i < seconds; i++ {
					time.Sleep(time.Second)
					currentCount = atomic.LoadInt64(&count)
					assert.InDelta(t, int64(i+1)*rateScaled, currentCount, 3)
				}
			}()
			startTime = time.Now()
			engineOut := make(chan stats.SampleContainer, 1000)
			err = executor.Run(ctx, engineOut)
			wg.Wait()
			require.NoError(t, err)
			require.Empty(t, logHook.Drain())
		})
	}
}

func TestArrivalRateCancel(t *testing.T) {
	t.Parallel()

	testCases := map[string]lib.ExecutorConfig{
		"constant": getTestConstantArrivalRateConfig(),
		"ramping":  getTestRampingArrivalRateConfig(),
	}
	for name, config := range testCases {
		config := config
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ch := make(chan struct{})
			errCh := make(chan error, 1)
			weAreDoneCh := make(chan struct{})
			et, err := lib.NewExecutionTuple(nil, nil)
			require.NoError(t, err)
			es := lib.NewExecutionState(lib.Options{}, et, 10, 50)
			ctx, cancel, executor, logHook := setupExecutor(
				t, config, es, simpleRunner(func(ctx context.Context) error {
					select {
					case <-ch:
						<-ch
					default:
					}
					return nil
				}))
			defer cancel()
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()

				engineOut := make(chan stats.SampleContainer, 1000)
				errCh <- executor.Run(ctx, engineOut)
				close(weAreDoneCh)
			}()

			time.Sleep(time.Second)
			ch <- struct{}{}
			cancel()
			time.Sleep(time.Second)
			select {
			case <-weAreDoneCh:
				t.Fatal("Run returned before all VU iterations were finished")
			default:
			}
			close(ch)
			<-weAreDoneCh
			wg.Wait()
			require.NoError(t, <-errCh)
			require.Empty(t, logHook.Drain())
		})
	}
}

func TestConstantArrivalRateDroppedIterations(t *testing.T) {
	t.Parallel()
	var count int64
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)

	config := &ConstantArrivalRateConfig{
		BaseConfig:      BaseConfig{GracefulStop: types.NullDurationFrom(0 * time.Second)},
		TimeUnit:        types.NullDurationFrom(time.Second),
		Rate:            null.IntFrom(10),
		Duration:        types.NullDurationFrom(950 * time.Millisecond),
		PreAllocatedVUs: null.IntFrom(5),
		MaxVUs:          null.IntFrom(5),
	}

	es := lib.NewExecutionState(lib.Options{}, et, 10, 50)
	ctx, cancel, executor, logHook := setupExecutor(
		t, config, es,
		simpleRunner(func(ctx context.Context) error {
			atomic.AddInt64(&count, 1)
			<-ctx.Done()
			return nil
		}),
	)
	defer cancel()
	engineOut := make(chan stats.SampleContainer, 1000)
	err = executor.Run(ctx, engineOut)
	require.NoError(t, err)
	logs := logHook.Drain()
	require.Len(t, logs, 1)
	assert.Contains(t, logs[0].Message, "cannot initialize more")
	assert.Equal(t, int64(5), count)
	assert.Equal(t, float64(5), sumMetricValues(engineOut, metrics.DroppedIterations.Name))
}
