package js

import (
	_ "embed" // this is used to embed the contents of summary.js
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/lib/summary"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

// Copied from https://github.com/k6io/jslib.k6.io/tree/master/lib/k6-summary
//
//go:embed summary.js
var jslibSummaryCode string

// TODO: Remove me once we stop supporting the legacy summary.
//
//go:embed summary-legacy.js
var jslibSummaryLegacyCode string

//go:embed summary-wrapper.js
var summaryWrapperLambdaCode string

// summarizeReportToObject is a method that makes certain transformations to the given [summary.Summary]
// in order to make it more suitable for the JS code that prints out the end-of-test summary.
// That includes, non-exclusively, things like transforming certain Go maps into JS objects to preserve
// the desired order of their keys (e.g. we want to display group results in the same order as groups are defined
// in code), as that's one of the characteristics of JS objects while it doesn't apply to Go maps (not preserved).
// TODO: Map "checks" into a JS object to get rid of "OrderedChecks" and remove that logic from summary.js.
// TODO: Explore if it's possible to apply the same idea with Thresholds, in order to preserve order from code.
func summarizeReportToObject(rt *sobek.Runtime, s *summary.Summary) (map[string]interface{}, error) {
	// We use a JS object to preserve insertion order of groups, so we don't need to pass
	// an extra array with the order of groups, but just an object.
	var mapGroups func(groups map[string]summary.Group, sorted []string) (*sobek.Object, error)

	mapGroup := func(g summary.Group) (map[string]interface{}, error) {
		gGroupsObj, err := mapGroups(g.Groups, g.GroupsOrder)
		if err != nil {
			return nil, err
		}

		groupObj := make(map[string]interface{})
		groupObj["checks"] = g.Checks
		groupObj["metrics"] = g.Metrics
		groupObj["groups"] = gGroupsObj
		return groupObj, nil
	}

	mapGroups = func(groups map[string]summary.Group, sorted []string) (*sobek.Object, error) {
		baseObj := rt.NewObject()
		for _, gName := range sorted {
			groupObj, err := mapGroup(groups[gName])
			if err != nil {
				return nil, err
			}

			if err := baseObj.Set(gName, groupObj); err != nil {
				return nil, err
			}
		}
		return baseObj, nil
	}

	groups, err := mapGroups(s.Groups, s.GroupsOrder)
	if err != nil {
		return nil, err
	}

	m := make(map[string]interface{})
	m["thresholds"] = s.Thresholds
	m["checks"] = s.Checks
	m["metrics"] = s.Metrics
	m["groups"] = groups
	m["scenarios"] = s.Scenarios
	return m, nil
}

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
			result["rate"] = sink.Rate(t)
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
func summarizeMetricsToObject(data *lib.LegacySummary, options lib.Options, setupData []byte) map[string]interface{} {
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
