package expv2

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/output/cloud/expv2/pbcloud"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestResolveBucketIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in  float64
		exp uint32
	}{
		{in: -1029, exp: 0},
		{in: -12, exp: 0},
		{in: -0.82673, exp: 0},
		{in: 0, exp: 0},
		{in: 0.12, exp: 1},
		{in: 1.91, exp: 2},
		{in: 10, exp: 10},
		{in: 12, exp: 12},
		{in: 12.5, exp: 13},
		{in: 20, exp: 20},
		{in: 255, exp: 255},
		{in: 256, exp: 256},
		{in: 282.29, exp: 269},
		{in: 1029, exp: 512},
		{in: 39751, exp: 1179},
		{in: (1 << 30) - 1, exp: 3071},
		{in: (1 << 30), exp: 3072},
		{in: math.MaxInt32, exp: 3199},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.exp, resolveBucketIndex(tc.in), tc.in)
	}
}

func TestHistogramAddWithSimpleValue(t *testing.T) {
	t.Parallel()

	// Zero as value
	res := histogram{}
	res.Add(0)
	exp := histogram{
		Buckets:         map[uint32]uint32{0: 1},
		Indexes:         []uint32{0},
		ExtraLowBucket:  0,
		ExtraHighBucket: 0,
		Max:             0,
		Min:             0,
		Sum:             0,
		Count:           1,
	}
	require.Equal(t, exp, res)

	// Add a lower bucket index within slice capacity
	res = histogram{}
	res.Add(8)
	res.Add(5)

	exp = histogram{
		Buckets:         map[uint32]uint32{5: 1, 8: 1},
		Indexes:         []uint32{5, 8},
		ExtraLowBucket:  0,
		ExtraHighBucket: 0,
		Max:             8,
		Min:             5,
		Sum:             13,
		Count:           2,
	}
	require.Equal(t, exp, res)

	// Add a higher bucket index within slice capacity
	res = histogram{}
	res.Add(100)
	res.Add(101)

	exp = histogram{
		Buckets:         map[uint32]uint32{100: 1, 101: 1},
		Indexes:         []uint32{100, 101},
		ExtraLowBucket:  0,
		ExtraHighBucket: 0,
		Max:             101,
		Min:             100,
		Sum:             201,
		Count:           2,
	}
	require.Equal(t, exp, res)

	// Same case but reversed test check
	res = histogram{}
	res.Add(101)
	res.Add(100)

	exp = histogram{
		Buckets:         map[uint32]uint32{100: 1, 101: 1},
		Indexes:         []uint32{100, 101},
		ExtraLowBucket:  0,
		ExtraHighBucket: 0,
		Max:             101,
		Min:             100,
		Sum:             201,
		Count:           2,
	}
	assert.Equal(t, exp, res)

	// One more complex case with lower index and more than two indexes
	res = histogram{}
	res.Add(8)
	res.Add(9)
	res.Add(10)
	res.Add(5)

	exp = histogram{
		Buckets:         map[uint32]uint32{8: 1, 9: 1, 10: 1, 5: 1},
		Indexes:         []uint32{5, 8, 9, 10},
		ExtraLowBucket:  0,
		ExtraHighBucket: 0,
		Max:             10,
		Min:             5,
		Sum:             32,
		Count:           4,
	}

	assert.Equal(t, exp, res)
}

func TestHistogramAddWithUntrackables(t *testing.T) {
	t.Parallel()

	res := histogram{}
	for _, v := range []float64{5, -3.14, 2 * 1e9, 1} {
		res.Add(v)
	}

	exp := histogram{
		Buckets:         map[uint32]uint32{1: 1, 5: 1},
		Indexes:         []uint32{1, 5},
		ExtraLowBucket:  1,
		ExtraHighBucket: 1,
		Max:             2 * 1e9,
		Min:             -3.14,
		Sum:             2*1e9 + 5 + 1 - 3.14,
		Count:           4,
	}
	assert.Equal(t, exp, res)
}

func TestHistogramAddWithMultipleOccurances(t *testing.T) {
	t.Parallel()

	res := histogram{}
	for _, v := range []float64{51.8, 103.6, 103.6, 103.6, 103.6} {
		res.Add(v)
	}

	exp := histogram{
		Buckets:         map[uint32]uint32{52: 1, 104: 4},
		Indexes:         []uint32{52, 104},
		Max:             103.6,
		Min:             51.8,
		ExtraLowBucket:  0,
		ExtraHighBucket: 0,
		Sum:             466.20000000000005,
		Count:           5,
	}
	assert.Equal(t, exp, res)
}

