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

package metrics

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/types"
	"gopkg.in/guregu/null.v3"
)

func TestNewThreshold(t *testing.T) {
	t.Parallel()

	src := `rate<0.01`
	abortOnFail := false
	gracePeriod := types.NullDurationFrom(2 * time.Second)

	gotThreshold := newThreshold(src, abortOnFail, gracePeriod)

	assert.Equal(t, src, gotThreshold.Source)
	assert.False(t, gotThreshold.LastFailed)
	assert.Equal(t, abortOnFail, gotThreshold.AbortOnFail)
	assert.Equal(t, gracePeriod, gotThreshold.AbortGracePeriod)
	assert.Nil(t, gotThreshold.parsed)
}

func TestThreshold_runNoTaint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		parsed           *thresholdExpression
		abortGracePeriod types.NullDuration
		sinks            map[string]float64
		wantOk           bool
		wantErr          bool
	}{
		{
			name:             "valid expression using the > operator over passing threshold",
			parsed:           &thresholdExpression{tokenRate, null.Float{}, tokenGreater, 0.01},
			abortGracePeriod: types.NullDurationFrom(0 * time.Second),
			sinks:            map[string]float64{"rate": 1},
			wantOk:           true,
			wantErr:          false,
		},
		{
			name:             "valid expression using the > operator over passing threshold and defined abort grace period",
			parsed:           &thresholdExpression{tokenRate, null.Float{}, tokenGreater, 0.01},
			abortGracePeriod: types.NullDurationFrom(2 * time.Second),
			sinks:            map[string]float64{"rate": 1},
			wantOk:           true,
			wantErr:          false,
		},
		{
			name:             "valid expression using the >= operator over passing threshold",
			parsed:           &thresholdExpression{tokenRate, null.Float{}, tokenGreaterEqual, 0.01},
			abortGracePeriod: types.NullDurationFrom(0 * time.Second),
			sinks:            map[string]float64{"rate": 0.01},
			wantOk:           true,
			wantErr:          false,
		},
		{
			name:             "valid expression using the <= operator over passing threshold",
			parsed:           &thresholdExpression{tokenRate, null.Float{}, tokenLessEqual, 0.01},
			abortGracePeriod: types.NullDurationFrom(0 * time.Second),
			sinks:            map[string]float64{"rate": 0.01},
			wantOk:           true,
			wantErr:          false,
		},
		{
			name:             "valid expression using the < operator over passing threshold",
			parsed:           &thresholdExpression{tokenRate, null.Float{}, tokenLess, 0.01},
			abortGracePeriod: types.NullDurationFrom(0 * time.Second),
			sinks:            map[string]float64{"rate": 0.00001},
			wantOk:           true,
			wantErr:          false,
		},
		{
			name:             "valid expression using the == operator over passing threshold",
			parsed:           &thresholdExpression{tokenRate, null.Float{}, tokenLooselyEqual, 0.01},
			abortGracePeriod: types.NullDurationFrom(0 * time.Second),
			sinks:            map[string]float64{"rate": 0.01},
			wantOk:           true,
			wantErr:          false,
		},
		{
			name:             "valid expression using the === operator over passing threshold",
			parsed:           &thresholdExpression{tokenRate, null.Float{}, tokenStrictlyEqual, 0.01},
			abortGracePeriod: types.NullDurationFrom(0 * time.Second),
			sinks:            map[string]float64{"rate": 0.01},
			wantOk:           true,
			wantErr:          false,
		},
		{
			name:             "valid expression using != operator over passing threshold",
			parsed:           &thresholdExpression{tokenRate, null.Float{}, tokenBangEqual, 0.01},
			abortGracePeriod: types.NullDurationFrom(0 * time.Second),
			sinks:            map[string]float64{"rate": 0.02},
			wantOk:           true,
			wantErr:          false,
		},
		{
			name:             "valid expression over failing threshold",
			parsed:           &thresholdExpression{tokenRate, null.Float{}, tokenGreater, 0.01},
			abortGracePeriod: types.NullDurationFrom(0 * time.Second),
			sinks:            map[string]float64{"rate": 0.00001},
			wantOk:           false,
			wantErr:          false,
		},
		{
			name:             "valid expression over non-existing sink",
			parsed:           &thresholdExpression{tokenRate, null.Float{}, tokenGreater, 0.01},
			abortGracePeriod: types.NullDurationFrom(0 * time.Second),
			sinks:            map[string]float64{"med": 27.2},
			wantOk:           false,
			wantErr:          true,
		},
		{
			// The ParseThresholdCondition constructor should ensure that no invalid
			// operator gets through, but let's protect our future selves anyhow.
			name:             "invalid expression operator",
			parsed:           &thresholdExpression{tokenRate, null.Float{}, "&", 0.01},
			abortGracePeriod: types.NullDurationFrom(0 * time.Second),
			sinks:            map[string]float64{"rate": 0.00001},
			wantOk:           false,
			wantErr:          true,
		},
	}
	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			threshold := &Threshold{
				LastFailed:       false,
				AbortOnFail:      false,
				AbortGracePeriod: testCase.abortGracePeriod,
				parsed:           testCase.parsed,
			}

			gotOk, gotErr := threshold.runNoTaint(testCase.sinks)

			assert.Equal(t,
				testCase.wantErr,
				gotErr != nil,
				"Threshold.runNoTaint() error = %v, wantErr %v", gotErr, testCase.wantErr,
			)

			assert.Equal(t,
				testCase.wantOk,
				gotOk,
				"Threshold.runNoTaint() gotOk = %v, want %v", gotOk, testCase.wantOk,
			)
		})
	}
}

