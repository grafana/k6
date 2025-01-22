package js

import (
	_ "embed" // this is used to embed the contents of summary.js
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

// Copied from https://github.com/k6io/jslib.k6.io/tree/master/lib/k6-summary
//
//go:embed summary.js
var jslibSummaryCode string

//go:embed summary-wrapper.js
var summaryWrapperLambdaCode string

// TODO: figure out something saner... refactor the sinks and how we deal with
// metrics in general... so much pain and misery... :sob:
func metricValueGetter(summaryTrendStats []string) func(metrics.Sink, time.Duration) map[string]float64 {
	trendResolvers, err := metrics.GetResolversForTrendColumns(summaryTrendStats)
	if err != nil {
		panic(err.Error()) // this should have been validated already
	}

	return func(sink metrics.Sink, t time.Duration) (result map[string]float64) {
		switch sink := sink.(type) {
		case *metrics.CounterSink:
			result = sink.Format(t)
			rate := 0.0
			if t > 0 {
				rate = sink.Value / (float64(t) / float64(time.Second))
			}
			result["rate"] = rate
		case *metrics.GaugeSink:
			result = sink.Format(t)
			result["min"] = sink.Min
			result["max"] = sink.Max
		case *metrics.RateSink:
			result = sink.Format(t)
			result["passes"] = float64(sink.Trues)
			result["fails"] = float64(sink.Total - sink.Trues)
		case *metrics.TrendSink:
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
func summarizeMetricsToObject(data *lib.Summary, options lib.Options, setupData []byte) map[string]interface{} {
	m := make(map[string]interface{})
	m["root_group"] = exportGroup(data.RootGroup)
	m["options"] = map[string]interface{}{
		// TODO: improve when we can easily export all option values, including defaults?
		"summaryTrendStats": options.SummaryTrendStats,
		"summaryTimeUnit":   options.SummaryTimeUnit.String,
		"noColor":           data.NoColor, // TODO: move to the (runtime) options
	}
	m["state"] = map[string]interface{}{
		"isStdOutTTY":       data.UIState.IsStdOutTTY,
		"isStdErrTTY":       data.UIState.IsStdErrTTY,
		"testRunDurationMs": float64(data.TestRunDuration) / float64(time.Millisecond),
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

	var setupDataI interface{}
	if setupData != nil {
		if err := json.Unmarshal(setupData, &setupDataI); err != nil {
			// TODO: log the error
			return m
		}
	} else {
		setupDataI = sobek.Undefined()
	}

	m["setup_data"] = setupDataI

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

func getSummaryResult(rawResult sobek.Value) (map[string]io.Reader, error) {
	if sobek.IsNull(rawResult) || sobek.IsUndefined(rawResult) {
		return nil, nil //nolint:nilnil // this is actually valid result in this case
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
