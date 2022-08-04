package lib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

//TODO: update test
/*
func TestSumStages(t *testing.T) {
	testdata := map[string]struct {
		Time   types.NullDuration
		Stages []Stage
	}{
		"Blank":    {types.NullDuration{}, []Stage{}},
		"Infinite": {types.NullDuration{}, []Stage{{}}},
		"Limit": {
			types.NullDurationFrom(10 * time.Second),
			[]Stage{
				{Duration: types.NullDurationFrom(5 * time.Second)},
				{Duration: types.NullDurationFrom(5 * time.Second)},
			},
		},
		"InfiniteTail": {
			types.NullDuration{Duration: types.Duration(10 * time.Second), Valid: false},
			[]Stage{
				{Duration: types.NullDurationFrom(5 * time.Second)},
				{Duration: types.NullDurationFrom(5 * time.Second)},
				{},
			},
		},
	}
	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, data.Time, SumStages(data.Stages))
		})
	}
}
*/

func TestMin(t *testing.T) {
	t.Parallel()
	assert.Equal(t, int64(10), Min(10, 100))
	assert.Equal(t, int64(10), Min(100, 10))
}

func TestMax(t *testing.T) {
	t.Parallel()
	assert.Equal(t, int64(100), Max(10, 100))
	assert.Equal(t, int64(100), Max(100, 10))
}
