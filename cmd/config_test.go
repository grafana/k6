/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package cmd

import (
	"testing"
	"time"

	"github.com/mstoykov/envconfig"
	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/executor"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/stats"
	"gopkg.in/guregu/null.v3"
)

type testCmdData struct {
	Name  string
	Tests []testCmdTest
}

type testCmdTest struct {
	Args     []string
	Expected []string
	Name     string
}

func TestConfigCmd(t *testing.T) {
	t.Parallel()
	testdata := []testCmdData{
		{
			Name: "Out",

			Tests: []testCmdTest{
				{
					Name:     "NoArgs",
					Args:     []string{""},
					Expected: []string{},
				},
				{
					Name:     "SingleArg",
					Args:     []string{"--out", "influxdb=http://localhost:8086/k6"},
					Expected: []string{"influxdb=http://localhost:8086/k6"},
				},
				{
					Name:     "MultiArg",
					Args:     []string{"--out", "influxdb=http://localhost:8086/k6", "--out", "json=test.json"},
					Expected: []string{"influxdb=http://localhost:8086/k6", "json=test.json"},
				},
			},
		},
	}

	for _, data := range testdata {
		t.Run(data.Name, func(t *testing.T) {
			t.Parallel()
			for _, test := range data.Tests {
				t.Run(`"`+test.Name+`"`, func(t *testing.T) {
					t.Parallel()
					fs := configFlagSet()
					fs.AddFlagSet(optionFlagSet())
					assert.NoError(t, fs.Parse(test.Args))

					config, err := getConfig(fs)
					assert.NoError(t, err)
					assert.Equal(t, test.Expected, config.Out)
				})
			}
		})
	}
}

func TestConfigEnv(t *testing.T) {
	t.Parallel()
	testdata := map[struct{ Name, Key string }]map[string]func(Config){
		{"Linger", "K6_LINGER"}: {
			"":      func(c Config) { assert.Equal(t, null.Bool{}, c.Linger) },
			"true":  func(c Config) { assert.Equal(t, null.BoolFrom(true), c.Linger) },
			"false": func(c Config) { assert.Equal(t, null.BoolFrom(false), c.Linger) },
		},
		{"NoUsageReport", "K6_NO_USAGE_REPORT"}: {
			"":      func(c Config) { assert.Equal(t, null.Bool{}, c.NoUsageReport) },
			"true":  func(c Config) { assert.Equal(t, null.BoolFrom(true), c.NoUsageReport) },
			"false": func(c Config) { assert.Equal(t, null.BoolFrom(false), c.NoUsageReport) },
		},
		{"Out", "K6_OUT"}: {
			"":         func(c Config) { assert.Equal(t, []string{}, c.Out) },
			"influxdb": func(c Config) { assert.Equal(t, []string{"influxdb"}, c.Out) },
		},
	}
	for field, data := range testdata {
		field, data := field, data
		t.Run(field.Name, func(t *testing.T) {
			t.Parallel()
			for value, fn := range data {
				value, fn := value, fn
				t.Run(`"`+value+`"`, func(t *testing.T) {
					t.Parallel()
					var config Config
					assert.NoError(t, envconfig.Process("", &config, func(key string) (string, bool) {
						if key == field.Key {
							return value, true
						}
						return "", false
					}))
					fn(config)
				})
			}
		})
	}
}

func TestConfigApply(t *testing.T) {
	t.Parallel()
	t.Run("Linger", func(t *testing.T) {
		t.Parallel()
		conf := Config{}.Apply(Config{Linger: null.BoolFrom(true)})
		assert.Equal(t, null.BoolFrom(true), conf.Linger)
	})
	t.Run("NoUsageReport", func(t *testing.T) {
		t.Parallel()
		conf := Config{}.Apply(Config{NoUsageReport: null.BoolFrom(true)})
		assert.Equal(t, null.BoolFrom(true), conf.NoUsageReport)
	})
	t.Run("Out", func(t *testing.T) {
		t.Parallel()
		conf := Config{}.Apply(Config{Out: []string{"influxdb"}})
		assert.Equal(t, []string{"influxdb"}, conf.Out)

		conf = Config{}.Apply(Config{Out: []string{"influxdb", "json"}})
		assert.Equal(t, []string{"influxdb", "json"}, conf.Out)
	})
}

