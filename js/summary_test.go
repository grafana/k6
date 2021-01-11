/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
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

package js

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
)

func TestTextSummary(t *testing.T) {
	var (
		checksOut = "     █ child\n\n" +
			"       ✗ check1\n        ↳  33% — ✓ 5 / ✗ 10\n" +
			"       ✗ check2\n        ↳  66% — ✓ 10 / ✗ 5\n\n" +
			"   ✓ checks......: 100.00% ✓ 3   ✗ 0  \n"
		countOut = "   ✗ http_reqs...: 3       3/s\n"
		gaugeOut = "     vus.........: 1       min=1 max=1\n"
		trendOut = "   ✗ my_trend....: avg=15ms min=10ms med=15ms max=20ms p(90)=19ms " +
			"p(95)=19.5ms p(99.9)=19.99ms\n"
	)

	metrics := createTestMetrics()
	testCases := []struct {
		stats    []string
		expected string
	}{
		{
			[]string{"avg", "min", "med", "max", "p(90)", "p(95)", "p(99.9)"},
			checksOut + countOut + trendOut + gaugeOut,
		},
		{[]string{"count"}, checksOut + countOut + "   ✗ my_trend....: count=3\n" + gaugeOut},
		{[]string{"avg", "count"}, checksOut + countOut + "   ✗ my_trend....: avg=15ms count=3\n" + gaugeOut},
	}

	rootG, err := lib.NewGroup("", nil)
	require.NoError(t, err)
	childG, err := rootG.Group("child")
	require.NoError(t, err)
	check1, err := childG.Check("check1")
	require.NoError(t, err)
	check1.Passes = 5
	check1.Fails = 10
	check2, err := childG.Check("check2")
	require.NoError(t, err)
	check2.Passes = 10
	check2.Fails = 5
	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("%v", tc.stats), func(t *testing.T) {
			trendStats, err := json.Marshal(tc.stats)
			require.NoError(t, err)
			runner, err := getSimpleRunner(
				t, "/script.js",
				fmt.Sprintf(`
					exports.options = {summaryTrendStats: %s};
					exports.default = function() {};
				`, string(trendStats)),
				lib.RuntimeOptions{
					CompatibilityMode: null.NewString("base", true),
				},
			)
			require.NoError(t, err)

			result, err := runner.HandleSummary(context.Background(), &lib.Summary{
				Metrics:         metrics,
				RootGroup:       rootG,
				TestRunDuration: time.Second,
			})
			require.NoError(t, err)

			require.Len(t, result, 1)
			stdout := result["stdout"]
			require.NotNil(t, stdout)
			summary, err := ioutil.ReadAll(stdout)
			require.NoError(t, err)
			assert.Equal(t, "\n"+tc.expected+"\n", string(summary))
		})
	}
}

func createTestMetrics() map[string]*stats.Metric {
	metrics := make(map[string]*stats.Metric)
	gaugeMetric := stats.New("vus", stats.Gauge)
	gaugeMetric.Sink.Add(stats.Sample{Value: 1})

	countMetric := stats.New("http_reqs", stats.Counter)
	countMetric.Tainted = null.BoolFrom(true)
	countMetric.Thresholds = stats.Thresholds{Thresholds: []*stats.Threshold{{Source: "rate<100"}}}

	checksMetric := stats.New("checks", stats.Rate)
	checksMetric.Tainted = null.BoolFrom(false)
	sink := &stats.TrendSink{}

	samples := []float64{10.0, 15.0, 20.0}
	for _, s := range samples {
		sink.Add(stats.Sample{Value: s})
		checksMetric.Sink.Add(stats.Sample{Value: 1})
		countMetric.Sink.Add(stats.Sample{Value: 1})
	}

	metrics["vus"] = gaugeMetric
	metrics["http_reqs"] = countMetric
	metrics["checks"] = checksMetric
	metrics["my_trend"] = &stats.Metric{
		Name:     "my_trend",
		Type:     stats.Trend,
		Contains: stats.Time,
		Sink:     sink,
		Tainted:  null.BoolFrom(true),
		Thresholds: stats.Thresholds{
			Thresholds: []*stats.Threshold{
				{
					Source:     "my_trend<1000",
					LastFailed: true,
				},
			},
		},
	}

	return metrics
}

func TestSummarizeMetricsJSON(t *testing.T) {
	metrics := createTestMetrics()
	expected := `{
    "root_group": {
        "name": "",
        "path": "",
        "id": "d41d8cd98f00b204e9800998ecf8427e",
        "groups": {
            "child": {
                "name": "child",
                "path": "::child",
                "id": "f41cbb53a398ec1c9fb3d33e20c9b040",
                "groups": {},
                "checks": {
                    "check1": {
                        "name": "check1",
                        "path": "::child::check1",
                        "id": "6289a7a06253a1c3f6137dfb25695563",
                        "passes": 8,
                        "fails": 2
                    }
                }
            }
        },
        "checks": {}
    },
    "metrics": {
        "checks": {
            "value": 1,
            "passes": 3,
            "fails": 0
        },
        "http_reqs": {
            "count": 3,
            "rate": 3,
            "thresholds": {
                "rate<100": false
            }
        },
        "my_trend": {
            "avg": 15,
            "max": 20,
            "med": 15,
            "min": 10,
            "p(90)": 19,
            "p(95)": 19.5,
            "p(99)": 19.9,
            "thresholds": {
                "my_trend<1000": true
            }
        },
        "vus": {
            "value": 1,
            "min": 1,
            "max": 1
        }
    }
}
`
	rootG, err := lib.NewGroup("", nil)
	require.NoError(t, err)
	childG, err := rootG.Group("child")
	require.NoError(t, err)
	// TODO: actually have a better example without conflicting check metrics...
	check, err := childG.Check("check1")
	require.NoError(t, err)
	check.Passes = 8
	check.Fails = 2

	runner, err := getSimpleRunner(
		t, "/script.js",
		`
		exports.options = {summaryTrendStats: ["avg", "min", "med", "max", "p(90)", "p(95)", "p(99)"]};
		exports.default = function() {};
		`,
		lib.RuntimeOptions{
			CompatibilityMode: null.NewString("base", true),
			SummaryExport:     null.StringFrom("result.json"),
		},
	)

	require.NoError(t, err)

	result, err := runner.HandleSummary(context.Background(), &lib.Summary{
		Metrics:         metrics,
		RootGroup:       rootG,
		TestRunDuration: time.Second,
	})
	require.NoError(t, err)

	require.Len(t, result, 2)
	require.NotNil(t, result["stdout"])
	require.NotNil(t, result["result.json"])
	jsonExport, err := ioutil.ReadAll(result["result.json"])
	require.NoError(t, err)
	assert.JSONEq(t, expected, string(jsonExport))
}
