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

func getTestPerVUIterationsConfig() PerVUIterationsConfig {
	return PerVUIterationsConfig{
		BaseConfig:  BaseConfig{GracefulStop: types.NullDurationFrom(1 * time.Second)},
		VUs:         null.IntFrom(10),
		Iterations:  null.IntFrom(100),
		MaxDuration: types.NullDurationFrom(3 * time.Second),
	}
}

// Baseline test
func TestPerVUIterationsRun(t *testing.T) {
	t.Parallel()

	// Arrange
	var result sync.Map

	executionTuple, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)

	executionState := lib.NewExecutionState(lib.Options{}, executionTuple, 10, 50)
	vuFn := func(ctx context.Context) error {
		state := lib.GetState(ctx)
		currentIteration, _ := result.LoadOrStore(state.VUID, uint64(0))
		result.Store(state.VUID, currentIteration.(uint64)+1)
		return nil
	}

	ctx, cancel, executor, _ := setupExecutor(
		t,
		getTestPerVUIterationsConfig(),
		executionState,
		simpleRunner(vuFn),
	)
	defer cancel()

	engineOut := make(chan stats.SampleContainer, 1000)
	builtinMetrics := metrics.RegisterBuiltinMetrics(metrics.NewRegistry())

	// Act
	err = executor.Run(ctx, engineOut, builtinMetrics)

	// Assert
	require.NoError(t, err)

	var totalIters uint64
	result.Range(func(key, value interface{}) bool {
		vuIters := value.(uint64)
		assert.Equal(t, uint64(100), vuIters)
		totalIters += vuIters
		return true
	})
	assert.Equal(t, uint64(1000), totalIters)
}

// Test that when one VU "slows down", others will *not* pick up the workload.
// This is the reverse behavior of the SharedIterations executor.
func TestPerVUIterationsRunVariableVU(t *testing.T) {
	t.Parallel()

	// Arrange
	var result sync.Map
	var slowVUID uint64 = 1

	executionTuple, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	executionState := lib.NewExecutionState(lib.Options{}, executionTuple, 10, 50)
	vuFn := func(ctx context.Context) error {
		state := lib.GetState(ctx)
		if state.VUID == slowVUID {
			time.Sleep(200 * time.Millisecond)
		}
		currentIteration, _ := result.LoadOrStore(state.VUID, uint64(0))
		result.Store(state.VUID, currentIteration.(uint64)+1)
		return nil
	}

	ctx, cancel, executor, _ := setupExecutor(t, getTestPerVUIterationsConfig(), executionState, simpleRunner(vuFn))
	defer cancel()

	engineOut := make(chan stats.SampleContainer, 1000)
	builtinMetrics := metrics.RegisterBuiltinMetrics(metrics.NewRegistry())

	// Act
	err = executor.Run(ctx, engineOut, builtinMetrics)
	val, ok := result.Load(slowVUID)

	// Assert
	require.NoError(t, err)
	assert.True(t, ok)

	var totalIterations uint64
	result.Range(func(key, value interface{}) bool {
		vuIters := value.(uint64)
		if key != slowVUID {
			assert.Equal(t, uint64(100), vuIters)
		}
		totalIterations += vuIters
		return true
	})

	// The slow VU should complete 15 iterations given these timings,
	// while the rest should equally complete their assigned 100 iterations.
	assert.Equal(t, uint64(15), val)
	assert.Equal(t, uint64(915), totalIterations)
}

func TestPerVuIterationsEmitDroppedIterations(t *testing.T) {
	t.Parallel()

	// Arrange
	var count int64

	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)

	config := PerVUIterationsConfig{
		VUs:         null.IntFrom(5),
		Iterations:  null.IntFrom(20),
		MaxDuration: types.NullDurationFrom(1 * time.Second),
	}

	es := lib.NewExecutionState(lib.Options{}, et, 10, 50)
	vuFn := func(ctx context.Context) error {
		atomic.AddInt64(&count, 1)
		<-ctx.Done()
		return nil
	}
	ctx, cancel, executor, logHook := setupExecutor(t, config, es, simpleRunner(vuFn))
	defer cancel()

	engineOut := make(chan stats.SampleContainer, 1000)
	builtinMetrics := metrics.RegisterBuiltinMetrics(metrics.NewRegistry())

	// Act
	err = executor.Run(ctx, engineOut, builtinMetrics)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, logHook.Drain())
	assert.Equal(t, int64(5), count)
	assert.Equal(t, float64(95), sumMetricValues(engineOut, metrics.DroppedIterationsName))
}