func TestDeriveAndValidateConfig(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		conf   Config
		isExec bool
		err    string
	}{
		{"defaultOK", Config{}, true, ""},
		{
			"defaultErr",
			Config{},
			false,
			"executor default: function 'default' not found in exports",
		},
		{
			"nonDefaultOK", Config{Options: lib.Options{Scenarios: lib.ScenarioConfigs{
				"per_vu_iters": executor.PerVUIterationsConfig{
					BaseConfig: executor.BaseConfig{
						Name: "per_vu_iters", Type: "per-vu-iterations", Exec: null.StringFrom("nonDefault"),
					},
					VUs:         null.IntFrom(1),
					Iterations:  null.IntFrom(1),
					MaxDuration: types.NullDurationFrom(time.Second),
				},
			}}}, true, "",
		},
		{
			"nonDefaultErr",
			Config{Options: lib.Options{Scenarios: lib.ScenarioConfigs{
				"per_vu_iters": executor.PerVUIterationsConfig{
					BaseConfig: executor.BaseConfig{
						Name: "per_vu_iters", Type: "per-vu-iterations", Exec: null.StringFrom("nonDefaultErr"),
					},
					VUs:         null.IntFrom(1),
					Iterations:  null.IntFrom(1),
					MaxDuration: types.NullDurationFrom(time.Second),
				},
			}}},
			false,
			"executor per_vu_iters: function 'nonDefaultErr' not found in exports",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := deriveAndValidateConfig(tc.conf, metrics.NewRegistry(),
				func(_ string) bool { return tc.isExec }, nil)
			if tc.err != "" {
				var ecerr errext.HasExitCode
				assert.ErrorAs(t, err, &ecerr)
				assert.Equal(t, exitcodes.InvalidConfig, ecerr.ExitCode())
				assert.Contains(t, err.Error(), tc.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	t.Parallel()

	// A registry filled with builtin metrics
	builtinMetricsRegistry := metrics.NewRegistry()
	metrics.RegisterBuiltinMetrics(builtinMetricsRegistry)

	// A registry filled with a custom metric
	customMetricsRegistry := metrics.NewRegistry()
	customMetricsRegistry.MustNewMetric("counter_ok", stats.Counter)

	testCases := []struct {
		name         string
		conf         Config
		registry     *metrics.Registry
		wantErr      bool
		wantExitCode bool
	}{
		{
			name: "config applying a threshold over an existing builtin metric succeeds",
			conf: Config{Options: lib.Options{Thresholds: map[string]stats.Thresholds{
				metrics.HTTPReqsName: {Thresholds: []*stats.Threshold{
					{Parsed: stats.NewThresholdExpression(stats.TokenCount, null.FloatFromPtr(nil), stats.TokenGreater, 1)},
				}},
			}}},
			registry:     builtinMetricsRegistry,
			wantErr:      false,
			wantExitCode: false,
		},
		{
			name: "config applying a threshold over an existing custom metric succeeds",
			conf: Config{Options: lib.Options{Thresholds: map[string]stats.Thresholds{
				"counter_ok": {Thresholds: []*stats.Threshold{
					{Parsed: stats.NewThresholdExpression(stats.TokenCount, null.FloatFromPtr(nil), stats.TokenGreater, 1)},
				}},
			}}},
			registry:     customMetricsRegistry,
			wantErr:      false,
			wantExitCode: false,
		},
		{
			name:         "passes when no thresholds are defined, and the provided registry is nil",
			conf:         Config{},
			registry:     nil,
			wantErr:      false,
			wantExitCode: false,
		},
		{
			name: "fails when thresholds are defined, and the provided registry is nil",
			conf: Config{Options: lib.Options{Thresholds: map[string]stats.Thresholds{
				"counter_ok": {Thresholds: []*stats.Threshold{
					{Parsed: stats.NewThresholdExpression(stats.TokenCount, null.FloatFromPtr(nil), stats.TokenGreater, 1)},
				}},
			}}},
			registry:     nil,
			wantErr:      true,
			wantExitCode: false,
		},
		{
			name: "config applying a threshold to a non-existing metric fails",
			conf: Config{Options: lib.Options{Thresholds: map[string]stats.Thresholds{
				"nonexisting": {Thresholds: []*stats.Threshold{
					{Parsed: stats.NewThresholdExpression(stats.TokenCount, null.FloatFromPtr(nil), stats.TokenGreater, 1)},
				}},
			}}},
			registry:     builtinMetricsRegistry,
			wantErr:      true,
			wantExitCode: true,
		},
		{
			name: "config applying a threshold to an existing metric not supporting its aggregation method",
			conf: Config{Options: lib.Options{Thresholds: map[string]stats.Thresholds{
				metrics.HTTPReqFailedName: {Thresholds: []*stats.Threshold{
					{Parsed: stats.NewThresholdExpression(stats.TokenPercentile, null.NewFloat(99, true), stats.TokenGreater, 1)},
				}},
			}}},
			registry:     builtinMetricsRegistry,
			wantErr:      true,
			wantExitCode: true,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// TODO: when/if scenario validation is tested, the isExecutable should be part of testCase
			gotErr := validateConfig(testCase.conf, testCase.registry, func(string) bool { return true })

			assert.Equal(t, testCase.wantErr, gotErr != nil)
			if testCase.wantErr == true && testCase.wantExitCode == true {
				var ecerr errext.HasExitCode
				assert.ErrorAs(t, gotErr, &ecerr)
				assert.Equal(t, exitcodes.InvalidConfig, ecerr.ExitCode())
			}
		})
	}
}

func TestValidateThresholdsConfig(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		thresholds []*stats.Threshold
		metric     *stats.Metric
		wantErr    bool
	}{
		{
			name: "threshold using count method over Counter metric is valid",
			thresholds: []*stats.Threshold{
				{Parsed: stats.NewThresholdExpression(stats.TokenCount, null.FloatFromPtr(nil), stats.TokenGreater, 1)},
			},
			metric:  &stats.Metric{Type: stats.Counter},
			wantErr: false,
		},
		{
			name: "threshold using rate method over Counter metric is valid",
			thresholds: []*stats.Threshold{
				{Parsed: stats.NewThresholdExpression(stats.TokenRate, null.FloatFromPtr(nil), stats.TokenGreater, 1)},
			},
			metric:  &stats.Metric{Type: stats.Counter},
			wantErr: false,
		},
		{
			name: "threshold using unsupported method over Counter metric is invalid",
			thresholds: []*stats.Threshold{
				{Parsed: stats.NewThresholdExpression(stats.TokenValue, null.FloatFromPtr(nil), stats.TokenGreater, 1)},
			},
			metric:  &stats.Metric{Type: stats.Counter},
			wantErr: true,
		},
		{
			name: "mixed threshold using supported/unsupported method over Counter metric is invalid",
			thresholds: []*stats.Threshold{
				{
					// valid method over counter metric
					Parsed: stats.NewThresholdExpression(stats.TokenCount, null.FloatFromPtr(nil), stats.TokenGreater, 1),
				},
				{
					// invalid method over counter metric
					Parsed: stats.NewThresholdExpression(stats.TokenValue, null.FloatFromPtr(nil), stats.TokenGreater, 1),
				},
			},
			metric:  &stats.Metric{Type: stats.Counter},
			wantErr: true,
		},
		{
			name: "threshold using value method over Gauge metric is valid",
			thresholds: []*stats.Threshold{
				{Parsed: stats.NewThresholdExpression(stats.TokenValue, null.FloatFromPtr(nil), stats.TokenGreater, 1)},
			},
			metric:  &stats.Metric{Type: stats.Gauge},
			wantErr: false,
		},
		{
			name: "threshold using unsupported method over Gauge metric is invalid",
			thresholds: []*stats.Threshold{
				{Parsed: stats.NewThresholdExpression(stats.TokenRate, null.FloatFromPtr(nil), stats.TokenGreater, 1)},
			},
			metric:  &stats.Metric{Type: stats.Gauge},
			wantErr: true,
		},
		{
			name: "mixed threshold using supported/unsupported method over Gauge metric is invalid",
			thresholds: []*stats.Threshold{
				{
					// valid method over counter metric
					Parsed: stats.NewThresholdExpression(stats.TokenValue, null.FloatFromPtr(nil), stats.TokenGreater, 1),
				},
				{
					// invalid method over counter metric
					Parsed: stats.NewThresholdExpression(stats.TokenRate, null.FloatFromPtr(nil), stats.TokenGreater, 1),
				},
			},
			metric:  &stats.Metric{Type: stats.Gauge},
			wantErr: true,
		},
		{
			name: "threshold using rate method over Rate metric is valid",
			thresholds: []*stats.Threshold{
				{Parsed: stats.NewThresholdExpression(stats.TokenRate, null.FloatFromPtr(nil), stats.TokenGreater, 1)},
			},
			metric:  &stats.Metric{Type: stats.Rate},
			wantErr: false,
		},
		{
			name: "threshold using unsupported method over Rate metric is invalid",
			thresholds: []*stats.Threshold{
				{Parsed: stats.NewThresholdExpression(stats.TokenAvg, null.FloatFromPtr(nil), stats.TokenGreater, 1)},
			},
			metric:  &stats.Metric{Type: stats.Rate},
			wantErr: true,
		},
		{
			name: "mixed threshold using supported/unsupported method over Rate metric is invalid",
			thresholds: []*stats.Threshold{
				{
					// valid method over counter metric
					Parsed: stats.NewThresholdExpression(stats.TokenRate, null.FloatFromPtr(nil), stats.TokenGreater, 1),
				},
				{
					// invalid method over counter metric
					Parsed: stats.NewThresholdExpression(stats.TokenAvg, null.FloatFromPtr(nil), stats.TokenGreater, 1),
				},
			},
			metric:  &stats.Metric{Type: stats.Gauge},
			wantErr: true,
		},

		{
			name: "threshold using avg method over Trend metric is valid",
			thresholds: []*stats.Threshold{
				{Parsed: stats.NewThresholdExpression(stats.TokenAvg, null.FloatFromPtr(nil), stats.TokenGreater, 1)},
			},
			metric:  &stats.Metric{Type: stats.Trend},
			wantErr: false,
		},
		{
			name: "threshold using min method over Trend metric is valid",
			thresholds: []*stats.Threshold{
				{Parsed: stats.NewThresholdExpression(stats.TokenMin, null.FloatFromPtr(nil), stats.TokenGreater, 1)},
			},
			metric:  &stats.Metric{Type: stats.Trend},
			wantErr: false,
		},
		{
			name: "threshold using med method over Trend metric is valid",
			thresholds: []*stats.Threshold{
				{Parsed: stats.NewThresholdExpression(stats.TokenMed, null.FloatFromPtr(nil), stats.TokenGreater, 1)},
			},
			metric:  &stats.Metric{Type: stats.Trend},
			wantErr: false,
		},
		{
			name: "threshold using p(N) method over Trend metric is valid",
			thresholds: []*stats.Threshold{
				{Parsed: stats.NewThresholdExpression(stats.TokenPercentile, null.NewFloat(99, true), stats.TokenGreater, 1)},
			},
			metric:  &stats.Metric{Type: stats.Trend},
			wantErr: false,
		},
		{
			name: "threshold using unsupported method over Trend metric is invalid",
			thresholds: []*stats.Threshold{
				{Parsed: stats.NewThresholdExpression(stats.TokenCount, null.FloatFromPtr(nil), stats.TokenGreater, 1)},
			},
			metric:  &stats.Metric{Type: stats.Trend},
			wantErr: true,
		},
		{
			name: "mixed threshold using supported/unsupported method over Trend metric is invalid",
			thresholds: []*stats.Threshold{
				{
					// valid method over counter metric
					Parsed: stats.NewThresholdExpression(stats.TokenAvg, null.FloatFromPtr(nil), stats.TokenGreater, 1),
				},
				{
					// invalid method over counter metric
					Parsed: stats.NewThresholdExpression(stats.TokenCount, null.FloatFromPtr(nil), stats.TokenGreater, 1),
				},
			},
			metric:  &stats.Metric{Type: stats.Trend},
			wantErr: true,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			wrappedThresholds := stats.Thresholds{Thresholds: testCase.thresholds}

			gotErr := validateThresholdsConfig("ignoreme", wrappedThresholds, testCase.metric)

			assert.Equal(t, testCase.wantErr, gotErr != nil)
		})
	}
}

