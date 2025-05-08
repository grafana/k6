package expv2

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.k6.io/k6/internal/ds/histogram"
	"go.k6.io/k6/internal/output/cloud/expv2/pbcloud"
)

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
				MinResolution:          .001,
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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := histogram.NewHdr()
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
