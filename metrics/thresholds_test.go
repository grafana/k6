package metrics

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
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
			wantOk:           true,
			wantErr:          false,
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
		_, _ = threshold.runNoTaint(sinks)
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

func TestThresholdsValidate(t *testing.T) {
	t.Parallel()

	t.Run("validating thresholds applied to a non existing metric fails", func(t *testing.T) {
		t.Parallel()

		testRegistry := NewRegistry()

		// Prepare a Thresholds collection containing syntaxically
		// correct thresholds
		ts := Thresholds{
			Thresholds: []*Threshold{
				newThreshold("rate<1", false, types.NullDuration{}),
			},
		}

		parseErr := ts.Parse()
		require.NoError(t, parseErr)

		var wantErr errext.HasExitCode

		// Collect the result of the parsing operation
		gotErr := ts.Validate("non-existing", testRegistry)

		assert.Error(t, gotErr)
		assert.ErrorIs(t, gotErr, ErrInvalidThreshold)
		assert.ErrorAs(t, gotErr, &wantErr)
		assert.Equal(t, exitcodes.InvalidConfig, wantErr.ExitCode())
	})

	t.Run("validating unparsed thresholds fails", func(t *testing.T) {
		t.Parallel()

		testRegistry := NewRegistry()
		_, err := testRegistry.NewMetric("test_counter", Counter)
		require.NoError(t, err)

		// Prepare a Thresholds collection containing syntaxically
		// correct thresholds
		ts := Thresholds{
			Thresholds: []*Threshold{
				newThreshold("rate<1", false, types.NullDuration{}),
			},
		}

		// Note that we're not parsing the thresholds

		// Collect the result of the parsing operation
		gotErr := ts.Validate("non-existing", testRegistry)

		assert.Error(t, gotErr)
	})

	t.Run("thresholds supported aggregation methods for metrics", func(t *testing.T) {
		t.Parallel()

		testRegistry := NewRegistry()
		testCounter, err := testRegistry.NewMetric("test_counter", Counter)
		require.NoError(t, err)
		_, err = testCounter.AddSubmetric("foo:bar")
		require.NoError(t, err)
		_, err = testCounter.AddSubmetric("abc:123,easyas:doremi")
		require.NoError(t, err)

		_, err = testRegistry.NewMetric("test_rate", Rate)
		require.NoError(t, err)

		_, err = testRegistry.NewMetric("test_gauge", Gauge)
		require.NoError(t, err)

		_, err = testRegistry.NewMetric("test_trend", Trend)
		require.NoError(t, err)

		tests := []struct {
			name       string
			metricName string
			thresholds Thresholds
			wantErr    bool
		}{
			{
				name:       "threshold expression using 'count' is valid against a counter metric",
				metricName: "test_counter",
				thresholds: Thresholds{
					Thresholds: []*Threshold{newThreshold("count==1", false, types.NullDuration{})},
				},
				wantErr: false,
			},
			{
				name:       "threshold expression using 'rate' is valid against a counter metric",
				metricName: "test_counter",
				thresholds: Thresholds{
					Thresholds: []*Threshold{newThreshold("rate==1", false, types.NullDuration{})},
				},
				wantErr: false,
			},
			{
				name:       "threshold expression using 'rate' is valid against a counter single tag submetric",
				metricName: "test_counter{foo:bar}",
				thresholds: Thresholds{
					Thresholds: []*Threshold{newThreshold("rate==1", false, types.NullDuration{})},
				},
				wantErr: false,
			},
			{
				name:       "threshold expression using 'rate' is valid against a counter multi-tag submetric",
				metricName: "test_counter{abc:123,easyas:doremi}",
				thresholds: Thresholds{
					Thresholds: []*Threshold{newThreshold("rate==1", false, types.NullDuration{})},
				},
				wantErr: false,
			},
			{
				name:       "threshold expression using 'value' is valid against a gauge metric",
				metricName: "test_gauge",
				thresholds: Thresholds{
					Thresholds: []*Threshold{newThreshold("value==1", false, types.NullDuration{})},
				},
				wantErr: false,
			},
			{
				name:       "threshold expression using 'rate' is valid against a rate metric",
				metricName: "test_rate",
				thresholds: Thresholds{
					Thresholds: []*Threshold{newThreshold("rate==1", false, types.NullDuration{})},
				},
				wantErr: false,
			},
			{
				name:       "threshold expression using 'avg' is valid against a trend metric",
				metricName: "test_trend",
				thresholds: Thresholds{
					Thresholds: []*Threshold{newThreshold("avg==1", false, types.NullDuration{})},
				},
				wantErr: false,
			},
			{
				name:       "threshold expression using 'min' is valid against a trend metric",
				metricName: "test_trend",
				thresholds: Thresholds{
					Thresholds: []*Threshold{newThreshold("min==1", false, types.NullDuration{})},
				},
				wantErr: false,
			},
			{
				name:       "threshold expression using 'max' is valid against a trend metric",
				metricName: "test_trend",
				thresholds: Thresholds{
					Thresholds: []*Threshold{newThreshold("max==1", false, types.NullDuration{})},
				},
				wantErr: false,
			},
			{
				name:       "threshold expression using 'med' is valid against a trend metric",
				metricName: "test_trend",
				thresholds: Thresholds{
					Thresholds: []*Threshold{newThreshold("med==1", false, types.NullDuration{})},
				},
				wantErr: false,
			},
			{
				name:       "threshold expression using 'p(value)' is valid against a trend metric",
				metricName: "test_trend",
				thresholds: Thresholds{
					Thresholds: []*Threshold{newThreshold("p(99)==1", false, types.NullDuration{})},
				},
				wantErr: false,
			},
		}

		for _, testCase := range tests {
			testCase := testCase

			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				// Ensure thresholds are parsed
				parseErr := testCase.thresholds.Parse()
				require.NoError(t, parseErr)

				gotErr := testCase.thresholds.Validate(testCase.metricName, testRegistry)

				assert.Equal(t,
					testCase.wantErr,
					gotErr != nil,
					"Thresholds.Validate() error = %v, wantErr %v", gotErr, testCase.wantErr,
				)

				if testCase.wantErr {
					var targetErr errext.HasExitCode

					assert.ErrorIs(t,
						gotErr,
						ErrInvalidThreshold,
						"Validate error chain should contain an ErrInvalidThreshold error",
					)

					assert.ErrorAs(t,
						gotErr,
						&targetErr,
						"Validate error chain should contain an errext.HasExitCode error",
					)

					assert.Equal(t,
						exitcodes.InvalidConfig,
						targetErr.ExitCode(),
						"Validate error should have been exitcodes.InvalidConfig",
					)
				}
			})
		}
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
		name, data := name, data
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

func getTrendSink(values ...float64) *TrendSink {
	sink := NewTrendSink()
	for _, v := range values {
		sink.Add(Sample{Value: v})
	}
	return sink
}

func TestThresholdsRun(t *testing.T) {
	t.Parallel()

	type args struct {
		sink                 Sink
		thresholdExpressions []string
		duration             time.Duration
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "Running thresholds of existing sink",
			args: args{
				sink:                 &CounterSink{Value: 1234.5},
				thresholdExpressions: []string{"count<2000"},
				duration:             0,
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Running thresholds of existing sink but failing threshold",
			args: args{
				sink:                 &CounterSink{Value: 3000},
				thresholdExpressions: []string{"count<2000"},
				duration:             0,
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "Running threshold on non existing sink does not fail",
			args: args{
				sink:                 &CounterSink{},
				thresholdExpressions: []string{"p(95)<2000"},
				duration:             0,
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Running threshold on trend sink with no values and passing med statement succeeds",
			args: args{
				sink:                 getTrendSink(),
				thresholdExpressions: []string{"med<39"},
				duration:             0,
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Running threshold on trend sink with no values and non passing med statement fails",
			args: args{
				sink:                 getTrendSink(),
				thresholdExpressions: []string{"med>39"},
				duration:             0,
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "Running threshold on trend sink with values and passing med statement succeeds",
			args: args{
				sink:                 getTrendSink(70, 80, 90),
				thresholdExpressions: []string{"med>39"},
				duration:             0,
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Running threshold on trend sink with values and failing med statement fails",
			args: args{
				sink:                 getTrendSink(70, 80, 90),
				thresholdExpressions: []string{"med<39"},
				duration:             0,
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			thresholds := NewThresholds(testCase.args.thresholdExpressions)
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
