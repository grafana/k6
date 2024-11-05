package summary

import (
	"strings"
	"sync/atomic"
	"time"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

type dataModel struct {
	aggregatedGroupData
	scenarios map[string]aggregatedGroupData
}

// storeSample differs from addSample in that it stores the metric and the metric sink from the sample,
// while addSample updates the internally stored metric sink with the sample, which differs from the
// original metric sink.
func (d dataModel) storeSample(sample metrics.Sample) {
	d.metrics.storeSample(sample)

	if checkName, hasCheckTag := sample.Tags.Get(metrics.TagCheck.String()); hasCheckTag && sample.Metric.Name == metrics.ChecksName {
		check := d.checks.checkFor(checkName)
		if sample.Value == 0 {
			atomic.AddInt64(&check.Fails, 1)
		} else {
			atomic.AddInt64(&check.Passes, 1)
		}
	}
}

func newDataModel() dataModel {
	return dataModel{
		aggregatedGroupData: newAggregatedGroupData(),
		scenarios:           make(map[string]aggregatedGroupData),
	}
}

func (d dataModel) groupDataFor(scenario string) aggregatedGroupData {
	if groupData, exists := d.scenarios[scenario]; exists {
		return groupData
	}
	d.scenarios[scenario] = newAggregatedGroupData()
	return d.scenarios[scenario]
}

type aggregatedGroupData struct {
	checks     *aggregatedChecksData
	metrics    aggregatedMetricData
	groupsData map[string]aggregatedGroupData
}

func newAggregatedGroupData() aggregatedGroupData {
	return aggregatedGroupData{
		checks:     newAggregatedChecksData(),
		metrics:    make(map[string]aggregatedMetric),
		groupsData: make(map[string]aggregatedGroupData),
	}
}

func (a aggregatedGroupData) groupDataFor(group string) aggregatedGroupData {
	if groupData, exists := a.groupsData[group]; exists {
		return groupData
	}
	a.groupsData[group] = newAggregatedGroupData()
	return a.groupsData[group]
}

// addSample differs from storeSample in that it updates the internally stored metric sink with the sample,
// which differs from the original metric sink, while storeSample stores the metric and the metric sink from
// the sample.
func (a aggregatedGroupData) addSample(sample metrics.Sample) {
	a.metrics.addSample(sample)

	if checkName, hasCheckTag := sample.Tags.Get(metrics.TagCheck.String()); hasCheckTag && sample.Metric.Name == metrics.ChecksName {
		check := a.checks.checkFor(checkName)
		if sample.Value == 0 {
			atomic.AddInt64(&check.Fails, 1)
		} else {
			atomic.AddInt64(&check.Passes, 1)
		}
	}
}

type aggregatedMetricData map[string]aggregatedMetric

func (a aggregatedMetricData) addSample(sample metrics.Sample) {
	if _, exists := a[sample.Metric.Name]; !exists {
		a[sample.Metric.Name] = newAggregatedMetric(sample.Metric)
	}

	a[sample.Metric.Name].Sink.Add(sample)
}

func (a aggregatedMetricData) storeSample(sample metrics.Sample) {
	if _, exists := a[sample.Metric.Name]; !exists {
		a[sample.Metric.Name] = aggregatedMetric{
			Metric: sample.Metric,
			Sink:   sample.Metric.Sink,
		}
	}
}

type aggregatedMetric struct {
	Metric *metrics.Metric
	Sink   metrics.Sink
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

func populateReportGroup(
	reportGroup *lib.ReportGroup,
	groupData aggregatedGroupData,
	testRunDuration time.Duration,
	summaryTrendStats []string,
) {
	// First, we populate the checks metrics, which are treated independently.
	populateReportChecks(reportGroup, groupData, testRunDuration, summaryTrendStats)

	storeMetric := func(dest lib.ReportMetrics, info lib.ReportMetricInfo, sink metrics.Sink, testDuration time.Duration, summaryTrendStats []string) {
		reportMetric := lib.NewReportMetricFrom(info, sink, testDuration, summaryTrendStats)

		switch {
		case isSkippedMetric(info.Name):
			// Do nothing, just skip.
		case isHTTPMetric(info.Name):
			dest.HTTP[info.Name] = reportMetric
		case isExecutionMetric(info.Name):
			dest.Execution[info.Name] = reportMetric
		case isNetworkMetric(info.Name):
			dest.Network[info.Name] = reportMetric
		case isBrowserMetric(info.Name):
			dest.Browser[info.Name] = reportMetric
		case isGrpcMetric(info.Name):
			dest.Grpc[info.Name] = reportMetric
		case isWebSocketsMetric(info.Name):
			dest.WebSocket[info.Name] = reportMetric
		case isWebVitalsMetric(info.Name):
			dest.WebVitals[info.Name] = reportMetric
		default:
			dest.Miscellaneous[info.Name] = reportMetric
		}
	}

	for _, metricData := range groupData.metrics {
		storeMetric(
			reportGroup.Metrics,
			lib.ReportMetricInfo{
				Name:     metricData.Metric.Name,
				Type:     metricData.Metric.Type.String(),
				Contains: metricData.Metric.Contains.String(),
			},
			metricData.Sink,
			testRunDuration,
			summaryTrendStats,
		)
	}

	for groupName, subGroupData := range groupData.groupsData {
		subReportGroup := lib.NewReportGroup()
		populateReportGroup(&subReportGroup, subGroupData, testRunDuration, summaryTrendStats)
		reportGroup.Groups[groupName] = subReportGroup
	}
}

// FIXME: This function is a bit flurry, we should consider refactoring it.
// For instance, it would be possible to directly construct these metrics on-the-fly.
func populateReportChecks(
	reportGroup *lib.ReportGroup,
	groupData aggregatedGroupData,
	testRunDuration time.Duration,
	summaryTrendStats []string,
) {
	checksMetric, exists := groupData.metrics[metrics.ChecksName]
	if !exists {
		return
	}

	reportGroup.Checks = lib.NewReportChecks()

	totalChecks := float64(checksMetric.Sink.(*metrics.RateSink).Total)
	successChecks := float64(checksMetric.Sink.(*metrics.RateSink).Trues)

	reportGroup.Checks.Metrics.Total.Values["count"] = totalChecks
	reportGroup.Checks.Metrics.Total.Values["rate"] = calculateCounterRate(totalChecks, testRunDuration)

	reportGroup.Checks.Metrics.Success = lib.NewReportMetricFrom(
		lib.ReportMetricInfo{
			Name:     "checks_succeeded",
			Type:     checksMetric.Metric.Type.String(),
			Contains: checksMetric.Metric.Contains.String(),
		},
		checksMetric.Sink,
		testRunDuration,
		summaryTrendStats,
	)

	reportGroup.Checks.Metrics.Fail.Values["passes"] = totalChecks - successChecks
	reportGroup.Checks.Metrics.Fail.Values["fails"] = successChecks
	reportGroup.Checks.Metrics.Fail.Values["rate"] = (totalChecks - successChecks) / totalChecks

	reportGroup.Checks.OrderedChecks = groupData.checks.orderedChecks
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

func isSkippedMetric(metricName string) bool {
	return oneOfMetrics(metricName, metrics.ChecksName, metrics.GroupDurationName)
}

func oneOfMetrics(metricName string, values ...string) bool {
	for _, v := range values {
		if strings.HasPrefix(metricName, v) {
			return true
		}
	}
	return false
}

func calculateCounterRate(count float64, duration time.Duration) float64 {
	if duration == 0 {
		return 0
	}
	return count / (float64(duration) / float64(time.Second))
}
