package expv2

import (
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/output/cloud/expv2/pbcloud"
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
		{in: 100000, exp: 1347},
		{in: 182272, exp: 1458},
		{in: 183000, exp: 1458},
		{in: 184000, exp: 1459},
		{in: 200000, exp: 1475},

		{in: 1 << 20, exp: 1792},
		{in: (1 << 30) - 1, exp: 3071},
		{in: 1 << 30, exp: 3072},
		{in: 1 << 40, exp: 4352},
		{in: 1 << 62, exp: 7168},

		{in: math.MaxInt32, exp: 3199},        // 2B
		{in: math.MaxUint32, exp: 3327},       // 4B
		{in: math.MaxInt64, exp: 7296},        // Huge number // 9.22...e+18
		{in: math.MaxInt64 + 2000, exp: 7296}, // Assert that it does not overflow
	}
	for _, tc := range tests {
		assert.Equal(t, int(tc.exp), int(resolveBucketIndex(tc.in)), tc.in)
	}
}

func TestHistogramAddWithSimpleValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		vals []float64
		exp  histogram
	}{
		{
			vals: []float64{0},
			exp: histogram{
				Buckets:         map[uint32]uint32{0: 1},
				ExtraLowBucket:  0,
				ExtraHighBucket: 0,
				Max:             0,
				Min:             0,
				Sum:             0,
				Count:           1,
			},
		},
		{
			vals: []float64{8, 5},
			exp: histogram{
				Buckets:         map[uint32]uint32{5: 1, 8: 1},
				ExtraLowBucket:  0,
				ExtraHighBucket: 0,
				Max:             8,
				Min:             5,
				Sum:             13,
				Count:           2,
			},
		},
		{
			vals: []float64{8, 9, 10, 5},
			exp: histogram{
				Buckets:         map[uint32]uint32{8: 1, 9: 1, 10: 1, 5: 1},
				ExtraLowBucket:  0,
				ExtraHighBucket: 0,
				Max:             10,
				Min:             5,
				Sum:             32,
				Count:           4,
			},
		},
		{
			vals: []float64{100, 101},
			exp: histogram{
				Buckets:         map[uint32]uint32{100: 1, 101: 1},
				ExtraLowBucket:  0,
				ExtraHighBucket: 0,
				Max:             101,
				Min:             100,
				Sum:             201,
				Count:           2,
			},
		},
		{
			vals: []float64{101, 100},
			exp: histogram{
				Buckets:         map[uint32]uint32{100: 1, 101: 1},
				ExtraLowBucket:  0,
				ExtraHighBucket: 0,
				Max:             101,
				Min:             100,
				Sum:             201,
				Count:           2,
			},
		},
	}

	for i, tc := range cases {
		tc := tc
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			h := newHistogram()
			// We use a lower resolution instead of the default
			// so we can keep smaller numbers in this test
			h.MinimumResolution = 1.0
			for _, v := range tc.vals {
				h.Add(v)
			}
			tc.exp.MinimumResolution = 1.0
			assert.Equal(t, &tc.exp, h)
		})
	}
}

func TestHistogramAddWithUntrackables(t *testing.T) {
	t.Parallel()

	h := newHistogram()
	h.MinimumResolution = 1.0
	for _, v := range []float64{5, -3.14, math.MaxInt64 + 3239, 1} {
		h.Add(v)
	}

	exp := &histogram{
		Buckets:           map[uint32]uint32{1: 1, 5: 1},
		ExtraLowBucket:    1,
		ExtraHighBucket:   1,
		Max:               9223372036854779046,
		Min:               -3.14,
		Sum:               math.MaxInt64 + 3239 + 5 + 1 - 3.14,
		Count:             4,
		MinimumResolution: 1.0,
	}
	assert.Equal(t, exp, h)
}

func TestHistogramAddWithMultipleOccurances(t *testing.T) {
	t.Parallel()

	h := newHistogram()
	h.MinimumResolution = 1.0
	for _, v := range []float64{51.8, 103.6, 103.6, 103.6, 103.6} {
		h.Add(v)
	}

	exp := &histogram{
		Buckets:         map[uint32]uint32{52: 1, 104: 4},
		Max:             103.6,
		Min:             51.8,
		ExtraLowBucket:  0,
		ExtraHighBucket: 0,
		Sum:             466.20000000000005,
		Count:           5,
	}
	exp.MinimumResolution = 1.0
	assert.Equal(t, exp, h)
}

