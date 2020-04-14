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
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/types"
)

func TestVariableLoopingVUsRun(t *testing.T) {
	t.Parallel()

	config := VariableLoopingVUsConfig{
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
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(lib.Options{}, et, 10, 50)
	var ctx, cancel, executor, _ = setupExecutor(
		t, config, es,
		simpleRunner(func(ctx context.Context) error {
			// Sleeping for a weird duration somewhat offset from the
			// executor ticks to hopefully keep race conditions out of
			// our control from failing the test.
			time.Sleep(300 * time.Millisecond)
			atomic.AddInt64(&iterCount, 1)
			return nil
		}),
	)
	defer cancel()

	sampleTimes := []time.Duration{
		500 * time.Millisecond,
		1000 * time.Millisecond,
		700 * time.Millisecond,
	}

	errCh := make(chan error)
	go func() { errCh <- executor.Run(ctx, nil) }()

	var result = make([]int64, len(sampleTimes))
	for i, d := range sampleTimes {
		time.Sleep(d)
		result[i] = es.GetCurrentlyActiveVUsCount()
	}

	require.NoError(t, <-errCh)

	assert.Equal(t, []int64{5, 3, 0}, result)
	assert.Equal(t, int64(29), atomic.LoadInt64(&iterCount))
}

// Ensure there's no wobble of VUs during graceful ramp-down, without segments.
// See https://github.com/loadimpact/k6/issues/1296
func TestVariableLoopingVUsRampDownNoWobble(t *testing.T) {
	t.Parallel()

	config := VariableLoopingVUsConfig{
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

	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(lib.Options{}, et, 10, 50)
	var ctx, cancel, executor, _ = setupExecutor(
		t, config, es,
		simpleRunner(func(ctx context.Context) error {
			time.Sleep(1 * time.Second)
			return nil
		}),
	)
	defer cancel()

	sampleTimes := []time.Duration{
		100 * time.Millisecond,
		3000 * time.Millisecond,
	}
	const rampDownSamples = 50

	errCh := make(chan error)
	go func() { errCh <- executor.Run(ctx, nil) }()

	var result = make([]int64, len(sampleTimes)+rampDownSamples)
	for i, d := range sampleTimes {
		time.Sleep(d)
		result[i] = es.GetCurrentlyActiveVUsCount()
	}

	// Sample ramp-down at a higher rate
	for i := len(sampleTimes); i < rampDownSamples; i++ {
		time.Sleep(50 * time.Millisecond)
		result[i] = es.GetCurrentlyActiveVUsCount()
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
