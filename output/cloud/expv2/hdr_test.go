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

func TestValueBacket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in  float64
		exp uint32
	}{
		{in: -1029, exp: 0},
		{in: -12, exp: 0},
		{in: -0.82673, exp: 0},
		{in: 10, exp: 10},
		{in: 12, exp: 12},
		{in: 12.5, exp: 13},
		{in: 20, exp: 20},
		{in: 255, exp: 255},
		{in: 256, exp: 256},
		{in: 282.29, exp: 269},
		{in: 1029, exp: 512},
		{in: (1 << 30) - 1, exp: 3071},
		{in: (1 << 30), exp: 3072},
		{in: math.MaxInt32, exp: 3199},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.exp, resolveBucketIndex(tc.in), tc.in)
	}
}

func TestNewHistogramWithSimpleValue(t *testing.T) {
	t.Parallel()

	// Zero as value
	res := histogram{}
	res.addToBucket(0)
	exp := histogram{
		Buckets:            []uint32{1},
		FirstNotZeroBucket: 0,
		LastNotZeroBucket:  0,
		ExtraLowBucket:     0,
		ExtraHighBucket:    0,
		Max:                0,
		Min:                0,
		Sum:                0,
		Count:              1,
	}
	require.Equal(t, exp, res)

	// Add a lower bucket index within slice capacity
	res = histogram{}
	res.addToBucket(8)
	res.addToBucket(5)

	exp = histogram{
		Buckets:            []uint32{1, 0, 0, 1},
		FirstNotZeroBucket: 5,
		LastNotZeroBucket:  8,
		ExtraLowBucket:     0,
		ExtraHighBucket:    0,
		Max:                8,
		Min:                5,
		Sum:                13,
		Count:              2,
	}
	require.Equal(t, exp, res)

	// Add a higher bucket index within slice capacity
	res = histogram{}
	res.addToBucket(100)
	res.addToBucket(101)

	exp = histogram{
		Buckets:            []uint32{1, 1},
		FirstNotZeroBucket: 100,
		LastNotZeroBucket:  101,
		ExtraLowBucket:     0,
		ExtraHighBucket:    0,
		Max:                101,
		Min:                100,
		Sum:                201,
		Count:              2,
	}
	require.Equal(t, exp, res)

	// Same case but reversed test check
	res = histogram{}
	res.addToBucket(101)
	res.addToBucket(100)

	exp = histogram{
		Buckets:            []uint32{1, 1},
		FirstNotZeroBucket: 100,
		LastNotZeroBucket:  101,
		ExtraLowBucket:     0,
		ExtraHighBucket:    0,
		Max:                101,
		Min:                100,
		Sum:                201,
		Count:              2,
	}
	assert.Equal(t, exp, res)

	// One more complex case with lower index and more than two indexes
	res = histogram{}
	res.addToBucket(8)
	res.addToBucket(9)
	res.addToBucket(10)
	res.addToBucket(5)

	exp = histogram{
		Buckets:            []uint32{1, 0, 0, 1, 1, 1},
		FirstNotZeroBucket: 5,
		LastNotZeroBucket:  10,
		ExtraLowBucket:     0,
		ExtraHighBucket:    0,
		Max:                10,
		Min:                5,
		Sum:                32,
		Count:              4,
	}

	assert.Equal(t, exp, res)
}

func TestNewHistogramWithUntrackables(t *testing.T) {
	t.Parallel()

	res := histogram{}
	for _, v := range []float64{5, -3.14, 2 * 1e9, 1} {
		res.addToBucket(v)
	}

	exp := histogram{
		Buckets:            []uint32{1, 0, 0, 0, 1},
		FirstNotZeroBucket: 1,
		LastNotZeroBucket:  5,
		ExtraLowBucket:     1,
		ExtraHighBucket:    1,
		Max:                2 * 1e9,
		Min:                -3.14,
		Sum:                2*1e9 + 5 + 1 - 3.14,
		Count:              4,
	}
	assert.Equal(t, exp, res)
}

func TestNewHistogramWithMultipleValues(t *testing.T) {
	t.Parallel()

	res := histogram{}
	for _, v := range []float64{51.8, 103.6, 103.6, 103.6, 103.6} {
		res.addToBucket(v)
	}

	exp := histogram{
		FirstNotZeroBucket: 52,
		LastNotZeroBucket:  104,
		Max:                103.6,
		Min:                51.8,
		ExtraLowBucket:     0,
		ExtraHighBucket:    0,
		Buckets:            append(append([]uint32{1}, make([]uint32, 51)...), 4),
		// Buckets = {1, 0 for 51 times, 4}
		Sum:   466.20000000000005,
		Count: 5,
	}
	assert.Equal(t, exp, res)
}

func TestNewHistogramWithNegativeNum(t *testing.T) {
	t.Parallel()

	res := histogram{}
	res.addToBucket(-2.42314)

	exp := histogram{
		FirstNotZeroBucket: 0,
		Max:                -2.42314,
		Min:                -2.42314,
		Buckets:            nil,
		ExtraLowBucket:     1,
		ExtraHighBucket:    0,
		Sum:                -2.42314,
		Count:              1,
	}
	assert.Equal(t, exp, res)
}

func TestNewHistogramWithMultipleNegativeNums(t *testing.T) {
	t.Parallel()
	res := histogram{}
	for _, v := range []float64{-0.001, -0.001, -0.001} {
		res.addToBucket(v)
	}

	exp := histogram{
		Buckets:            nil,
		FirstNotZeroBucket: 0,
		ExtraLowBucket:     3,
		ExtraHighBucket:    0,
		Max:                -0.001,
		Min:                -0.001,
		Sum:                -0.003,
		Count:              3,
	}
	assert.Equal(t, exp, res)
}

func TestNewHistoramWithNoVals(t *testing.T) {
	t.Parallel()

	res := histogram{}
	exp := histogram{
		Buckets:            nil,
		FirstNotZeroBucket: 0,
		ExtraLowBucket:     0,
		ExtraHighBucket:    0,
		Max:                0,
		Min:                0,
		Sum:                0,
	}
	assert.Equal(t, exp, res)
}

func TestHistogramAppendBuckets(t *testing.T) {
	t.Parallel()
	h := histogram{}

	// the cap is smaller than requested index
	// so it creates a new slice
	h.appendBuckets(3)
	assert.Len(t, h.Buckets, 4)

	// it must preserve already existing items
	h.Buckets[2] = 101

	// it appends to the same slice
	h.appendBuckets(5)
	assert.Len(t, h.Buckets, 6)
	assert.Equal(t, uint32(101), h.Buckets[2])
	assert.Equal(t, uint32(1), h.Buckets[5])

	// it is not possible to request an index smaller than
	// the last already available index
	h.LastNotZeroBucket = 5
	assert.Panics(t, func() { h.appendBuckets(4) })
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
				Count:                  2,
				ExtraLowValuesCounter:  uint32ptr(1),
				ExtraHighValuesCounter: uint32ptr(1),
				Counters:               nil,
				LowerCounterIndex:      0,
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
				LowerCounterIndex:      2,
				MinValue:               1.1,
				MaxValue:               3,
				Sum:                    6.1,
			},
		},
	}

	for _, tc := range cases {
		h := histogram{}
		for _, v := range tc.vals {
			h.addToBucket(v)
		}
		tc.exp.MinResolution = 1.0
		tc.exp.SignificantDigits = 2
		tc.exp.Time = &timestamppb.Timestamp{Seconds: 1}
		assert.Equal(t, tc.exp, histogramAsProto(&h, time.Unix(1, 0).UnixNano()), tc.name)
	}
}