func TestHistogramAddWithNegativeNum(t *testing.T) {
	t.Parallel()

	res := histogram{}
	res.Add(-2.42314)

	exp := histogram{
		Max:             -2.42314,
		Min:             -2.42314,
		Buckets:         nil,
		ExtraLowBucket:  1,
		ExtraHighBucket: 0,
		Sum:             -2.42314,
		Count:           1,
	}
	assert.Equal(t, exp, res)
}

func TestHistogramAddWithMultipleNegativeNums(t *testing.T) {
	t.Parallel()
	res := histogram{}
	for _, v := range []float64{-0.001, -0.001, -0.001} {
		res.Add(v)
	}

	exp := histogram{
		Buckets:         nil,
		ExtraLowBucket:  3,
		ExtraHighBucket: 0,
		Max:             -0.001,
		Min:             -0.001,
		Sum:             -0.003,
		Count:           3,
	}
	assert.Equal(t, exp, res)
}

func TestNewHistoramWithNoVals(t *testing.T) {
	t.Parallel()

	res := histogram{}
	exp := histogram{
		Buckets:         nil,
		ExtraLowBucket:  0,
		ExtraHighBucket: 0,
		Max:             0,
		Min:             0,
		Sum:             0,
	}
	assert.Equal(t, exp, res)
}

func TestHistogramAsProto(t *testing.T) {
	t.Parallel()

	uint32ptr := func(v uint32) *uint32 {
		return &v
	}

	cases := []struct {
		name string
		vals []float64
		exp  *pbcloud.TrendHdrValue
	}{
		{
			name: "empty histogram",
			exp:  &pbcloud.TrendHdrValue{},
		},
		{
			name: "not trackable values",
			vals: []float64{-0.23, 1<<30 + 1},
			exp: &pbcloud.TrendHdrValue{
				ExtraLowValuesCounter:  uint32ptr(1),
				ExtraHighValuesCounter: uint32ptr(1),
				Counters:               nil,
				Spans:                  nil,
				Count:                  2,
				MinValue:               -0.23,
				MaxValue:               1<<30 + 1,
				Sum:                    (1 << 30) + 1 - 0.23,
			},
		},
		{
			name: "normal values",
			vals: []float64{2, 1.1, 3},
			exp: &pbcloud.TrendHdrValue{
				Count:                  3,
				ExtraLowValuesCounter:  nil,
				ExtraHighValuesCounter: nil,
				Counters:               []uint32{2, 1},
				Spans: []*pbcloud.BucketSpan{
					{
						Offset: 2,
						Length: 2,
					},
				},
				MinValue: 1.1,
				MaxValue: 3,
				Sum:      6.1,
			},
		},
		{
			name: "longer sequence",
			vals: []float64{
				2275, 52.25, 268.85, 383.47, 18.49,
				163.85, 4105, 835.27, 52, 18.28, 238.44, 39751, 18.86,
				967.05, 967.01, 967, 4123.5, 270.69, 677.27,
			},
			// Sorted:
			//     18.28,18.49,18.86,52,52.25,163.85,
			//     238.44,268.85,270.69,383.47,677.27,835.27,967,967.01,967.05
			//     2275, 4105, 4123.5, 39751
			// Distribution
			// - {x<256}: 19:3, 52:1, 53:1, 164:1, 239:1
			// - {x >= 256}: 262:1, 263:1, 320:1, 425:1, 465:1, 497:1 498:2
			// - {x > 1k}: 654:1, 768:2, 1179:1
			exp: &pbcloud.TrendHdrValue{
				Count:    19,
				Counters: []uint32{3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 1, 2, 1},
				Spans: []*pbcloud.BucketSpan{
					{Offset: 19, Length: 1},
					{Offset: 33, Length: 2},
					{Offset: 111, Length: 1},
					{Offset: 75, Length: 1},
					{Offset: 23, Length: 2}, // 262
					{Offset: 57, Length: 1},
					{Offset: 105, Length: 1},
					{Offset: 40, Length: 1},
					{Offset: 32, Length: 2},
					{Offset: 156, Length: 1}, // 654
					{Offset: 114, Length: 1},
					{Offset: 411, Length: 1},
				},
				ExtraLowValuesCounter:  nil,
				ExtraHighValuesCounter: nil,
				MinValue:               18.28,
				MaxValue:               39751,
				Sum:                    56153.280000000006,
			},
		},
	}

	for _, tc := range cases {
		h := histogram{}
		for _, v := range tc.vals {
			h.Add(v)
		}
		tc.exp.Time = &timestamppb.Timestamp{Seconds: 1}
		assert.Equal(t, tc.exp, histogramAsProto(&h, time.Unix(1, 0).UnixNano()), tc.name)
	}
}