func BenchmarkRunNoTaint(b *testing.B) {
	threshold := &Threshold{
		Source:           "rate>0.01",
		LastFailed:       false,
		AbortOnFail:      false,
		AbortGracePeriod: types.NullDurationFrom(2 * time.Second),
		parsed:           &thresholdExpression{tokenRate, null.Float{}, tokenGreater, 0.01},
	}

	sinks := map[string]float64{"rate": 1}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		threshold.runNoTaint(sinks) // nolint
	}
}

func TestThresholdRun(t *testing.T) {
	t.Parallel()

	t.Run("true", func(t *testing.T) {
		t.Parallel()

		sinks := map[string]float64{"rate": 0.0001}
		parsed, parseErr := parseThresholdExpression("rate<0.01")
		require.NoError(t, parseErr)
		threshold := newThreshold(`rate<0.01`, false, types.NullDuration{})
		threshold.parsed = parsed

		t.Run("no taint", func(t *testing.T) {
			b, err := threshold.runNoTaint(sinks)
			assert.NoError(t, err)
			assert.True(t, b)
			assert.False(t, threshold.LastFailed)
		})

		t.Run("taint", func(t *testing.T) {
			t.Parallel()

			b, err := threshold.run(sinks)
			assert.NoError(t, err)
			assert.True(t, b)
			assert.False(t, threshold.LastFailed)
		})
	})

	t.Run("false", func(t *testing.T) {
		t.Parallel()

		sinks := map[string]float64{"rate": 1}
		parsed, parseErr := parseThresholdExpression("rate<0.01")
		require.NoError(t, parseErr)
		threshold := newThreshold(`rate<0.01`, false, types.NullDuration{})
		threshold.parsed = parsed

		t.Run("no taint", func(t *testing.T) {
			b, err := threshold.runNoTaint(sinks)
			assert.NoError(t, err)
			assert.False(t, b)
			assert.False(t, threshold.LastFailed)
		})

		t.Run("taint", func(t *testing.T) {
			b, err := threshold.run(sinks)
			assert.NoError(t, err)
			assert.False(t, b)
			assert.True(t, threshold.LastFailed)
		})
	})
}

