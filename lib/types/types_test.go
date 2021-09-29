/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package types

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"
)

func TestNullDecoder(t *testing.T) {
	type foo struct {
		Strs      []string
		Str       null.String
		Boolean   null.Bool
		Integer   null.Int
		Integer32 null.Int
		Integer64 null.Int
		Float32   null.Float
		Float64   null.Float
		Dur       NullDuration
	}
	f := foo{}
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: NullDecoder,
		Result:     &f,
	})
	require.NoError(t, err)

	conf := map[string]interface{}{
		"strs":      []string{"fake"},
		"str":       "bar",
		"boolean":   true,
		"integer":   42,
		"integer32": int32(42),
		"integer64": int64(42),
		"float32":   float32(3.14),
		"float64":   float64(3.14),
		"dur":       "1m",
	}

	err = dec.Decode(conf)
	require.NoError(t, err)

	require.Equal(t, foo{
		Strs:      []string{"fake"},
		Str:       null.StringFrom("bar"),
		Boolean:   null.BoolFrom(true),
		Integer:   null.IntFrom(42),
		Integer32: null.IntFrom(42),
		Integer64: null.IntFrom(42),
		Float32:   null.FloatFrom(3.140000104904175),
		Float64:   null.FloatFrom(3.14),
		Dur:       NewNullDuration(1*time.Minute, true),
	}, f)

	input := map[string][]interface{}{
		"Str":       {true, "string", "bool"},
		"Boolean":   {"invalid", "bool", "string"},
		"Integer":   {"invalid", "int", "string"},
		"Integer32": {true, "int", "bool"},
		"Integer64": {"invalid", "int", "string"},
		"Float32":   {true, "float32 or float64", "bool"},
		"Float64":   {"invalid", "float32 or float64", "string"},
		"Dur":       {10, "string", "int"},
	}

	for k, v := range input {
		t.Run("Error Message/"+k, func(t *testing.T) {
			err = dec.Decode(map[string]interface{}{
				k: v[0],
			})
			assert.EqualError(t, err, fmt.Sprintf("1 error(s) decoding:\n\n* error decoding '%s': expected '%s', got '%s'", k, v[1], v[2]))
		})
	}
}

func TestParseExtendedDuration(t *testing.T) {
	testCases := []struct {
		durStr string
		expErr bool
		expDur time.Duration
	}{
		{"", true, 0},
		{"d", true, 0},
		{"d2h", true, 0},
		{"d2h", true, 0},
		{"2.1d", true, 0},
		{"2d-2h", true, 0},
		{"-2d-2h", true, 0},
		{"2+d", true, 0},
		{"2da", true, 0},
		{"2-d", true, 0},
		{"1.12s", false, 1120 * time.Millisecond},
		{"0d1.12s", false, 1120 * time.Millisecond},
		{"10d1.12s", false, 240*time.Hour + 1120*time.Millisecond},
		{"1s", false, 1 * time.Second},
		{"1d", false, 24 * time.Hour},
		{"20d", false, 480 * time.Hour},
		{"1d23h", false, 47 * time.Hour},
		{"1d24h15m", false, 48*time.Hour + 15*time.Minute},
		{"1d25h80m", false, 50*time.Hour + 20*time.Minute},
		{"0d25h120m80s", false, 27*time.Hour + 80*time.Second},
		{"-1d2h", false, -26 * time.Hour},
		{"-1d24h", false, -48 * time.Hour},
		{"2d1ns", false, 48*time.Hour + 1},
		{"-2562047h47m16.854775807s", false, time.Duration(math.MinInt64 + 1)},
		{"-106751d23h47m16.854775807s", false, time.Duration(math.MinInt64 + 1)},
		{"2562047h47m16.854775807s", false, time.Duration(math.MaxInt64)},
		{"106751d23h47m16.854775807s", false, time.Duration(math.MaxInt64)},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("tc_%s_exp", tc.durStr), func(t *testing.T) {
			result, err := ParseExtendedDuration(tc.durStr)
			if tc.expErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expDur, result)
			}
		})
	}
}

