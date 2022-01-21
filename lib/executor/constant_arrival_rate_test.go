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

	"github.com/benbjohnson/clock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/lib/testutils/minirunner"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/stats"
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
	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)

	err = executor.Run(ctx, engineOut, builtinMetrics)
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
	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	err = executor.Run(ctx, engineOut, builtinMetrics)
	wg.Wait()
	require.NoError(t, err)
	require.Empty(t, logHook.Drain())
}

//nolint:tparallel,paralleltest // this is flaky if ran with other tests
func TestConstantArrivalRateRunCorrectTiming(t *testing.T) {
	// t.Parallel()
	tests := []struct {
		segment                *lib.ExecutionSegment
		sequence               *lib.ExecutionSegmentSequence
		wantExecutionStartTime time.Duration
		steps                  []int64
	}{
		{
			segment:                newExecutionSegmentFromString("0:1/3"),
			wantExecutionStartTime: time.Millisecond * 20,
			steps:                  []int64{40, 60, 60, 60, 60, 60, 60},
		},
		// {
		// 	segment: newExecutionSegmentFromString("1/3:2/3"),
		// 	start:   time.Millisecond * 20,
		// 	steps:   []int64{60, 60, 60, 60, 60, 60, 40},
		// },
		// {
		// 	segment: newExecutionSegmentFromString("2/3:1"),
		// 	start:   time.Millisecond * 20,
		// 	steps:   []int64{40, 60, 60, 60, 60, 60, 60},
		// },
		// {
		// 	segment: newExecutionSegmentFromString("1/6:3/6"),
		// 	start:   time.Millisecond * 20,
		// 	steps:   []int64{40, 80, 40, 80, 40, 80, 40},
		// },
		// {
		// 	segment:  newExecutionSegmentFromString("1/6:3/6"),
		// 	sequence: newExecutionSegmentSequenceFromString("1/6,3/6"),
		// 	start:    time.Millisecond * 20,
		// 	steps:    []int64{40, 80, 40, 80, 40, 80, 40},
		// },
		// // sequences
		// {
		// 	segment:  newExecutionSegmentFromString("0:1/3"),
		// 	sequence: newExecutionSegmentSequenceFromString("0,1/3,2/3,1"),
		// 	start:    time.Millisecond * 0,
		// 	steps:    []int64{60, 60, 60, 60, 60, 60, 40},
		// },
		// {
		// 	segment:  newExecutionSegmentFromString("1/3:2/3"),
		// 	sequence: newExecutionSegmentSequenceFromString("0,1/3,2/3,1"),
		// 	start:    time.Millisecond * 20,
		// 	steps:    []int64{60, 60, 60, 60, 60, 60, 40},
		// },
		// {
		// 	segment:  newExecutionSegmentFromString("2/3:1"),
		// 	sequence: newExecutionSegmentSequenceFromString("0,1/3,2/3,1"),
		// 	start:    time.Millisecond * 40,
		// 	steps:    []int64{60, 60, 60, 60, 60, 100},
		// },
	}
	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
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
			var numberOfRuns int64
			seconds := 2
			config := getTestConstantArrivalRateConfig()
			mockClock := clock.New()
			config.clock = mockClock
			config.Duration.Duration = types.Duration(time.Second * time.Duration(seconds))
			newET, err := es.ExecutionTuple.GetNewExecutionTupleFromValue(config.MaxVUs.Int64)
			require.NoError(t, err)
			// FIXME: Ignore above

			numberOfIterationsPerSegment := newET.ScaleInt64(config.Rate.Int64)
			startTime := config.clock.Now()
			expectedTimeInt64 := int64(test.wantExecutionStartTime)
			ctx, cancel, executor, logHook := setupExecutor(
				t, config, es,
				simpleRunner(func(ctx context.Context) error {
					// HERE we don't care about what the runner does.
					// Instead, we're verifying that the executor interacts
					// with it at the moment(s) we expect. Because the role
					// of the executor as we understand it, is to do just that:
					// execute our VU script code at the right moment and interval (Orchestrator).
					//
					// simpleRunner = 1 iteration (synonyms)
					//
					// * simpleRunner, here, registers itself as running through the count
					//   atomic variable
					currentRunCount := atomic.AddInt64(&numberOfRuns, 1)

					fmt.Printf("currentRunCount=%d\n", currentRunCount)

					expectedRunTime := test.wantExecutionStartTime
					if currentRunCount != 1 {
						// et = et + {value at step X} ms
						expectedRunTime = time.Duration(atomic.AddInt64(&expectedTimeInt64, int64(time.Millisecond)*test.steps[(currentRunCount-2)%int64(len(test.steps))]))
					}

					assert.WithinDuration(t,
						startTime.Add(expectedRunTime),
						config.clock.Now(),
						time.Microsecond*2,
						"%d expectedTime %s", currentRunCount, expectedRunTime,
					)

					return nil
				}),
			)

			defer cancel()
			var wg sync.WaitGroup
			wg.Add(1)

			// This is some testing-oriented control execut
			go func() {
				defer wg.Done()
				// check that we got around the amount of VU iterations as we would expect
				var currentRunCount int64 = 0

				for i := 0; i < seconds; i++ {
					time.Sleep(time.Second)
					currentRunCount = atomic.LoadInt64(&numberOfRuns) // Read NumberOfRuns
					// TODO: put the comment, why here is 3
					fmt.Printf("numberOfIterationsPerSegment=%d, currentRunCount=%d\n", numberOfIterationsPerSegment, currentRunCount)
					assert.InDelta(t, int64(i+1)*numberOfIterationsPerSegment, currentRunCount, 3) // Is NumberOfRuns within
				}
			}()
			startTime = time.Now()
			engineOut := make(chan stats.SampleContainer, 1000)
			err = executor.Run(ctx, engineOut, builtinMetrics)
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
	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
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
				errCh <- executor.Run(ctx, engineOut, builtinMetrics)
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
	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	err = executor.Run(ctx, engineOut, builtinMetrics)
	require.NoError(t, err)
	logs := logHook.Drain()
	require.Len(t, logs, 1)
	assert.Contains(t, logs[0].Message, "cannot initialize more")
	assert.Equal(t, int64(5), count)
	assert.Equal(t, float64(5), sumMetricValues(engineOut, metrics.DroppedIterationsName))
}