func TestThresholdsParse(t *testing.T) {
	t.Parallel()

	t.Run("valid threshold expressions", func(t *testing.T) {
		t.Parallel()

		// Prepare a Thresholds collection containing syntaxically
		// correct thresholds
		ts := Thresholds{
			Thresholds: []*Threshold{
				newThreshold("rate<1", false, types.NullDuration{}),
			},
		}

		// Collect the result of the parsing operation
		gotErr := ts.Parse()

		assert.NoError(t, gotErr, "Parse shouldn't fail parsing valid expressions")
		assert.Condition(t, func() bool {
			for _, threshold := range ts.Thresholds {
				if threshold.parsed == nil {
					return false
				}
			}

			return true
		}, "Parse did not fail, but some Thresholds' parsed field is left empty")
	})

	t.Run("invalid threshold expressions", func(t *testing.T) {
		t.Parallel()

		// Prepare a Thresholds collection containing syntaxically
		// correct thresholds
		ts := Thresholds{
			Thresholds: []*Threshold{
				newThreshold("foo&1", false, types.NullDuration{}),
			},
		}

		// Collect the result of the parsing operation
		gotErr := ts.Parse()

		assert.Error(t, gotErr, "Parse should fail parsing invalid expressions")
		assert.Condition(t, func() bool {
			for _, threshold := range ts.Thresholds {
				if threshold.parsed == nil {
					return true
				}
			}

			return false
		}, "Parse failed, but some Thresholds' parsed field was not empty")
	})

	t.Run("mixed valid/invalid threshold expressions", func(t *testing.T) {
		t.Parallel()

		// Prepare a Thresholds collection containing syntaxically
		// correct thresholds
		ts := Thresholds{
			Thresholds: []*Threshold{
				newThreshold("rate<1", false, types.NullDuration{}),
				newThreshold("foo&1", false, types.NullDuration{}),
			},
		}

		// Collect the result of the parsing operation
		gotErr := ts.Parse()

		assert.Error(t, gotErr, "Parse should fail parsing invalid expressions")
		assert.Condition(t, func() bool {
			for _, threshold := range ts.Thresholds {
				if threshold.parsed == nil {
					return true
				}
			}

			return false
		}, "Parse failed, but some Thresholds' parsed field was not empty")
	})
}

func TestNewThresholds(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		ts := NewThresholds([]string{})
		assert.Len(t, ts.Thresholds, 0)
	})
	t.Run("two", func(t *testing.T) {
		t.Parallel()

		sources := []string{`rate<0.01`, `p(95)<200`}
		ts := NewThresholds(sources)
		assert.Len(t, ts.Thresholds, 2)
		for i, th := range ts.Thresholds {
			assert.Equal(t, sources[i], th.Source)
			assert.False(t, th.LastFailed)
			assert.False(t, th.AbortOnFail)
		}
	})
}

func TestNewThresholdsWithConfig(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		ts := NewThresholds([]string{})
		assert.Len(t, ts.Thresholds, 0)
	})
	t.Run("two", func(t *testing.T) {
		t.Parallel()

		configs := []thresholdConfig{
			{`rate<0.01`, false, types.NullDuration{}},
			{`p(95)<200`, true, types.NullDuration{}},
		}
		ts := newThresholdsWithConfig(configs)
		assert.Len(t, ts.Thresholds, 2)
		for i, th := range ts.Thresholds {
			assert.Equal(t, configs[i].Threshold, th.Source)
			assert.False(t, th.LastFailed)
			assert.Equal(t, configs[i].AbortOnFail, th.AbortOnFail)
		}
	})
}

func TestThresholdsRunAll(t *testing.T) {
	t.Parallel()

	zero := types.NullDuration{}
	oneSec := types.NullDurationFrom(time.Second)
	twoSec := types.NullDurationFrom(2 * time.Second)
	testdata := map[string]struct {
		succeeded bool
		err       bool
		abort     bool
		grace     types.NullDuration
		sources   []string
	}{
		"one passing":                {true, false, false, zero, []string{`rate<0.01`}},
		"one failing":                {false, false, false, zero, []string{`p(95)<200`}},
		"two passing":                {true, false, false, zero, []string{`rate<0.1`, `rate<0.01`}},
		"two failing":                {false, false, false, zero, []string{`p(95)<200`, `rate<0.1`}},
		"two mixed":                  {false, false, false, zero, []string{`rate<0.01`, `p(95)<200`}},
		"one aborting":               {false, false, true, zero, []string{`p(95)<200`}},
		"abort with grace period":    {false, false, true, oneSec, []string{`p(95)<200`}},
		"no abort with grace period": {false, false, true, twoSec, []string{`p(95)<200`}},
	}

	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			thresholds := NewThresholds(data.sources)
			gotParseErr := thresholds.Parse()
			require.NoError(t, gotParseErr)
			thresholds.sinked = map[string]float64{"rate": 0.0001, "p(95)": 500}
			thresholds.Thresholds[0].AbortOnFail = data.abort
			thresholds.Thresholds[0].AbortGracePeriod = data.grace

			runDuration := 1500 * time.Millisecond

			succeeded, err := thresholds.runAll(runDuration)

			if data.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if data.succeeded {
				assert.True(t, succeeded)
			} else {
				assert.False(t, succeeded)
			}

			if data.abort && data.grace.Duration < types.Duration(runDuration) {
				assert.True(t, thresholds.Abort)
			} else {
				assert.False(t, thresholds.Abort)
			}
		})
	}
}

