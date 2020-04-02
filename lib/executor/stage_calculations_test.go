package executor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
	null "gopkg.in/guregu/null.v3"
)

func TestStageCalculation(t *testing.T) {
	t.Parallel()

	var (
		defaultTimeUnit = time.Second.Nanoseconds()
		StartRate       = int64(0)
		Stages          = []Stage{ // TODO make this even bigger and longer .. will need more time
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
		}
	)

	testCases := []struct {
		expectedTimes []time.Duration
		et            *lib.ExecutionTuple
		timeUnit      int64
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
			timeUnit: time.Second.Nanoseconds() / 3, // three  times as fast
		},
		// TODO: extend more
	}

	for _, testCase := range testCases {
		et := testCase.et
		expectedTimes := testCase.expectedTimes
		timeUnit := testCase.timeUnit
		if timeUnit == 0 {
			timeUnit = defaultTimeUnit
		}

		t.Run(et.String(), func(t *testing.T) {
			start, offsets, _ := et.GetStripedOffsets()
			stc := newStageTransitionCalculator(start, offsets, StartRate, timeUnit, Stages)

			for i, expectedTime := range expectedTimes {
				require.True(t, stc.more(), "%d", i)
				value := stc.nextEvent()
				assert.InEpsilon(t, expectedTime, value, 0.001, "%s %s", expectedTime, value)
			}
			require.False(t, stc.more())
		})
	}
}
