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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/lib/testutils/minirunner"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/stats"
)

func getTestSharedIterationsConfig() SharedIterationsConfig {
	return SharedIterationsConfig{
		VUs:         null.IntFrom(10),
		Iterations:  null.IntFrom(100),
		MaxDuration: types.NullDurationFrom(5 * time.Second),
	}
}

// Baseline test
func TestSharedIterationsRun(t *testing.T) {
	t.Parallel()
	var doneIters uint64
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(lib.Options{}, et, 10, 50)
	var ctx, cancel, executor, _ = setupExecutor(
		t, getTestSharedIterationsConfig(), es,
		simpleRunner(func(ctx context.Context) error {
			atomic.AddUint64(&doneIters, 1)
			return nil
		}),
	)
	defer cancel()
	err = executor.Run(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, uint64(100), doneIters)
}

// Test that when one VU "slows down", others will pick up the workload.
// This is the reverse behavior of the PerVUIterations executor.
func TestSharedIterationsRunVariableVU(t *testing.T) {
	t.Parallel()
	var (
		result   sync.Map
		slowVUID uint64
	)
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(lib.Options{}, et, 10, 50)
	var ctx, cancel, executor, _ = setupExecutor(
		t, getTestSharedIterationsConfig(), es,
		simpleRunner(func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond) // small wait to stabilize the test
			state := lib.GetState(ctx)
			// Pick one VU randomly and always slow it down.
			sid := atomic.LoadUint64(&slowVUID)
			if sid == uint64(0) {
				atomic.StoreUint64(&slowVUID, state.Vu)
			}
			if sid == state.Vu {
				time.Sleep(200 * time.Millisecond)
			}
			currIter, _ := result.LoadOrStore(state.Vu, uint64(0))
			result.Store(state.Vu, currIter.(uint64)+1)
			return nil
		}),
	)
	defer cancel()
	err = executor.Run(ctx, nil)
	require.NoError(t, err)

	var totalIters uint64
	result.Range(func(key, value interface{}) bool {
		totalIters += value.(uint64)
		return true
	})

	// The slow VU should complete 2 iterations given these timings,
	// while the rest should randomly complete the other 98 iterations.
	val, ok := result.Load(slowVUID)
	assert.True(t, ok)
	assert.Equal(t, uint64(2), val)
	assert.Equal(t, uint64(100), totalIters)
}

func TestSharedIterationsEmitDroppedIterations(t *testing.T) {
	t.Parallel()
	var count int64
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)

	config := &SharedIterationsConfig{
		VUs:         null.IntFrom(5),
		Iterations:  null.IntFrom(100),
		MaxDuration: types.NullDurationFrom(1 * time.Second),
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
	assert.Empty(t, logHook.Drain())
	assert.Equal(t, int64(5), count)
	assert.Equal(t, float64(95), sumMetricValues(engineOut, metrics.DroppedIterations.Name))
}

func TestSharedIterationsGlobalIters(t *testing.T) {
	t.Parallel()

	config := &SharedIterationsConfig{
		VUs:         null.IntFrom(5),
		Iterations:  null.IntFrom(50),
		MaxDuration: types.NullDurationFrom(1 * time.Second),
	}

	testCases := []struct {
		seq, seg string
		expIters []int64
	}{
		{"0,1/4,3/4,1", "0:1/4", []int64{1, 6, 11, 16, 21, 26, 31, 36, 41, 46}},
		{"0,1/4,3/4,1", "1/4:3/4", []int64{0, 2, 4, 5, 7, 9, 10, 12, 14, 15, 17, 19, 20, 22, 24, 25, 27, 29, 30, 32, 34, 35, 37, 39, 40, 42, 44, 45, 47, 49}},
		{"0,1/4,3/4,1", "3/4:1", []int64{3, 8, 13, 18, 23, 28, 33, 38, 43, 48}},
	}

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

			gotIters := []int64{}
			var mx sync.Mutex
			runner.Fn = func(ctx context.Context, _ chan<- stats.SampleContainer) error {
				state := lib.GetState(ctx)
				mx.Lock()
				gotIters = append(gotIters, state.GetScenarioGlobalVUIter())
				mx.Unlock()
				return nil
			}

			engineOut := make(chan stats.SampleContainer, 100)
			err = executor.Run(ctx, engineOut)
			require.NoError(t, err)
			assert.Equal(t, tc.expIters, gotIters)
		})
	}
}
