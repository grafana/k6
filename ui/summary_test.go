/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2018 Load Impact
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

package ui

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
)

func TestSummary(t *testing.T) {
	t.Run("SummarizeMetrics", func(t *testing.T) {
		var (
			checksOut = "     █ child\n\n       ✗ check1\n        ↳  33% — ✓ 5 / ✗ 10\n\n" +
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
			{[]string{"avg", "min", "med", "max", "p(90)", "p(95)", "p(99.9)"},
				checksOut + countOut + trendOut + gaugeOut},
			{[]string{"count"}, checksOut + countOut + "   ✗ my_trend....: count=3\n" + gaugeOut},
			{[]string{"avg", "count"}, checksOut + countOut + "   ✗ my_trend....: avg=15ms count=3\n" + gaugeOut},
		}

		rootG, _ := lib.NewGroup("", nil)
		childG, _ := rootG.Group("child")
		check, _ := lib.NewCheck("check1", childG)
		check.Passes = 5
		check.Fails = 10
		childG.Checks["check1"] = check
		for _, tc := range testCases {
			tc := tc
			t.Run(fmt.Sprintf("%v", tc.stats), func(t *testing.T) {
				var w bytes.Buffer
				s := NewSummary(tc.stats)

				s.SummarizeMetrics(&w, " ", SummaryData{
					Metrics:   metrics,
					RootGroup: rootG,
					Time:      time.Second,
					TimeUnit:  "",
				})
				assert.Equal(t, tc.expected, w.String())
			})
		}
	})

	t.Run("generateCustomTrendValueResolvers", func(t *testing.T) {
		var customResolversTests = []struct {
			stats      []string
			percentile float64
		}{
			{[]string{"p(99)", "p(err)"}, 0.99},
			{[]string{"p(none", "p(99.9)"}, 0.9990000000000001},
			{[]string{"p(none", "p(99.99)"}, 0.9998999999999999},
			{[]string{"p(none", "p(99.999)"}, 0.9999899999999999},
		}

		sink := createTestTrendSink(100)

		for _, tc := range customResolversTests {
			tc := tc
			t.Run(fmt.Sprintf("%v", tc.stats), func(t *testing.T) {
				s := Summary{trendColumns: tc.stats}
				res := s.generateCustomTrendValueResolvers(tc.stats)
				assert.Len(t, res, 1)
				for k := range res {
					assert.Equal(t, sink.P(tc.percentile), res[k](sink))
				}
			})
		}
	})
}

func TestValidateSummary(t *testing.T) {
	var validateTests = []struct {
		stats  []string
		expErr error
	}{
		{[]string{}, nil},
		{[]string{"avg", "min", "med", "max", "p(0)", "p(99)", "p(99.999)", "count"}, nil},
		{[]string{"avg", "p(err)"}, ErrInvalidStat{"p(err)", errPercentileStatInvalidValue}},
		{[]string{"nil", "p(err)"}, ErrInvalidStat{"nil", errStatUnknownFormat}},
		{[]string{"p90"}, ErrInvalidStat{"p90", errStatUnknownFormat}},
		{[]string{"p(90"}, ErrInvalidStat{"p(90", errStatUnknownFormat}},
		{[]string{" avg"}, ErrInvalidStat{" avg", errStatUnknownFormat}},
		{[]string{"avg "}, ErrInvalidStat{"avg ", errStatUnknownFormat}},
		{[]string{"", "avg "}, ErrInvalidStat{"", errStatEmptyString}},
	}

	for _, tc := range validateTests {
		tc := tc
		t.Run(fmt.Sprintf("%v", tc.stats), func(t *testing.T) {
			err := ValidateSummary(tc.stats)
			assert.Equal(t, tc.expErr, err)
		})
	}
}

func createTestTrendSink(count int) *stats.TrendSink {
	sink := stats.TrendSink{}

	for i := 0; i < count; i++ {
		sink.Add(stats.Sample{Value: float64(i)})
	}

	return &sink
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
                        "passes": 5,
                        "fails": 10
                    }
                }
            }
        },
        "checks": {}
    },
    "metrics": {
        "checks": {
            "value": 0,
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
	rootG, _ := lib.NewGroup("", nil)
	childG, _ := rootG.Group("child")
	check, _ := lib.NewCheck("check1", childG)
	check.Passes = 5
	check.Fails = 10
	childG.Checks["check1"] = check

	s := NewSummary([]string{"avg", "min", "med", "max", "p(90)", "p(95)", "p(99.9)"})
	data := SummaryData{
		Metrics:   metrics,
		RootGroup: rootG,
		Time:      time.Second,
		TimeUnit:  "",
	}

	var w bytes.Buffer
	err := s.SummarizeMetricsJSON(&w, data)
	require.Nil(t, err)
	require.Contains(t, w.String(), "<")
	require.JSONEq(t, expected, w.String())
}
