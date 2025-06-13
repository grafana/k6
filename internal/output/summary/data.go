package summary

import (
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"go.k6.io/k6/internal/lib/summary"
	"go.k6.io/k6/metrics"
)

type dataModel struct {
	thresholds
	*aggregatedGroupData
	scenarios map[string]*aggregatedGroupData
}

func newDataModel() dataModel {
	return dataModel{
		thresholds:          make(map[string]metricThresholds),
		aggregatedGroupData: newAggregatedGroupData(),
		scenarios:           make(map[string]*aggregatedGroupData),
	}
}

func (d *dataModel) groupDataFor(scenario string) *aggregatedGroupData {
	if groupData, exists := d.scenarios[scenario]; exists {
		return groupData
	}
	d.scenarios[scenario] = newAggregatedGroupData()
	return d.scenarios[scenario]
}

func (d *dataModel) storeThresholdsFor(m *metrics.Metric) {
	if len(m.Thresholds.Thresholds) == 0 {
		return
	}

	if _, exists := d.thresholds[m.Name]; !exists {
		d.thresholds[m.Name] = metricThresholds{
			aggregatedMetric: relayAggregatedMetricFrom(m),
			tt:               m.Thresholds.Thresholds,
		}
	}
}

type metricThresholds struct {
	aggregatedMetric
	tt []*metrics.Threshold
}

type thresholds map[string]metricThresholds

type aggregatedGroupData struct {
	checks            *aggregatedChecksData
	aggregatedMetrics aggregatedMetricData
	groupsData        map[string]*aggregatedGroupData
	groupsOrder       []string
}

func newAggregatedGroupData() *aggregatedGroupData {
	return &aggregatedGroupData{
		checks:            newAggregatedChecksData(),
		aggregatedMetrics: make(map[string]aggregatedMetric),
		groupsData:        make(map[string]*aggregatedGroupData),
		groupsOrder:       make([]string, 0),
	}
}

func (a *aggregatedGroupData) groupDataFor(group string) *aggregatedGroupData {
	if groupData, exists := a.groupsData[group]; exists {
		return groupData
	}
	newGroupData := newAggregatedGroupData()
	a.groupsData[group] = newGroupData
	a.groupsOrder = append(a.groupsOrder, group)
	return a.groupsData[group]
}

// addSample differs from relayAggregatedMetricFrom in that it updates the internally stored metric sink with the
// sample, which differs from the original metric sink, while relayAggregatedMetricFrom stores the metric and the
// metric sink from the sample's metric.
func (a *aggregatedGroupData) addSample(sample metrics.Sample) {
	a.aggregatedMetrics.addSample(sample)

	checkName, hasCheckTag := sample.Tags.Get(metrics.TagCheck.String())
	if hasCheckTag && sample.Metric.Name == metrics.ChecksName {
		check := a.checks.checkFor(checkName)
		if sample.Value == 0 {
			atomic.AddInt64(&check.Fails, 1)
		} else {
			atomic.AddInt64(&check.Passes, 1)
		}
	}
}

// aggregatedMetricData is a container that can either hold a reference to a k6 metric stored in the registry, or
// hold a pointer to such metric but keeping a separated Sink of values in order to keep an aggregated view of the
// metric values. The latter is useful for tracking aggregated metric values specific to a group or scenario.
type aggregatedMetricData map[string]aggregatedMetric

// relayAggregatedMetricFrom instantiates a new aggregatedMetric by relaying the metric's Sink, so the same Sink
// instance is shared between the original metric and the aggregatedMetric, to avoid duplicating memory allocations.
func relayAggregatedMetricFrom(m *metrics.Metric) aggregatedMetric {
	return aggregatedMetric{
		MetricInfo: summaryMetricInfoFrom(m),
		Sink:       m.Sink,
	}
}

// summaryMetricInfoFrom creates a new summary.MetricInfo from a k6 metric.
func summaryMetricInfoFrom(m *metrics.Metric) summary.MetricInfo {
	return summary.MetricInfo{
		Name:     m.Name,
		Type:     m.Type.String(),
		Contains: m.Contains.String(),
	}
}

// addSample stores the value of the sample in a separate internal sink completely detached from the underlying metrics.
// This allows to keep an aggregated view of the values specific to a group or scenario.
func (a aggregatedMetricData) addSample(sample metrics.Sample) {
	if _, exists := a[sample.Metric.Name]; !exists {
		a[sample.Metric.Name] = newAggregatedMetric(sample.Metric)
	}

	a[sample.Metric.Name].Sink.Add(sample)
}

type aggregatedMetric struct {
	summary.MetricInfo
	Sink metrics.Sink
}

func newAggregatedMetric(m *metrics.Metric) aggregatedMetric {
	return aggregatedMetric{
		MetricInfo: summaryMetricInfoFrom(m),
		Sink:       metrics.NewSink(m.Type),
	}
}