func TestHistogramAddWithNegativeNum(t *testing.T) {
	t.Parallel()

	h := newHistogram()
	h.Add(-2.42314)

	exp := &histogram{
		Max:               -2.42314,
		Min:               -2.42314,
		Buckets:           map[uint32]uint32{},
		ExtraLowBucket:    1,
		ExtraHighBucket:   0,
		Sum:               -2.42314,
		Count:             1,
		MinimumResolution: .001,
	}
	assert.Equal(t, exp, h)
}

func TestHistogramAddWithMultipleNegativeNums(t *testing.T) {
	t.Parallel()
	h := newHistogram()
	for _, v := range []float64{-0.001, -0.001, -0.001} {
		h.Add(v)
	}

	exp := &histogram{
		Buckets:           map[uint32]uint32{},
		ExtraLowBucket:    3,
		ExtraHighBucket:   0,
		Max:               -0.001,
		Min:               -0.001,
		Sum:               -0.003,
		Count:             3,
		MinimumResolution: .001,
	}
	assert.Equal(t, exp, h)
}

func TestHistogramAddWithZeroToOneValues(t *testing.T) {
	t.Parallel()
	h := newHistogram()
	for _, v := range []float64{0.000052, 0.002115, 0.012013, 0.05017, 0.250, 0.54, 0.541, 0.803} {
		h.Add(v)
	}

	exp := &histogram{
		Buckets:           map[uint32]uint32{1: 1, 3: 1, 13: 1, 51: 1, 250: 1, 391: 2, 456: 1},
		ExtraLowBucket:    0,
		ExtraHighBucket:   0,
		Max:               .803,
		Min:               .000052,
		Sum:               2.19835,
		Count:             8,
		MinimumResolution: .001,
	}
	assert.Equal(t, exp, h)
}

func TestNewHistoram(t *testing.T) {
	t.Parallel()

	h := newHistogram()
	exp := &histogram{
		Buckets:           map[uint32]uint32{},
		ExtraLowBucket:    0,
		ExtraHighBucket:   0,
		Max:               -math.MaxFloat64,
		Min:               math.MaxFloat64,
		Sum:               0,
		MinimumResolution: .001,
	}
	assert.Equal(t, exp, h)
}