func TestDuration(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		assert.Equal(t, "1m15s", Duration(75*time.Second).String())
	})
	t.Run("JSON", func(t *testing.T) {
		t.Run("Unmarshal", func(t *testing.T) {
			t.Run("Number", func(t *testing.T) {
				var d Duration
				assert.NoError(t, json.Unmarshal([]byte(`75000`), &d))
				assert.Equal(t, Duration(75*time.Second), d)
			})
			t.Run("Seconds", func(t *testing.T) {
				var d Duration
				assert.NoError(t, json.Unmarshal([]byte(`"75s"`), &d))
				assert.Equal(t, Duration(75*time.Second), d)
			})
			t.Run("String", func(t *testing.T) {
				var d Duration
				assert.NoError(t, json.Unmarshal([]byte(`"1m15s"`), &d))
				assert.Equal(t, Duration(75*time.Second), d)
			})
			t.Run("Extended", func(t *testing.T) {
				var d Duration
				assert.NoError(t, json.Unmarshal([]byte(`"1d2h1m15s"`), &d))
				assert.Equal(t, Duration(26*time.Hour+75*time.Second), d)
			})
		})
		t.Run("Marshal", func(t *testing.T) {
			d := Duration(75 * time.Second)
			data, err := json.Marshal(d)
			assert.NoError(t, err)
			assert.Equal(t, `"1m15s"`, string(data))
		})
	})
	t.Run("Text", func(t *testing.T) {
		var d Duration
		assert.NoError(t, d.UnmarshalText([]byte(`10s`)))
		assert.Equal(t, Duration(10*time.Second), d)
	})
}

func TestNullDuration(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		assert.Equal(t, "1m15s", Duration(75*time.Second).String())
	})
	t.Run("JSON", func(t *testing.T) {
		t.Run("Unmarshal", func(t *testing.T) {
			t.Run("Number", func(t *testing.T) {
				var d NullDuration
				assert.NoError(t, json.Unmarshal([]byte(`75000`), &d))
				assert.Equal(t, NullDuration{Duration(75 * time.Second), true}, d)
			})
			t.Run("Seconds", func(t *testing.T) {
				var d NullDuration
				assert.NoError(t, json.Unmarshal([]byte(`"75s"`), &d))
				assert.Equal(t, NullDuration{Duration(75 * time.Second), true}, d)
			})
			t.Run("String", func(t *testing.T) {
				var d NullDuration
				assert.NoError(t, json.Unmarshal([]byte(`"1m15s"`), &d))
				assert.Equal(t, NullDuration{Duration(75 * time.Second), true}, d)
			})
			t.Run("Null", func(t *testing.T) {
				var d NullDuration
				assert.NoError(t, json.Unmarshal([]byte(`null`), &d))
				assert.Equal(t, NullDuration{Duration(0), false}, d)
			})
		})
		t.Run("Marshal", func(t *testing.T) {
			t.Run("Valid", func(t *testing.T) {
				d := NullDuration{Duration(75 * time.Second), true}
				data, err := json.Marshal(d)
				assert.NoError(t, err)
				assert.Equal(t, `"1m15s"`, string(data))
			})
			t.Run("null", func(t *testing.T) {
				var d NullDuration
				data, err := json.Marshal(d)
				assert.NoError(t, err)
				assert.Equal(t, `null`, string(data))
			})
		})
	})
	t.Run("Text", func(t *testing.T) {
		var d NullDuration
		assert.NoError(t, d.UnmarshalText([]byte(`10s`)))
		assert.Equal(t, NullDurationFrom(10*time.Second), d)

		t.Run("Empty", func(t *testing.T) {
			var d NullDuration
			assert.NoError(t, d.UnmarshalText([]byte(``)))
			assert.Equal(t, NullDuration{}, d)
		})
	})
}

func TestNullDurationFrom(t *testing.T) {
	assert.Equal(t, NullDuration{Duration(10 * time.Second), true}, NullDurationFrom(10*time.Second))
}

func TestGetDurationValue(t *testing.T) {
	testCases := []struct {
		val      interface{}
		expError bool
		exp      time.Duration
	}{
		{false, true, 0},
		{time.Now(), true, 0},
		{"invalid", true, 0},
		{uint64(math.MaxInt64) + 1, true, 0},

		{int8(100), false, 100 * time.Millisecond},
		{uint8(100), false, 100 * time.Millisecond},
		{uint(1000), false, time.Second},
		{int(1000), false, time.Second},
		{uint16(1000), false, time.Second},
		{int16(1000), false, time.Second},
		{uint32(1000), false, time.Second},
		{int32(1000), false, time.Second},
		{uint(1000), false, time.Second},
		{int(1000), false, time.Second},
		{uint64(1000), false, time.Second},
		{int64(1000), false, time.Second},
		{1000, false, time.Second},
		{1000, false, time.Second},
		{1000.0, false, time.Second},
		{float32(1000.0), false, time.Second},
		{float64(1000.001), false, time.Second + time.Microsecond},
		{"1000", false, time.Second},
		{"1000.001", false, time.Second + time.Microsecond},
		{"1s", false, time.Second},
		{"1.5s", false, 1500 * time.Millisecond},
		{time.Second, false, time.Second},
		{"1d3h1s", false, 27*time.Hour + time.Second},
		// TODO: test for int overflows when that's implemented
	}

	for i, tc := range testCases {
		i, tc := i, tc
		t.Run(fmt.Sprintf("testcase_%02d", i), func(t *testing.T) {
			res, err := GetDurationValue(tc.val)
			if tc.expError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.exp, res)
			}
		})
	}
}