// FIXME: Remove these...
// func TestValidateThresholdsConfigWithNilRegistry(t *testing.T) {
// 	t.Parallel()
// 	var registry *metrics.Registry
// 	config := Config{}
// 	var wantErrType errext.HasExitCode

// 	gotErr := validateThresholdsConfig(config, registry)

// 	assert.Error(t, gotErr, "validateThresholdsConfig should fail when passed registry is nil")
// 	assert.ErrorAs(t, gotErr, &wantErrType, "validateThresholdsConfig error should be an instance of errext.HasExitCode")
// }

// func TestValidateThresholdsConfigAppliesToBuiltinMetrics(t *testing.T) {
// 	t.Parallel()
// 	// Prepare a registry loaded with builtin metrics
// 	registry := metrics.NewRegistry()
// 	metrics.RegisterBuiltinMetrics(registry)

// 	// Assuming builtin metrics are indeed registered, and
// 	// thresholds parsing works as expected, we prepare
// 	// thresholds for a counter builting metric; namely http_reqs
// 	HTTPReqsThresholds, err := stats.NewThresholds([]string{"count>0", "rate>1"})
// 	require.NoError(t, err, "instantiating Thresholds with expression 'count>0' should not fail")
// 	options := lib.Options{
// 		Thresholds: map[string]stats.Thresholds{
// 			metrics.HTTPReqsName: HTTPReqsThresholds,
// 		},
// 	}
// 	config := Config{Options: options}

