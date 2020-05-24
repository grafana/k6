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
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v4"

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
	ctx, cancel, executor, _ := setupExecutor(
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
		800 * time.Millisecond,
	}

	errCh := make(chan error)
	go func() { errCh <- executor.Run(ctx, nil) }()

	result := make([]int64, len(sampleTimes))
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
	ctx, cancel, executor, _ := setupExecutor(
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

	result := make([]int64, len(sampleTimes)+rampDownSamples)
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

func TestVariableLoopingVUsConfigExecutionPlanExample(t *testing.T) {
	t.Parallel()
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	conf := NewVariableLoopingVUsConfig("test")
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

func TestVariableLoopingVUsConfigExecutionPlanExampleOneThird(t *testing.T) {
	t.Parallel()
	et, err := lib.NewExecutionTuple(newExecutionSegmentFromString("0:1/3"), nil)
	require.NoError(t, err)
	conf := NewVariableLoopingVUsConfig("test")
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

func TestVariableLoopingVUsExecutionTupleTests(t *testing.T) {
	t.Parallel()

	conf := NewVariableLoopingVUsConfig("test")
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
			rawStepsNoZeroEnd := conf.getRawExecutionSteps(et, false)
			assert.Equal(t, expectedSteps, rawStepsNoZeroEnd)
		})
	}
}

func TestVariableLoopingVUsGetRawExecutionStepsCornerCases(t *testing.T) {
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
		conf := NewVariableLoopingVUsConfig("test")
		conf.StartVUs = null.IntFrom(testCase.start)
		conf.Stages = testCase.stages
		et := testCase.et
		if et == nil {
			et = mustNewExecutionTuple(nil, nil)
		}
		expectedSteps := testCase.expectedSteps

		t.Run(testCase.name, func(t *testing.T) {
			rawStepsNoZeroEnd := conf.getRawExecutionSteps(et, false)
			assert.Equal(t, expectedSteps, rawStepsNoZeroEnd)
		})
	}
}

func BenchmarkVariableLoopingVUsGetRawExecutionSteps(b *testing.B) {
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
				vlvc := VariableLoopingVUsConfig{
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

func TestSegmentedIndex(t *testing.T) {
	// TODO ... more structure ?
	t.Run("full", func(t *testing.T) {
		s := segmentedIndex{start: 0, lcd: 1, offsets: []int64{1}}

		s.next()
		assert.EqualValues(t, 1, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.prev()
		assert.EqualValues(t, 0, s.unscaled)
		assert.EqualValues(t, 0, s.scaled)

		s.next()
		assert.EqualValues(t, 1, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.next()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.next()
		assert.EqualValues(t, 3, s.unscaled)
		assert.EqualValues(t, 3, s.scaled)

		s.prev()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.prev()
		assert.EqualValues(t, 1, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.next()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)
	})

	t.Run("half", func(t *testing.T) {
		s := segmentedIndex{start: 0, lcd: 2, offsets: []int64{2}}

		s.next()
		assert.EqualValues(t, 1, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.prev()
		assert.EqualValues(t, 0, s.unscaled)
		assert.EqualValues(t, 0, s.scaled)

		s.next()
		assert.EqualValues(t, 1, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.next()
		assert.EqualValues(t, 3, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.next()
		assert.EqualValues(t, 5, s.unscaled)
		assert.EqualValues(t, 3, s.scaled)

		s.prev()
		assert.EqualValues(t, 3, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.prev()
		assert.EqualValues(t, 1, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.prev()
		assert.EqualValues(t, 0, s.unscaled)
		assert.EqualValues(t, 0, s.scaled)

		s.next()
		assert.EqualValues(t, 1, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)
	})

	t.Run("the other half", func(t *testing.T) {
		s := segmentedIndex{start: 1, lcd: 2, offsets: []int64{2}}

		s.next()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.prev()
		assert.EqualValues(t, 0, s.unscaled)
		assert.EqualValues(t, 0, s.scaled)

		s.next()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.next()
		assert.EqualValues(t, 4, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.next()
		assert.EqualValues(t, 6, s.unscaled)
		assert.EqualValues(t, 3, s.scaled)

		s.prev()
		assert.EqualValues(t, 4, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.prev()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.prev()
		assert.EqualValues(t, 0, s.unscaled)
		assert.EqualValues(t, 0, s.scaled)

		s.next()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)
	})

	t.Run("strange", func(t *testing.T) {
		s := segmentedIndex{start: 1, lcd: 7, offsets: []int64{4, 3}}

		s.next()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.prev()
		assert.EqualValues(t, 0, s.unscaled)
		assert.EqualValues(t, 0, s.scaled)

		s.next()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.next()
		assert.EqualValues(t, 6, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.next()
		assert.EqualValues(t, 9, s.unscaled)
		assert.EqualValues(t, 3, s.scaled)

		s.prev()
		assert.EqualValues(t, 6, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.prev()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.prev()
		assert.EqualValues(t, 0, s.unscaled)
		assert.EqualValues(t, 0, s.scaled)

		s.next()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.goTo(6)
		assert.EqualValues(t, 6, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.goTo(5)
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.goTo(7)
		assert.EqualValues(t, 6, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.goTo(8)
		assert.EqualValues(t, 6, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.goTo(9)
		assert.EqualValues(t, 9, s.unscaled)
		assert.EqualValues(t, 3, s.scaled)

		s.prev()
		assert.EqualValues(t, 6, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.prev()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.prev()
		assert.EqualValues(t, 0, s.unscaled)
		assert.EqualValues(t, 0, s.scaled)
	})
}

// TODO: delete in favor of lib.generateRandomSequence() after
// https://github.com/loadimpact/k6/issues/1302 is done (can't import now due to
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

	seed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(seed))
	t.Logf("Random source seeded with %d\n", seed)

	const (
		numTests         = 10
		maxStages        = 10
		minStageDuration = 1 * time.Second
		maxStageDuration = 10 * time.Minute
		maxVUs           = 300
		segmentSeqMaxLen = 15
		maxNumerator     = 300
	)
	getTestConfig := func(name string) VariableLoopingVUsConfig {
		stagesCount := 1 + r.Int31n(maxStages)
		stages := make([]Stage, stagesCount)
		for s := int32(0); s < stagesCount; s++ {
			dur := (minStageDuration + time.Duration(r.Int63n(int64(maxStageDuration-minStageDuration)))).Round(time.Second)
			stages[s] = Stage{Duration: types.NullDurationFrom(dur), Target: null.IntFrom(r.Int63n(maxVUs))}
		}

		c := NewVariableLoopingVUsConfig(name)
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
			c := getTestConfig(name)
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
