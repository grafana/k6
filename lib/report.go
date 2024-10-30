package lib

import (
	"go.k6.io/k6/metrics"

	"time"
)

type ReportMetricData struct {
	Type     string
	Contains string
	Values   map[string]float64
}

func NewReportMetricsDataFrom(
	mType metrics.MetricType, vType metrics.ValueType, sink metrics.Sink,
	testDuration time.Duration, summaryTrendStats []string,
) ReportMetricData {
	// TODO: we obtain this from [options.SummaryTrendStats] which is a string slice
	getMetricValues := metricValueGetter(summaryTrendStats)

	return ReportMetricData{
		Type:     mType.String(),
		Contains: vType.String(),
		Values:   getMetricValues(sink, testDuration),
	}
}

type ReportMetrics struct {
	// HTTP contains report data specific to HTTP metrics and is used
	// to produce the summary HTTP subsection's content.
	HTTP map[string]ReportMetricData
	// Execution contains report data specific to Execution metrics and is used
	// to produce the summary Execution subsection's content.
	Execution map[string]ReportMetricData
	// Network contains report data specific to Network metrics and is used
	// to produce the summary Network subsection's content.
	Network map[string]ReportMetricData

	Browser map[string]ReportMetricData

	WebVitals map[string]ReportMetricData

	Grpc map[string]ReportMetricData

	WebSocket map[string]ReportMetricData `js:"websocket"`

	// Miscellaneous contains user-defined metric results as well as extensions metrics
	Miscellaneous map[string]ReportMetricData
}

func NewReportMetrics() ReportMetrics {
	return ReportMetrics{
		HTTP:          make(map[string]ReportMetricData),
		Execution:     make(map[string]ReportMetricData),
		Network:       make(map[string]ReportMetricData),
		Browser:       make(map[string]ReportMetricData),
		WebVitals:     make(map[string]ReportMetricData),
		Grpc:          make(map[string]ReportMetricData),
		WebSocket:     make(map[string]ReportMetricData),
		Miscellaneous: make(map[string]ReportMetricData),
	}
}

type ReportChecksMetrics struct {
	Total   ReportMetricData `js:"checks_total"`
	Success ReportMetricData `js:"checks_succeeded"`
	Fail    ReportMetricData `js:"checks_failed"`
}

type ReportChecks struct {
	Metrics       ReportChecksMetrics
	OrderedChecks []*Check
}

type ReportGroup struct {
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
	Checks ReportChecks

	ReportGroup
	Scenarios map[string]ReportGroup
}

func NewReport() Report {
	initMetricData := func(t metrics.MetricType) ReportMetricData {
		return ReportMetricData{
			Type:     t.String(),
			Contains: metrics.Default.String(),
			Values:   make(map[string]float64),
		}
	}

	return Report{
		ReportGroup: ReportGroup{
			Metrics: NewReportMetrics(),
			Groups:  make(map[string]ReportGroup),
		},
		Checks: ReportChecks{
			Metrics: ReportChecksMetrics{
				Total:   initMetricData(metrics.Counter),
				Success: initMetricData(metrics.Rate),
				Fail:    initMetricData(metrics.Rate),
			},
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
