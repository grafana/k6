package summary

import (
	"strings"
	"sync/atomic"
	"time"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

type dataModel struct {
	thresholds
	aggregatedGroupData
	scenarios map[string]aggregatedGroupData
}

func newDataModel() dataModel {
	return dataModel{
		aggregatedGroupData: newAggregatedGroupData(),
		scenarios:           make(map[string]aggregatedGroupData),
	}
}

func (d *dataModel) groupDataFor(scenario string) aggregatedGroupData {
	if groupData, exists := d.scenarios[scenario]; exists {
		return groupData
	}
	d.scenarios[scenario] = newAggregatedGroupData()
	return d.scenarios[scenario]
}

func (d *dataModel) storeThresholdsFor(m *metrics.Metric) {
	for _, threshold := range m.Thresholds.Thresholds {
		d.thresholds = append(d.thresholds, struct {
			*metrics.Threshold
			Metric *metrics.Metric
		}{Metric: m, Threshold: threshold})
	}
}

type thresholds []struct {
	*metrics.Threshold
	Metric *metrics.Metric
}

type aggregatedGroupData struct {
	checks            *aggregatedChecksData
	aggregatedMetrics aggregatedMetricData
	groupsData        map[string]aggregatedGroupData
}

func newAggregatedGroupData() aggregatedGroupData {
	return aggregatedGroupData{
		checks:            newAggregatedChecksData(),
		aggregatedMetrics: make(map[string]aggregatedMetric),
		groupsData:        make(map[string]aggregatedGroupData),
	}
}

func (a aggregatedGroupData) groupDataFor(group string) aggregatedGroupData {
	if groupData, exists := a.groupsData[group]; exists {
		return groupData
	}
	a.groupsData[group] = newAggregatedGroupData()
	return a.groupsData[group]
}

// addSample differs from relayMetricFrom in that it updates the internally stored metric sink with the sample,
// which differs from the original metric sink, while relayMetricFrom stores the metric and the metric sink from
// the sample.
func (a aggregatedGroupData) addSample(sample metrics.Sample) {
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

// relayMetricFrom stores the metric and the metric sink from the sample. It makes the underlying metric of our
// summary's aggregatedMetricData point directly to a metric in the k6 registry, and relies on that specific pointed
// at metrics internal state for its computations.
func (a aggregatedMetricData) relayMetricFrom(sample metrics.Sample) {
	a[sample.Metric.Name] = aggregatedMetric{
		Metric: sample.Metric,
		Sink:   sample.Metric.Sink,
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

// FIXME (@joan): rename this to make it explicit this is different from an actual k6 metric, and this is used
// only to keep an aggregated view of specific metric-check-group-scenario-thresholds set of values.
type aggregatedMetric struct {
	// FIXME (@joan): Drop this and replace it with a concrete copy of the metric data we want to track
	// to avoid any potential confusion.
	Metric *metrics.Metric

	// FIXME (@joan): Introduce our own way of tracking thresholds, and whether they're crossed or not.
	// Without relying on the internal submetrics the engine maintains specifically for thresholds.
	// Thresholds []OurThreshold // { crossed: boolean }

	Sink metrics.Sink
}

func newAggregatedMetric(metric *metrics.Metric) aggregatedMetric {
	return aggregatedMetric{
		Metric: metric,
		Sink:   metrics.NewSink(metric.Type),
	}
}

type aggregatedChecksData struct {
	checks        map[string]*lib.Check
	orderedChecks []*lib.Check
}

func newAggregatedChecksData() *aggregatedChecksData {
	return &aggregatedChecksData{
		checks:        make(map[string]*lib.Check),
		orderedChecks: make([]*lib.Check, 0),
	}
}

func (a *aggregatedChecksData) checkFor(name string) *lib.Check {
	check, ok := a.checks[name]
	if !ok {
		var err error
		check, err = lib.NewCheck(name, &lib.Group{}) // FIXME: Do we really need the group?
		if err != nil {
			panic(err) // This should never happen
		}
		a.checks[name] = check
		a.orderedChecks = append(a.orderedChecks, check)
	}
	return check
}

func populateSummaryGroup(
	summaryMode lib.SummaryMode,
	summaryGroup *lib.SummaryGroup,
	groupData aggregatedGroupData,
	testRunDuration time.Duration,
	summaryTrendStats []string,
) {
	// First, we populate the checks metrics, which are treated independently.
	populateSummaryChecks(summaryGroup, groupData, testRunDuration, summaryTrendStats)

	// Then, we store the metrics.
	storeMetric := func(
		dest lib.SummaryMetrics,
		info lib.SummaryMetricInfo,
		sink metrics.Sink,
		testDuration time.Duration,
		summaryTrendStats []string,
	) {
		summaryMetric := lib.NewSummaryMetricFrom(info, sink, testDuration, summaryTrendStats)

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
			lib.SummaryMetricInfo{
				Name:     metricData.Metric.Name,
				Type:     metricData.Metric.Type.String(),
				Contains: metricData.Metric.Contains.String(),
			},
			metricData.Sink,
			testRunDuration,
			summaryTrendStats,
		)
	}

	// Finally, we keep moving down the hierarchy and populate the nested groups.
	for groupName, subGroupData := range groupData.groupsData {
		summarySubGroup := lib.NewSummaryGroup()
		populateSummaryGroup(summaryMode, &summarySubGroup, subGroupData, testRunDuration, summaryTrendStats)
		summaryGroup.Groups[groupName] = summarySubGroup
	}
}

func summaryThresholds(
	thresholds thresholds,
	testRunDuration time.Duration,
	summaryTrendStats []string,
) lib.SummaryThresholds {
	rts := make(map[string]lib.MetricThresholds, len(thresholds))
	for _, threshold := range thresholds {
		metric := threshold.Metric

		mt, exists := rts[metric.Name]
		if !exists {
			mt = lib.MetricThresholds{
				Metric: lib.NewSummaryMetricFrom(
					lib.SummaryMetricInfo{
						Name:     metric.Name,
						Type:     metric.Type.String(),
						Contains: metric.Contains.String(),
					},
					metric.Sink,
					testRunDuration,
					summaryTrendStats,
				),
			}
		}

		mt.Thresholds = append(mt.Thresholds, lib.SummaryThreshold{
			Source: threshold.Source,
			Ok:     !threshold.LastFailed,
		})
		rts[metric.Name] = mt
	}
	return rts
}

// FIXME: This function is a bit flurry, we should consider refactoring it.
// For instance, it would be possible to directly construct these metrics on-the-fly.
func populateSummaryChecks(
	summaryGroup *lib.SummaryGroup,
	groupData aggregatedGroupData,
	testRunDuration time.Duration,
	summaryTrendStats []string,
) {
	checksMetric, exists := groupData.aggregatedMetrics[metrics.ChecksName]
	if !exists {
		return
	}

	summaryGroup.Checks = lib.NewSummaryChecks()

	totalChecks := float64(checksMetric.Sink.(*metrics.RateSink).Total)   //nolint:forcetypeassert
	successChecks := float64(checksMetric.Sink.(*metrics.RateSink).Trues) //nolint:forcetypeassert

	summaryGroup.Checks.Metrics.Total.Values["count"] = totalChecks
	summaryGroup.Checks.Metrics.Total.Values["rate"] = calculateRate(totalChecks, testRunDuration)

	summaryGroup.Checks.Metrics.Success = lib.NewSummaryMetricFrom(
		lib.SummaryMetricInfo{
			Name:     "checks_succeeded",
			Type:     checksMetric.Metric.Type.String(),
			Contains: checksMetric.Metric.Contains.String(),
		},
		checksMetric.Sink,
		testRunDuration,
		summaryTrendStats,
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

func isSkippedMetric(summaryMode lib.SummaryMode, metricName string) bool {
	switch summaryMode {
	case lib.SummaryModeCompact:
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