type aggregatedChecksData struct {
	checks        map[string]*summary.Check
	orderedChecks []*summary.Check
}

func newAggregatedChecksData() *aggregatedChecksData {
	return &aggregatedChecksData{
		checks:        make(map[string]*summary.Check),
		orderedChecks: make([]*summary.Check, 0),
	}
}

func (a *aggregatedChecksData) checkFor(name string) *summary.Check {
	check, ok := a.checks[name]
	if !ok {
		check = &summary.Check{
			Name: name,
		}
		a.checks[name] = check
		a.orderedChecks = append(a.orderedChecks, check)
	}
	return check
}

// populateSummaryGroup populates a [summary.Group], which is the common type for holding groups data to be displayed
// in the summary, with the data hold by [aggregatedGroupData], which is the type used specifically by this output
// implementation aimed to collect data that will be displayed in the summary.
func populateSummaryGroup(
	summaryMode summary.Mode,
	summaryGroup *summary.Group,
	groupData *aggregatedGroupData,
	testRunDuration time.Duration,
	summaryTrendStats []string,
) {
	// First, we populate the checks metrics, which are treated independently.
	populateSummaryChecks(summaryGroup, groupData, testRunDuration, summaryTrendStats)

	// Then, we store the metrics.
	storeMetric := func(
		dest summary.Metrics,
		info summary.MetricInfo,
		sink metrics.Sink,
		testDuration time.Duration,
		summaryTrendStats []string,
	) {
		getMetricValues := metricValueGetter(summaryTrendStats)
		summaryMetric := summary.NewMetricFrom(info, getMetricValues(sink, testDuration))

		switch {
		case isSkippedMetric(summaryMode, info.Name):
			// Do nothing, just skip.
		case isHTTPMetric(info.Name):
			dest.HTTP[info.Name] = summaryMetric
		case isExecutionMetric(info.Name):
			dest.Execution[info.Name] = summaryMetric
		case isNetworkMetric(info.Name):
			dest.Network[info.Name] = summaryMetric
		case isBrowserMetric(info.Name):
			dest.Browser[info.Name] = summaryMetric
		case isGrpcMetric(info.Name):
			dest.Grpc[info.Name] = summaryMetric
		case isWebSocketsMetric(info.Name):
			dest.WebSocket[info.Name] = summaryMetric
		case isWebVitalsMetric(info.Name):
			dest.WebVitals[info.Name] = summaryMetric
		default:
			dest.Custom[info.Name] = summaryMetric
		}
	}

	for _, metricData := range groupData.aggregatedMetrics {
		storeMetric(
			summaryGroup.Metrics,
			metricData.MetricInfo,
			metricData.Sink,
			testRunDuration,
			summaryTrendStats,
		)
	}

	// We also set the groups order, so it's preserved from code.
	summaryGroup.GroupsOrder = groupData.groupsOrder

	// Finally, we keep moving down the hierarchy and populate the nested groups.
	for groupName, subGroupData := range groupData.groupsData {
		summarySubGroup := summary.NewGroup()
		populateSummaryGroup(summaryMode, &summarySubGroup, subGroupData, testRunDuration, summaryTrendStats)
		summaryGroup.Groups[groupName] = summarySubGroup
	}
}

func summaryThresholds(
	thresholds thresholds,
	testRunDuration time.Duration,
	summaryTrendStats []string,
) summary.Thresholds {
	getMetricValues := metricValueGetter(summaryTrendStats)

	rts := make(map[string]summary.MetricThresholds, len(thresholds))
	for mName, mThresholds := range thresholds {
		mt, exists := rts[mName]
		if !exists {
			mt = summary.MetricThresholds{
				Metric: summary.NewMetricFrom(
					mThresholds.MetricInfo,
					getMetricValues(mThresholds.Sink, testRunDuration),
				),
				Thresholds: make([]summary.Threshold, 0, len(mThresholds.tt)),
			}
		}

		for _, threshold := range mThresholds.tt {
			mt.Thresholds = append(mt.Thresholds, summary.Threshold{
				Source: threshold.Source,
				Ok:     !threshold.LastFailed,
			})

			// Additionally, if metric is a trend and the threshold source is a percentile,
			// we may need to add the percentile value to the metric values, in case it
			// isn't one of [summaryTrendStats].
			if trendSink, isTrend := mThresholds.Sink.(*metrics.TrendSink); isTrend {
				if agg, percentile, isPercentile := extractPercentileThresholdSource(threshold.Source); isPercentile {
					mt.Metric.Values[agg] = trendSink.P(percentile / 100)
				}
			}
		}

		rts[mName] = mt
	}

	return rts
}

