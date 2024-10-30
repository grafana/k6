package summary

import (
	"strings"
	"time"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

type dataModel struct {
	aggregatedGroupData
	scenarios map[string]aggregatedGroupData
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
	Metrics aggregatedMetricData
	Groups  map[string]aggregatedGroupData
}

func newAggregatedGroupData() aggregatedGroupData {
	return aggregatedGroupData{
		Metrics: make(map[string]aggregatedMetric),
		Groups:  make(map[string]aggregatedGroupData),
	}
}

func (d aggregatedGroupData) groupDataFor(group string) aggregatedGroupData {
	if groupData, exists := d.Groups[group]; exists {
		return groupData
	}
	d.Groups[group] = newAggregatedGroupData()
	return d.Groups[group]
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

func populateReportChecks(report *lib.Report, summary *lib.Summary, options lib.Options) {
	totalChecks := float64(summary.Metrics[metrics.ChecksName].Sink.(*metrics.RateSink).Total)
	successChecks := float64(summary.Metrics[metrics.ChecksName].Sink.(*metrics.RateSink).Trues)

	report.Checks.Metrics.Total.Values["count"] = totalChecks
	report.Checks.Metrics.Total.Values["rate"] = calculateCounterRate(totalChecks, summary.TestRunDuration)

	checksMetric := summary.Metrics[metrics.ChecksName]
	report.Checks.Metrics.Success = lib.NewReportMetricFrom(
		lib.ReportMetricInfo{
			Name:     "checks_succeeded",
			Type:     checksMetric.Type.String(),
			Contains: checksMetric.Contains.String(),
		},
		checksMetric.Sink,
		summary.TestRunDuration,
		options.SummaryTrendStats,
	)

	report.Checks.Metrics.Fail.Values["passes"] = totalChecks - successChecks
	report.Checks.Metrics.Fail.Values["fails"] = successChecks
	report.Checks.Metrics.Fail.Values["rate"] = (totalChecks - successChecks) / totalChecks

	report.Checks.OrderedChecks = summary.RootGroup.OrderedChecks
}

func populateReportGroup(reportGroup *lib.ReportGroup, groupData aggregatedGroupData, summary *lib.Summary, options lib.Options) {
	storeMetric := func(dest lib.ReportMetrics, info lib.ReportMetricInfo, sink metrics.Sink, testDuration time.Duration, summaryTrendStats []string) {
		switch {
		case isSkippedMetric(info.Name):
			// Do nothing, just skip.
		case isHTTPMetric(info.Name):
			dest.HTTP[info.Name] = lib.NewReportMetricFrom(info, sink, summary.TestRunDuration, options.SummaryTrendStats)
		case isExecutionMetric(info.Name):
			dest.Execution[info.Name] = lib.NewReportMetricFrom(info, sink, summary.TestRunDuration, options.SummaryTrendStats)
		case isNetworkMetric(info.Name):
			dest.Network[info.Name] = lib.NewReportMetricFrom(info, sink, summary.TestRunDuration, options.SummaryTrendStats)
		case isBrowserMetric(info.Name):
			dest.Browser[info.Name] = lib.NewReportMetricFrom(info, sink, summary.TestRunDuration, options.SummaryTrendStats)
		case isGrpcMetric(info.Name):
			dest.Grpc[info.Name] = lib.NewReportMetricFrom(info, sink, summary.TestRunDuration, options.SummaryTrendStats)
		case isWebSocketsMetric(info.Name):
			dest.WebSocket[info.Name] = lib.NewReportMetricFrom(info, sink, summary.TestRunDuration, options.SummaryTrendStats)
		case isWebVitalsMetric(info.Name):
			dest.WebVitals[info.Name] = lib.NewReportMetricFrom(info, sink, summary.TestRunDuration, options.SummaryTrendStats)
		default:
			dest.Miscellaneous[info.Name] = lib.NewReportMetricFrom(info, sink, summary.TestRunDuration, options.SummaryTrendStats)
		}
	}

	for _, metricData := range groupData.Metrics {
		storeMetric(
			reportGroup.Metrics,
			lib.ReportMetricInfo{
				Name:     metricData.Metric.Name,
				Type:     metricData.Metric.Type.String(),
				Contains: metricData.Metric.Contains.String(),
			},
			metricData.Sink,
			summary.TestRunDuration,
			options.SummaryTrendStats,
		)
	}

	for groupName, subGroupData := range groupData.Groups {
		subReportGroup := lib.NewReportGroup()
		populateReportGroup(&subReportGroup, subGroupData, summary, options)
		reportGroup.Groups[groupName] = subReportGroup
	}
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