// 	gotErr := validateThresholdsConfig(config, registry)

// 	assert.NoError(t, gotErr, "validateThresholdsConfig should not fail against builtin metrics")
// }

// func TestValidateThresholdsConfigAppliesToCustomMetrics(t *testing.T) {
// 	t.Parallel()

// 	// Prepare a registry loaded with both builtin metrics,
// 	// and a custom counter metric.
// 	testCounterMetricName := "testcounter"
// 	registry := metrics.NewRegistry()
// 	metrics.RegisterBuiltinMetrics(registry)
// 	_, err := registry.NewMetric(testCounterMetricName, stats.Counter)
// 	require.NoError(t, err, "registering custom counter metric should not fail")

// 	// Prepare a configuration containing a Threshold
// 	counterThresholds, err := stats.NewThresholds([]string{"count>0", "rate>1"})
// 	require.NoError(t, err, "instantiating Thresholds with expression 'count>0' should not fail")
// 	options := lib.Options{
// 		Thresholds: map[string]stats.Thresholds{
// 			testCounterMetricName: counterThresholds,
// 		},
// 	}
// 	config := Config{Options: options}

// 	gotErr := validateThresholdsConfig(config, registry)

// 	// Assert
// 	assert.NoError(t, gotErr, "validateThresholdsConfig should not fail against existing and valid custom metric")
// }