func TestHistogramAsProto(t *testing.T) {
	t.Parallel()

	uint32ptr := func(v uint32) *uint32 {
		return &v
	}

	cases := []struct {
		name          string
		vals          []float64
		minResolution float64
		exp           *pbcloud.TrendHdrValue
	}{
		{
			name: "EmptyHistogram",
			exp: &pbcloud.TrendHdrValue{
				MaxValue: -math.MaxFloat64,
				MinValue: math.MaxFloat64,
			},
		},
		{
			name:          "UntrackableValues",
			vals:          []float64{-0.23, 1<<64 - 1},
			minResolution: 1.0,
			exp: &pbcloud.TrendHdrValue{
				ExtraLowValuesCounter:  uint32ptr(1),
				ExtraHighValuesCounter: uint32ptr(1),
				Counters:               nil,
				Spans:                  nil,
				Count:                  2,
				MinValue:               -0.23,
				MaxValue:               1<<64 - 1,
				Sum:                    (1 << 64) - 1 - 0.23,
				MinResolution:          1.0,
			},
		},
		{
			name:          "SimpleValuesWithLowerResolution",
			vals:          []float64{7, 8, 9, 11, 12, 11.5, 10.5},
			minResolution: 1.0,
			exp: &pbcloud.TrendHdrValue{
				Count:                  7,
				ExtraLowValuesCounter:  nil,
				ExtraHighValuesCounter: nil,
				Counters:               []uint32{1, 1, 1, 2, 2},
				Spans: []*pbcloud.BucketSpan{
					{Offset: 7, Length: 3},
					{Offset: 1, Length: 2}, // 11
				},
				MinValue:      7,
				MaxValue:      12,
				Sum:           69,
				MinResolution: 1.0,
			},
		},
		{
			name:          "SimpleValues",
			vals:          []float64{7, 8, 9, 11, 12, 11.5, 10.5},
			minResolution: .001,
			exp: &pbcloud.TrendHdrValue{
				Count:                  7,
				ExtraLowValuesCounter:  nil,
				ExtraHighValuesCounter: nil,
				Counters:               []uint32{1, 1, 1, 1, 1, 1, 1},
				Spans: []*pbcloud.BucketSpan{
					{Offset: 858, Length: 1},
					{Offset: 31, Length: 1}, // 890
					{Offset: 17, Length: 1}, // 908
					{Offset: 23, Length: 1}, // 932
					{Offset: 6, Length: 1},  // 939
					{Offset: 7, Length: 1},  // 947
					{Offset: 7, Length: 1},  // 955
				},
				MinValue:      7,
				MaxValue:      12,
				Sum:           69,
				MinResolution: .001,
			},
		},
		{
			name:          "WithZeroPointValues",
			vals:          []float64{2, 0.01, 3},
			minResolution: .001,
			exp: &pbcloud.TrendHdrValue{
				Count:                  3,
				ExtraLowValuesCounter:  nil,
				ExtraHighValuesCounter: nil,
				Counters:               []uint32{1, 1, 1},
				Spans: []*pbcloud.BucketSpan{
					{
						Offset: 10,
						Length: 1,
					},
					{
						Offset: 623,
						Length: 1,
					},
					{
						Offset: 64,
						Length: 1,
					},
				},
				MinValue:      0.01,
				MaxValue:      3,
				Sum:           5.01,
				MinResolution: .001,
			},
		},
		{
			name:          "VeryBasic",
			vals:          []float64{2, 1.1, 3},
			minResolution: 1.0,
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
				MinValue:      1.1,
				MaxValue:      3,
				Sum:           6.1,
				MinResolution: 1.0,
			},
		},
		{
			name: "LongerSequence",
			vals: []float64{
				2275, 52.25, 268.85, 383.47, 18.49,
				163.85, 4105, 835.27, 52, 18.28, 238.44, 39751, 18.86,
				967.05, 967.01, 967, 4123.5, 270.69, 677.27,
			},
			// It uses 1.0 as resolution for keeping numbers smaller
			// and the test more controllable.
			minResolution: 1.0,
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
					{Offset: 32, Length: 2},
					{Offset: 110, Length: 1},
					{Offset: 74, Length: 1},
					{Offset: 22, Length: 2}, // 262
					{Offset: 56, Length: 1},
					{Offset: 104, Length: 1},
					{Offset: 39, Length: 1},
					{Offset: 31, Length: 2},
					{Offset: 155, Length: 1}, // 654
					{Offset: 113, Length: 1},
					{Offset: 410, Length: 1},
				},
				ExtraLowValuesCounter:  nil,
				ExtraHighValuesCounter: nil,
				MinValue:               18.28,
				MaxValue:               39751,
				Sum:                    56153.280000000006,
				MinResolution:          1.0,
			},
		},
		{
			name:          "Unrealistic",
			vals:          []float64{math.MaxUint32},
			minResolution: .001,
			exp: &pbcloud.TrendHdrValue{
				Count:                  1,
				ExtraLowValuesCounter:  nil,
				ExtraHighValuesCounter: nil,
				Counters:               []uint32{1},
				Spans: []*pbcloud.BucketSpan{
					{
						Offset: 4601,
						Length: 1,
					},
				},
				MinValue:      math.MaxUint32,
				MaxValue:      math.MaxUint32,
				Sum:           math.MaxUint32,
				MinResolution: .001,
			},
		},
		{
			name:          "DefaultMinimumResolution",
			vals:          []float64{200, 100, 200.1},
			minResolution: .001,
			exp: &pbcloud.TrendHdrValue{
				Count:                  3,
				ExtraLowValuesCounter:  nil,
				ExtraHighValuesCounter: nil,
				MinResolution:          defaultMinimumResolution,
				Counters:               []uint32{1, 2},
				Spans: []*pbcloud.BucketSpan{
					{
						Offset: 1347,
						Length: 1,
					},
					{
						Offset: 127,
						Length: 1,
					},
				},
				MinValue: 100,
				MaxValue: 200.1,
				Sum:      500.1,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := newHistogram()
			h.MinimumResolution = tc.minResolution

			for _, v := range tc.vals {
				h.Add(v)
			}
			tc.exp.Time = &timestamppb.Timestamp{Seconds: 1}
			hproto := histogramAsProto(h, time.Unix(1, 0).UnixNano())
			require.Equal(t, tc.exp.Count, hproto.Count)
			require.Equal(t, tc.exp.Counters, hproto.Counters)
			require.Equal(t, len(tc.exp.Spans), len(hproto.Spans))
			assert.Equal(t, tc.exp, hproto, tc.name)
		})
	}
}