func populateSummaryChecks(
	summaryGroup *summary.Group,
	groupData *aggregatedGroupData,
	testRunDuration time.Duration,
	summaryTrendStats []string,
) {
	getMetricValues := metricValueGetter(summaryTrendStats)

	checksMetric, exists := groupData.aggregatedMetrics[metrics.ChecksName]
	if !exists {
		return
	}

	summaryGroup.Checks = summary.NewChecks()

	totalChecks := float64(checksMetric.Sink.(*metrics.RateSink).Total)   //nolint:forcetypeassert
	successChecks := float64(checksMetric.Sink.(*metrics.RateSink).Trues) //nolint:forcetypeassert

	summaryGroup.Checks.Metrics.Total.Values["count"] = totalChecks
	summaryGroup.Checks.Metrics.Total.Values["rate"] = calculateRate(totalChecks, testRunDuration)

	summaryGroup.Checks.Metrics.Success = summary.NewMetricFrom(
		summary.MetricInfo{
			Name:     "checks_succeeded",
			Type:     checksMetric.Type,
			Contains: checksMetric.Contains,
		},
		getMetricValues(checksMetric.Sink, testRunDuration),
	)

	summaryGroup.Checks.Metrics.Fail.Values["passes"] = totalChecks - successChecks
	summaryGroup.Checks.Metrics.Fail.Values["fails"] = successChecks
	summaryGroup.Checks.Metrics.Fail.Values["rate"] = (totalChecks - successChecks) / totalChecks

	summaryGroup.Checks.OrderedChecks = groupData.checks.orderedChecks
}

func isHTTPMetric(metricName string) bool {
	return oneOfMetrics(metricName,
		metrics.HTTPReqsName,
		metrics.HTTPReqFailedName,
		metrics.HTTPReqDurationName,
		metrics.HTTPReqBlockedName,
		metrics.HTTPReqConnectingName,
		metrics.HTTPReqTLSHandshakingName,
		metrics.HTTPReqSendingName,
		metrics.HTTPReqWaitingName,
		metrics.HTTPReqReceivingName,
	)
}

func isExecutionMetric(metricName string) bool {
	return oneOfMetrics(metricName, metrics.VUsName,
		metrics.VUsMaxName,
		metrics.IterationsName,
		metrics.IterationDurationName,
		metrics.DroppedIterationsName,
	)
}

func isNetworkMetric(metricName string) bool {
	return oneOfMetrics(metricName, metrics.DataSentName, metrics.DataReceivedName)
}

func isBrowserMetric(metricName string) bool {
	return strings.HasPrefix(metricName, "browser_") && !isWebVitalsMetric(metricName)
}

func isWebVitalsMetric(metricName string) bool {
	return strings.HasPrefix(metricName, "browser_web_vital_")
}

func isGrpcMetric(metricName string) bool {
	return strings.HasPrefix(metricName, "grpc_")
}

func isWebSocketsMetric(metricName string) bool {
	return strings.HasPrefix(metricName, "ws_")
}

func isSkippedMetric(summaryMode summary.Mode, metricName string) bool {
	switch summaryMode {
	case summary.ModeCompact:
		return oneOfMetrics(metricName,
			metrics.ChecksName, metrics.GroupDurationName,
			metrics.HTTPReqBlockedName, metrics.HTTPReqConnectingName, metrics.HTTPReqReceivingName,
			metrics.HTTPReqSendingName, metrics.HTTPReqTLSHandshakingName, metrics.HTTPReqWaitingName,
		)
	default:
		return oneOfMetrics(metricName,
			metrics.ChecksName, metrics.GroupDurationName,
		)
	}
}

func oneOfMetrics(metricName string, values ...string) bool {
	for _, v := range values {
		if strings.HasPrefix(metricName, v) {
			return true
		}
	}
	return false
}

func calculateRate(count float64, duration time.Duration) float64 {
	if duration == 0 {
		return 0
	}
	return count / (float64(duration) / float64(time.Second))
}

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

var percentileThresholdSourceRe = regexp.MustCompile(`^p\((\d+(?:\.\d+)?)\)\s*([<>=])`)

// TODO(@joanlopez): Evaluate whether we should expose the parsed expression from the threshold.
// For now, and until we formally decide how do we want the `metrics` package to look like, we do "manually"
// parse the threshold expression here in order to extract, if existing, the percentile aggregation.
func extractPercentileThresholdSource(source string) (agg string, percentile float64, isPercentile bool) {
	// We capture the following three matches, in order to detect whether source is a percentile:
	// 1. The percentile definition: p(...)
	// 2. The percentile value: p(??)
	// 3. The beginning of the operator: '<', '>', or '='
	const expectedMatches = 3
	matches := percentileThresholdSourceRe.FindStringSubmatch(strings.TrimSpace(source))

	if len(matches) == expectedMatches {
		var err error
		percentile, err = strconv.ParseFloat(matches[1], 64)
		if err != nil {
			return "", 0, false
		}

		agg = "p(" + matches[1] + ")"
		isPercentile = true
		return
	}

	return "", 0, false
}
