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
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/ui"
)

// TODO: move this to a separate JS file and use go.rice to embed it
const summaryWrapperLambdaCode = `
(function() {
	var forEach = function(obj, callback) {
		for (var key in obj) {
			if (obj.hasOwnProperty(key)) {
				callback(key, obj[key]);
			}
		}
	}

	var transformGroup = function(group) {
		if (Array.isArray(group.groups)) {
			var newFormatGroups = group.groups;
			group.groups = {};
			for (var i = 0; i < newFormatGroups.length; i++) {
				group.groups[newFormatGroups[i].name] = transformGroup(newFormatGroups[i]);
			}
		}
		if (Array.isArray(group.checks)) {
			var newFormatChecks = group.checks;
			group.checks = {};
			for (var i = 0; i < newFormatChecks.length; i++) {
				group.checks[newFormatChecks[i].name] = newFormatChecks[i];
			}
		}
		return group;
	};

	var oldJSONSummary = function(data) {
		// Quick copy of the data, since it's easiest to modify it in place.
		var results = JSON.parse(JSON.stringify(data));
		delete results.options;

		forEach(results.metrics, function(metricName, metric) {
			var oldFormatMetric = metric.values;
			if (metric.thresholds && Object.keys(metric.thresholds).length > 0) {
				var newFormatThresholds = metric.thresholds;
				oldFormatMetric.thresholds = {};
				forEach(newFormatThresholds, function(thresholdName, threshold) {
					oldFormatMetric.thresholds[thresholdName] = !threshold.ok;
				});
			}
			if (metric.type == 'rate' && oldFormatMetric.hasOwnProperty('rate')) {
				oldFormatMetric.value = oldFormatMetric.rate; // sigh...
				delete oldFormatMetric.rate;
			}
			results.metrics[metricName] = oldFormatMetric;
		});

		results.root_group = transformGroup(results.root_group);

		return JSON.stringify(results, null, 4);
	};

	var oldTextSummary = function(data) {
		// TODO: implement something like the current end of test summary
	};

	return function(exportedSummaryCallback, jsonSummaryPath, data, oldCallback) {
		var result = {};
		if (exportedSummaryCallback) {
			try {
				result = exportedSummaryCallback(data, oldCallback);
			} catch (e) {
				console.error('handleSummary() failed with error "' + e + '", falling back to the default summary');
				//result["stdout"] = oldTextSummary(data);
				result["stdout"] = oldCallback(); // TODO: delete
			}
		} else {
			// result["stdout"] = oldTextSummary(data);
			result["stdout"] = oldCallback(); // TODO: delete
		}

		// TODO: ensure we're returning a map of strings or null/undefined...
		// and if not, log an error and generate the default summary?

		if (jsonSummaryPath != '') {
			result[jsonSummaryPath] = oldJSONSummary(data);
		}

		return result;
	};
})();
`

// TODO: figure out something saner... refactor the sinks and how we deal with
// metrics in general... so much pain and misery... :sob:
func metricValueGetter(summaryTrendStats []string) func(stats.Sink, time.Duration) map[string]float64 {
	trendResolvers, err := stats.GetResolversForTrendColumns(summaryTrendStats)
	if err != nil {
		panic(err.Error()) // this should have been validated already
	}

	return func(sink stats.Sink, t time.Duration) (result map[string]float64) {
		sink.Calc()

		switch sink := sink.(type) {
		case *stats.CounterSink:
			result = sink.Format(t)
			rate := 0.0
			if t > 0 {
				rate = sink.Value / (float64(t) / float64(time.Second))
			}
			result["rate"] = rate
		case *stats.GaugeSink:
			result = sink.Format(t)
			result["min"] = sink.Min
			result["max"] = sink.Max
		case *stats.RateSink:
			result = sink.Format(t)
			result["passes"] = float64(sink.Trues)
			result["fails"] = float64(sink.Total - sink.Trues)
		case *stats.TrendSink:
			result = make(map[string]float64, len(summaryTrendStats))
			for _, col := range summaryTrendStats {
				result[col] = trendResolvers[col](sink)
			}
		}

		return result
	}
}

// summarizeMetricsToObject transforms the summary objects in a way that's
// suitable to pass to the JS runtime or export to JSON.
func summarizeMetricsToObject(data *lib.Summary, options lib.Options) map[string]interface{} {
	m := make(map[string]interface{})
	m["root_group"] = exportGroup(data.RootGroup)
	m["options"] = map[string]interface{}{
		// TODO: improve when we can easily export all option values, including defaults?
		"summaryTrendStats": options.SummaryTrendStats,
		"summaryTimeUnit":   options.SummaryTimeUnit.String,
	}

	getMetricValues := metricValueGetter(options.SummaryTrendStats)

	metricsData := make(map[string]interface{})
	for name, m := range data.Metrics {
		metricData := map[string]interface{}{
			"type":     m.Type.String(),
			"contains": m.Contains.String(),
			"values":   getMetricValues(m.Sink, data.TestRunDuration),
		}

		if len(m.Thresholds.Thresholds) > 0 {
			thresholds := make(map[string]interface{})
			for _, threshold := range m.Thresholds.Thresholds {
				thresholds[threshold.Source] = map[string]interface{}{
					"ok": !threshold.LastFailed,
				}
			}
			metricData["thresholds"] = thresholds
		}
		metricsData[name] = metricData
	}
	m["metrics"] = metricsData

	return m
}

func exportGroup(group *lib.Group) map[string]interface{} {
	subGroups := make([]map[string]interface{}, len(group.OrderedGroups))
	for i, subGroup := range group.OrderedGroups {
		subGroups[i] = exportGroup(subGroup)
	}

	checks := make([]map[string]interface{}, len(group.OrderedChecks))
	for i, check := range group.OrderedChecks {
		checks[i] = map[string]interface{}{
			"name":   check.Name,
			"path":   check.Path,
			"id":     check.ID,
			"passes": check.Passes,
			"fails":  check.Fails,
		}
	}

	return map[string]interface{}{
		"name":   group.Name,
		"path":   group.Path,
		"id":     group.ID,
		"groups": subGroups,
		"checks": checks,
	}
}

func getSummaryResult(rawResult goja.Value) (map[string]io.Reader, error) {
	if goja.IsNull(rawResult) || goja.IsUndefined(rawResult) {
		return nil, nil
	}

	rawResultMap, ok := rawResult.Export().(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("handleSummary() should return a map with string keys")
	}

	results := make(map[string]io.Reader, len(rawResultMap))
	for path, val := range rawResultMap {
		readerVal, err := common.GetReader(val)
		if err != nil {
			return nil, fmt.Errorf("error handling summary object %s: %w", path, err)
		}
		results[path] = readerVal
	}

	return results, nil
}

// TODO: remove this after the JS alternative is written
func getOldTextSummaryFunc(summary *lib.Summary, options lib.Options) func() string {
	data := ui.SummaryData{
		Metrics:   summary.Metrics,
		RootGroup: summary.RootGroup,
		Time:      summary.TestRunDuration,
		TimeUnit:  options.SummaryTimeUnit.String,
	}

	return func() string {
		buffer := bytes.NewBuffer(nil)
		_ = buffer.WriteByte('\n')

		s := ui.NewSummary(options.SummaryTrendStats)
		s.SummarizeMetrics(buffer, " ", data)

		_ = buffer.WriteByte('\n')

		return buffer.String()
	}
}
