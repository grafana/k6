package pb

import (
	"fmt"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib/types"
)

func TestGetFixedLengthInt(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		val, maxVal int64
		expRes      string
	}{
		{1, 0, "1"},
		{1, 1, "1"},
		{1, 5, "1"},
		{111, 5, "111"},
		{-1, 5, "-1"},
		{-1, -50, "-01"},
		{-1, 50, "-1"},

		{1, 15, "01"},
		{1, 15, "01"},
		{1, 150, "001"},
		{1, 1500, "0001"},
		{999, 1500, "0999"},
		{-999, 1500, "-999"},
		{-9999, 1500, "-9999"},
		{1, 10000, "00001"},
		{1234567, 10000, "1234567"},
		{123456790, math.MaxInt64, "0000000000123456790"},
		{-123456790, math.MaxInt64, "-000000000123456790"},
		{math.MaxInt64, math.MaxInt64, "9223372036854775807"},
		{-123456790, math.MinInt64, "-0000000000123456790"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.expRes, func(t *testing.T) {
			t.Parallel()
			fmtFormat := GetFixedLengthIntFormat(tc.maxVal)
			res := fmt.Sprintf(fmtFormat, tc.val)
			assert.Equal(t, tc.expRes, res)
			back, err := strconv.ParseInt(res, 10, 64)
			require.NoError(t, err)
			assert.Equal(t, tc.val, back)
		})
	}
}

func TestGetFixedLengthFloat(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		val, maxVal float64
		precision   uint
		expRes      string
	}{
		{0, 0, 0, "0"},
		{0, 0, 2, "0.00"},
		{0, 100, 2, "000.00"},
		{0, -100, 2, "0000.00"},
		{12, -100, 2, "0012.00"},
		{-12, -100, 2, "-012.00"},
		{12, 99, 2, "12.00"},
		{12, 100, 2, "012.00"},
		{1, 0, 0, "1"},
		{1, 0, 1, "1.0"},
		{1, 0, 2, "1.00"},
		{1.01, 0, 1, "1.0"},
		{1.01, 0, 1, "1.0"},
		{1.01, 0, 2, "1.01"},
		{1.007, 0, 2, "1.01"},
		{1.003, 0, 2, "1.00"},
		{1.003, 0, 3, "1.003"},
		{1.003, 0, 4, "1.0030"},
		{1.003, 1, 4, "1.0030"},
		{1.003, 9.999, 4, "1.0030"},
		{1.003, 10, 4, "01.0030"},
		{1.003, -10, 4, "001.0030"},
		{-1.003, -10, 4, "-01.0030"},
		{12.003, 1000, 4, "0012.0030"},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("tc%d_exp_%s", i, tc.expRes), func(t *testing.T) {
			t.Parallel()
			fmtFormat := GetFixedLengthFloatFormat(tc.maxVal, tc.precision)
			res := fmt.Sprintf(fmtFormat, tc.val)
			assert.Equal(t, tc.expRes, res)
			back, err := strconv.ParseFloat(res, 64)
			require.NoError(t, err)

			precPow := math.Pow(10, float64(tc.precision))
			expParseVal := math.Round(tc.val*precPow) / precPow
			assert.Equal(t, expParseVal, back)
		})
	}
}

func TestGetFixedLengthDuration(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		val, maxVal time.Duration
		expRes      string
	}{
		{0, 0, "0.0s"},
		{1 * time.Second, 0, "1.0s"},
		{9*time.Second + 940*time.Millisecond, 0, "9.9s"},
		{9*time.Second + 950*time.Millisecond, 0, "10.0s"},
		{1100 * time.Millisecond, 0, "1.1s"},
		{-1100 * time.Millisecond, 0, "1.1s"},
		{1100 * time.Millisecond, 10 * time.Second, "01.1s"},
		{1100 * time.Millisecond, 1 * time.Minute, "0m01.1s"},
		{1100 * time.Millisecond, -1 * time.Minute, "0m01.1s"},
		{-1100 * time.Millisecond, -1 * time.Minute, "0m01.1s"},
		{1100 * time.Millisecond, 10 * time.Minute, "00m01.1s"},
		{1100 * time.Millisecond, time.Hour, "0h00m01.1s"},
		{1100 * time.Millisecond, 10 * time.Hour, "00h00m01.1s"},
		{183 * time.Second, 10 * time.Minute, "03m03.0s"},
		{183 * time.Second, 120 * time.Minute, "0h03m03.0s"},
		{183 * time.Second, 10 * time.Hour, "00h03m03.0s"},
		{183 * time.Second, 25 * time.Hour, "0d00h03m03.0s"},
		{25 * time.Hour, 25 * time.Hour, "1d01h00m00.0s"},
		{482 * time.Hour, 25 * time.Hour, "20d02h00m00.0s"},
		{482 * time.Hour, 4800 * time.Hour, "020d02h00m00.0s"},
		{482*time.Hour + 671*time.Second + 65*time.Millisecond, time.Duration(math.MaxInt64), "000020d02h11m11.1s"},

		// subtracting a second since rounding doesn't work as expected at the limits of int64
		{time.Duration(math.MaxInt64) - time.Second, time.Duration(math.MaxInt64), "106751d23h47m15.9s"},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("tc%d_exp_%s", i, tc.expRes), func(t *testing.T) {
			t.Parallel()
			res := GetFixedLengthDuration(tc.val, tc.maxVal)
			assert.Equal(t, tc.expRes, res)

			expBackDur := tc.val.Round(100 * time.Millisecond)
			if expBackDur < 0 {
				expBackDur = -expBackDur
			}
			backDur, err := types.ParseExtendedDuration(res)
			assert.NoError(t, err)
			assert.Equal(t, expBackDur, backDur)
		})
	}
}