// func TestValidateThresholdsConfigFailsOnNonExistingMetric(t *testing.T) {
// 	t.Parallel()

// 	// Prepare a registry loaded with builtin metrics only
// 	registry := metrics.NewRegistry()
// 	metrics.RegisterBuiltinMetrics(registry)

// 	// Prepare a configuration containing a Threshold applying to
// 	// a non-existing metric
// 	counterThresholds, err := stats.NewThresholds([]string{"count>0", "rate>1"})
// 	require.NoError(t, err, "instantiating Thresholds with expression 'count>0' should not fail")
// 	options := lib.Options{
// 		Thresholds: map[string]stats.Thresholds{
// 			"nonexisting": counterThresholds,
// 		},
// 	}
// 	config := Config{Options: options}
// 	var wantErrType errext.HasExitCode

// 	gotErr := validateThresholdsConfig(config, registry)

// 	// Assert
// 	assert.Error(t, gotErr, "validateThresholdsConfig should fail on thresholds applied to a non-existing metric")
// 	assert.ErrorAs(t, gotErr, &wantErrType, "validateThresholdsConfig error should be an instance of errext.HasExitCode")
// }

// func TestValidateThresholdsConfigFailsOnThresholdInvalidMetricType(t *testing.T) {
// 	t.Parallel()

// 	// Prepare a registry loaded with builtin metrics only
// 	registry := metrics.NewRegistry()
// 	metrics.RegisterBuiltinMetrics(registry)

// 	// Prepare a configuration containing a Threshold using a Counter metric
// 	// specific aggregation method, against a metric of type Gauge: which doesn't support
// 	// that method.
// 	VUsThresholds, err := stats.NewThresholds([]string{"count>0"})
// 	require.NoError(t, err, "instantiating Thresholds with expression 'count>0' should not fail")
// 	options := lib.Options{
// 		Thresholds: map[string]stats.Thresholds{
// 			metrics.VUsName: VUsThresholds,
// 		},
// 	}
// 	config := Config{Options: options}
// 	var wantErrType errext.HasExitCode

// 	gotErr := validateThresholdsConfig(config, registry)

// 	// Assert
// 	assert.Error(t, gotErr, "validateThresholdsConfig should fail applying the count method to a Gauge metric")
// 	assert.ErrorAs(t, gotErr, &wantErrType, "validateThresholdsConfig error should be an instance of errext.HasExitCode")
// }
