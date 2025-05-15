package histogram

import (
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
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
		exp  *Hdr
	}{
		{
			vals: []float64{0},
			exp: &Hdr{
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
			exp: &Hdr{
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
			exp: &Hdr{
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
			exp: &Hdr{
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
			exp: &Hdr{
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
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			h := NewHdr()
			// We use a lower resolution instead of the default
			// so we can keep smaller numbers in this test
			h.MinimumResolution = 1.0
			for _, v := range tc.vals {
				h.Add(v)
			}
			tc.exp.MinimumResolution = 1.0
			assert.Equal(t, tc.exp, h)
		})
	}
}

func TestHistogramAddWithUntrackables(t *testing.T) {
	t.Parallel()

	h := NewHdr()
	h.MinimumResolution = 1.0
	for _, v := range []float64{5, -3.14, math.MaxInt64 + 3239, 1} {
		h.Add(v)
	}

	exp := &Hdr{
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

	h := NewHdr()
	h.MinimumResolution = 1.0
	for _, v := range []float64{51.8, 103.6, 103.6, 103.6, 103.6} {
		h.Add(v)
	}

	exp := &Hdr{
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

	h := NewHdr()
	h.Add(-2.42314)

	exp := &Hdr{
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
	h := NewHdr()
	for _, v := range []float64{-0.001, -0.001, -0.001} {
		h.Add(v)
	}

	exp := &Hdr{
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
	h := NewHdr()
	for _, v := range []float64{0.000052, 0.002115, 0.012013, 0.05017, 0.250, 0.54, 0.541, 0.803} {
		h.Add(v)
	}

	exp := &Hdr{
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

func TestNewHistogram(t *testing.T) {
	t.Parallel()

	h := NewHdr()
	exp := &Hdr{
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