func TestConstantArrivalRateGlobalIters(t *testing.T) {
	t.Parallel()

	config := &ConstantArrivalRateConfig{
		BaseConfig:      BaseConfig{GracefulStop: types.NullDurationFrom(100 * time.Millisecond)},
		TimeUnit:        types.NullDurationFrom(950 * time.Millisecond),
		Rate:            null.IntFrom(20),
		Duration:        types.NullDurationFrom(1 * time.Second),
		PreAllocatedVUs: null.IntFrom(5),
		MaxVUs:          null.IntFrom(5),
	}

	testCases := []struct {
		seq, seg string
		expIters []uint64
	}{
		{"0,1/4,3/4,1", "0:1/4", []uint64{1, 6, 11, 16, 21}},
		{"0,1/4,3/4,1", "1/4:3/4", []uint64{0, 2, 4, 5, 7, 9, 10, 12, 14, 15, 17, 19, 20}},
		{"0,1/4,3/4,1", "3/4:1", []uint64{3, 8, 13, 18}},
	}

	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("%s_%s", tc.seq, tc.seg), func(t *testing.T) {
			t.Parallel()
			ess, err := lib.NewExecutionSegmentSequenceFromString(tc.seq)
			require.NoError(t, err)
			seg, err := lib.NewExecutionSegmentFromString(tc.seg)
			require.NoError(t, err)
			et, err := lib.NewExecutionTuple(seg, &ess)
			require.NoError(t, err)
			es := lib.NewExecutionState(lib.Options{}, et, 5, 5)

			runner := &minirunner.MiniRunner{}
			ctx, cancel, executor, _ := setupExecutor(t, config, es, runner)
			defer cancel()

			gotIters := []uint64{}
			var mx sync.Mutex
			runner.Fn = func(ctx context.Context, _ chan<- stats.SampleContainer) error {
				state := lib.GetState(ctx)
				mx.Lock()
				gotIters = append(gotIters, state.GetScenarioGlobalVUIter())
				mx.Unlock()
				return nil
			}

			engineOut := make(chan stats.SampleContainer, 100)
			err = executor.Run(ctx, engineOut, builtinMetrics)
			require.NoError(t, err)
			assert.Equal(t, tc.expIters, gotIters)
		})
	}
}
