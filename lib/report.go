package lib

import (
	"go.k6.io/k6/metrics"

	"time"
)

type ReportMetricInfo struct {
	Name     string
	Type     string
	Contains string
}

type ReportMetric struct {
	ReportMetricInfo
	Values map[string]float64
}

func NewReportMetricFrom(
	info ReportMetricInfo, sink metrics.Sink,
	testDuration time.Duration, summaryTrendStats []string,
) ReportMetric {
	// TODO: we obtain this from [options.SummaryTrendStats] which is a string slice
	getMetricValues := metricValueGetter(summaryTrendStats)

	return ReportMetric{
		ReportMetricInfo: info,
		Values:           getMetricValues(sink, testDuration),
	}
}

type ReportMetrics struct {
	// HTTP contains report data specific to HTTP metrics and is used
	// to produce the summary HTTP subsection's content.
	HTTP map[string]ReportMetric
	// Execution contains report data specific to Execution metrics and is used
	// to produce the summary Execution subsection's content.
	Execution map[string]ReportMetric
	// Network contains report data specific to Network metrics and is used
	// to produce the summary Network subsection's content.
	Network map[string]ReportMetric

	Browser map[string]ReportMetric

	WebVitals map[string]ReportMetric

	Grpc map[string]ReportMetric

	WebSocket map[string]ReportMetric `js:"websocket"`

	// Miscellaneous contains user-defined metric results as well as extensions metrics
	Miscellaneous map[string]ReportMetric
}

func NewReportMetrics() ReportMetrics {
	return ReportMetrics{
		HTTP:          make(map[string]ReportMetric),
		Execution:     make(map[string]ReportMetric),
		Network:       make(map[string]ReportMetric),
		Browser:       make(map[string]ReportMetric),
		WebVitals:     make(map[string]ReportMetric),
		Grpc:          make(map[string]ReportMetric),
		WebSocket:     make(map[string]ReportMetric),
		Miscellaneous: make(map[string]ReportMetric),
	}
}

type ReportChecksMetrics struct {
	Total   ReportMetric `js:"checks_total"`
	Success ReportMetric `js:"checks_succeeded"`
	Fail    ReportMetric `js:"checks_failed"`
}

type ReportChecks struct {
	Metrics       ReportChecksMetrics
	OrderedChecks []*Check
}

func NewReportChecks() *ReportChecks {
	initChecksMetricData := func(name string, t metrics.MetricType) ReportMetric {
		return ReportMetric{
			ReportMetricInfo: ReportMetricInfo{
				Name:     name,
				Type:     t.String(),
				Contains: metrics.Default.String(),
			},
			Values: make(map[string]float64),
		}
	}

	return &ReportChecks{
		Metrics: ReportChecksMetrics{
			Total:   initChecksMetricData("checks_total", metrics.Counter),
			Success: initChecksMetricData("checks_succeeded", metrics.Rate),
			Fail:    initChecksMetricData("checks_failed", metrics.Rate),
		},
	}
}

type ReportThreshold struct {
	Source string       `js:"source"`
	Metric ReportMetric `js:"metric"`
	Ok     bool         `js:"ok"`
}

type ReportThresholds map[string][]*ReportThreshold

func NewReportThresholds() ReportThresholds {
	thresholds := make(ReportThresholds)
	return thresholds
}

type ReportGroup struct {
	Checks  *ReportChecks // Not always present, thus we use a pointer.
	Metrics ReportMetrics
	Groups  map[string]ReportGroup
}

func NewReportGroup() ReportGroup {
	return ReportGroup{
		Metrics: NewReportMetrics(),
		Groups:  make(map[string]ReportGroup),
	}
}

type Report struct {
	ReportThresholds
	ReportGroup
	Scenarios map[string]ReportGroup
}

func NewReport() Report {
	return Report{
		ReportThresholds: NewReportThresholds(),
		ReportGroup: ReportGroup{
			Metrics: NewReportMetrics(),
			Groups:  make(map[string]ReportGroup),
		},
		Scenarios: make(map[string]ReportGroup),
	}
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
			result["rate"] = calculateCounterRate(sink.Value, t)
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

func calculateCounterRate(count float64, duration time.Duration) float64 {
	if duration == 0 {
		return 0
	}
	return count / (float64(duration) / float64(time.Second))
}
