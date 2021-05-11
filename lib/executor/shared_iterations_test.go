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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/metrics"
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
		slowVUID int64
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
			sid := atomic.LoadInt64(&slowVUID)
			if sid == int64(0) {
				atomic.StoreInt64(&slowVUID, state.Vu)
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