func TestThresholds_Run(t *testing.T) {
	t.Parallel()

	type args struct {
		sink     Sink
		duration time.Duration
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			"Running thresholds of existing sink",
			args{DummySink{"p(95)": 1234.5}, 0},
			true,
			false,
		},
		{
			"Running thresholds of existing sink but failing threshold",
			args{DummySink{"p(95)": 3000}, 0},
			false,
			false,
		},
		{
			"Running threshold on non existing sink fails",
			args{DummySink{"dummy": 0}, 0},
			false,
			true,
		},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			thresholds := NewThresholds([]string{"p(95)<2000"})
			gotParseErr := thresholds.Parse()
			require.NoError(t, gotParseErr)

			gotOk, gotErr := thresholds.Run(testCase.args.sink, testCase.args.duration)
			assert.Equal(t, gotErr != nil, testCase.wantErr, "Thresholds.Run() error = %v, wantErr %v", gotErr, testCase.wantErr)
			assert.Equal(t, gotOk, testCase.want, "Thresholds.Run() = %v, want %v", gotOk, testCase.want)
		})
	}
}

func TestThresholdsJSON(t *testing.T) {
	t.Parallel()

	testdata := []struct {
		JSON        string
		sources     []string
		abortOnFail bool
		gracePeriod types.NullDuration
		outputJSON  string
	}{
		{
			`[]`,
			[]string{},
			false,
			types.NullDuration{},
			"",
		},
		{
			`["rate<0.01"]`,
			[]string{"rate<0.01"},
			false,
			types.NullDuration{},
			"",
		},
		{
			`["rate<0.01"]`,
			[]string{"rate<0.01"},
			false,
			types.NullDuration{},
			`["rate<0.01"]`,
		},
		{
			`["rate<0.01","p(95)<200"]`,
			[]string{"rate<0.01", "p(95)<200"},
			false,
			types.NullDuration{},
			"",
		},
		{
			`[{"threshold":"rate<0.01"}]`,
			[]string{"rate<0.01"},
			false,
			types.NullDuration{},
			`["rate<0.01"]`,
		},
		{
			`[{"threshold":"rate<0.01","abortOnFail":true,"delayAbortEval":null}]`,
			[]string{"rate<0.01"},
			true,
			types.NullDuration{},
			"",
		},
		{
			`[{"threshold":"rate<0.01","abortOnFail":true,"delayAbortEval":"2s"}]`,
			[]string{"rate<0.01"},
			true,
			types.NullDurationFrom(2 * time.Second),
			"",
		},
		{
			`[{"threshold":"rate<0.01","abortOnFail":false}]`,
			[]string{"rate<0.01"},
			false,
			types.NullDuration{},
			`["rate<0.01"]`,
		},
		{
			`[{"threshold":"rate<0.01"}, "p(95)<200"]`,
			[]string{"rate<0.01", "p(95)<200"},
			false,
			types.NullDuration{},
			`["rate<0.01","p(95)<200"]`,
		},
	}

	for _, data := range testdata {
		data := data

		t.Run(data.JSON, func(t *testing.T) {
			t.Parallel()

			var ts Thresholds
			assert.NoError(t, json.Unmarshal([]byte(data.JSON), &ts))
			assert.Equal(t, len(data.sources), len(ts.Thresholds))
			for i, src := range data.sources {
				assert.Equal(t, src, ts.Thresholds[i].Source)
				assert.Equal(t, data.abortOnFail, ts.Thresholds[i].AbortOnFail)
				assert.Equal(t, data.gracePeriod, ts.Thresholds[i].AbortGracePeriod)
			}

			t.Run("marshal", func(t *testing.T) {
				data2, err := MarshalJSONWithoutHTMLEscape(ts)
				assert.NoError(t, err)
				output := data.JSON
				if data.outputJSON != "" {
					output = data.outputJSON
				}
				assert.Equal(t, output, string(data2))
			})
		})
	}

	t.Run("bad JSON", func(t *testing.T) {
		t.Parallel()

		var ts Thresholds
		assert.Error(t, json.Unmarshal([]byte("42"), &ts))
		assert.Nil(t, ts.Thresholds)
		assert.False(t, ts.Abort)
	})

	t.Run("bad source", func(t *testing.T) {
		t.Parallel()

		var ts Thresholds
		assert.Nil(t, ts.Thresholds)
		assert.False(t, ts.Abort)
	})
}
