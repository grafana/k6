package summary

import (
	"fmt"
	"strings"
	"time"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"

	"github.com/sirupsen/logrus"
)

const flushPeriod = 200 * time.Millisecond // TODO: make this configurable

var _ output.Output = &Output{}

// Output ...
type Output struct {
	output.SampleBuffer

	periodicFlusher *output.PeriodicFlusher
	logger          logrus.FieldLogger

	dataModel DataModel

	// FIXME: drop me
	startTime time.Time
}

// New returns a new JSON output.
func New(params output.Params) (*Output, error) {
	return &Output{
		logger: params.Logger.WithFields(logrus.Fields{
			"output": "summary",
		}),
		dataModel: NewDataModel(),
	}, nil
}

func (o *Output) Description() string {
	return ""
}

func (o *Output) Start() error {
	pf, err := output.NewPeriodicFlusher(flushPeriod, o.flushMetrics)
	if err != nil {
		return err
	}
	o.logger.Debug("Started!")
	o.periodicFlusher = pf

	//FIXME: drop me
	o.startTime = time.Now()

	return nil
}

func (o *Output) Stop() error {
	o.periodicFlusher.Stop()

	for groupName, aggregatedData := range o.dataModel.Groups {
		o.logger.Warning(groupName)

		for metricName, sink := range aggregatedData {
			o.logger.Warning(fmt.Sprintf("  %s: %+v", metricName, sink))
		}
	}
	return nil
}

type MetricData struct {
	container map[string]*metrics.Metric
}

type ScenarioData struct {
	MetricData

	// FIXME: Groups could have groups
	Groups map[string]AggregatedMetricData
}

type DataModel struct {
	ScenarioData

	Scenarios map[string]AggregatedMetricData
}

type AggregatedMetric struct {
	Metric *metrics.Metric
	Sink   metrics.Sink
}

func NewAggregatedMetric(metric *metrics.Metric) AggregatedMetric {
	return AggregatedMetric{
		Metric: metric,
		Sink:   metrics.NewSink(metric.Type),
	}
}

type AggregatedMetricData map[string]AggregatedMetric

func (a AggregatedMetricData) AddSample(sample metrics.Sample) {
	if _, exists := a[sample.Metric.Name]; !exists {
		a[sample.Metric.Name] = NewAggregatedMetric(sample.Metric)
	}

	a[sample.Metric.Name].Sink.Add(sample)
}

func NewDataModel() DataModel {
	return DataModel{
		ScenarioData: ScenarioData{
			MetricData: MetricData{
				container: make(map[string]*metrics.Metric),
			},
			Groups: make(map[string]AggregatedMetricData),
		},
		Scenarios: make(map[string]AggregatedMetricData),
	}
}

func (d DataModel) GroupStored(groupName string) bool {
	_, exists := d.Groups[groupName]
	return exists
}

func (d DataModel) ScenarioStored(scenarioName string) bool {
	_, exists := d.Scenarios[scenarioName]
	return exists
}

func (o *Output) flushMetrics() {
	samples := o.GetBufferedSamples()
	for _, sc := range samples {
		samples := sc.GetSamples()
		for _, sample := range samples {
			if _, ok := o.dataModel.container[sample.Metric.Name]; !ok {
				o.dataModel.container[sample.Metric.Name] = sample.Metric
			}

			if groupName, exists := sample.Tags.Get("group"); exists && len(groupName) > 0 {
				normalizedGroupName := strings.TrimPrefix(groupName, "::")

				if !o.dataModel.GroupStored(normalizedGroupName) {
					o.dataModel.Groups[normalizedGroupName] = make(AggregatedMetricData)
				}

				o.dataModel.Groups[normalizedGroupName].AddSample(sample)
			}

			if scenarioName, exists := sample.Tags.Get("scenario"); exists {
				if !o.dataModel.ScenarioStored(scenarioName) {
					o.dataModel.Scenarios[scenarioName] = make(AggregatedMetricData)
				}

				o.dataModel.Scenarios[scenarioName].AddSample(sample)
			}

		}
	}
}

func (o *Output) MetricsReport(summary *lib.Summary, options lib.Options) lib.Report {
	report := lib.NewReport()

	storeMetric := func(dest lib.ReportMetrics, m *metrics.Metric, sink metrics.Sink, testDuration time.Duration, summaryTrendStats []string) {
		switch {
		case isSkippedMetric(m.Name):
			// Do nothing, just skip.
		case isHTTPMetric(m.Name):
			dest.HTTP[m.Name] = lib.NewReportMetricsDataFrom(m.Type, m.Contains, m.Sink, summary.TestRunDuration, options.SummaryTrendStats)
		case isExecutionMetric(m.Name):
			dest.Execution[m.Name] = lib.NewReportMetricsDataFrom(m.Type, m.Contains, m.Sink, summary.TestRunDuration, options.SummaryTrendStats)
		case isNetworkMetric(m.Name):
			dest.Network[m.Name] = lib.NewReportMetricsDataFrom(m.Type, m.Contains, m.Sink, summary.TestRunDuration, options.SummaryTrendStats)
		case isBrowserMetric(m.Name):
			dest.Browser[m.Name] = lib.NewReportMetricsDataFrom(m.Type, m.Contains, m.Sink, summary.TestRunDuration, options.SummaryTrendStats)
		case isGrpcMetric(m.Name):
			dest.Grpc[m.Name] = lib.NewReportMetricsDataFrom(m.Type, m.Contains, m.Sink, summary.TestRunDuration, options.SummaryTrendStats)
		case isWebSocketsMetric(m.Name):
			dest.WebSocket[m.Name] = lib.NewReportMetricsDataFrom(m.Type, m.Contains, m.Sink, summary.TestRunDuration, options.SummaryTrendStats)
		case isWebVitalsMetric(m.Name):
			dest.WebVitals[m.Name] = lib.NewReportMetricsDataFrom(m.Type, m.Contains, m.Sink, summary.TestRunDuration, options.SummaryTrendStats)
		default:
			dest.Miscellaneous[m.Name] = lib.NewReportMetricsDataFrom(m.Type, m.Contains, m.Sink, summary.TestRunDuration, options.SummaryTrendStats)
		}
	}

	for _, m := range summary.Metrics {
		storeMetric(report.Metrics, m, m.Sink, summary.TestRunDuration, options.SummaryTrendStats)
	}

	totalChecks := float64(summary.Metrics[metrics.ChecksName].Sink.(*metrics.RateSink).Total)
	successChecks := float64(summary.Metrics[metrics.ChecksName].Sink.(*metrics.RateSink).Trues)

	report.Checks.Metrics.Total.Values["count"] = totalChecks // Counter metric with total checks
	report.Checks.Metrics.Total.Values["rate"] = calculateCounterRate(totalChecks, summary.TestRunDuration)

	checksMetric := summary.Metrics[metrics.ChecksName]
	report.Checks.Metrics.Success = lib.NewReportMetricsDataFrom(checksMetric.Type, checksMetric.Contains, checksMetric.Sink, summary.TestRunDuration, options.SummaryTrendStats) // Rate metric with successes (equivalent to the 'checks' metric)

	report.Checks.Metrics.Fail.Values["passes"] = totalChecks - successChecks
	report.Checks.Metrics.Fail.Values["fails"] = successChecks
	report.Checks.Metrics.Fail.Values["rate"] = (totalChecks - successChecks) / totalChecks

	report.Checks.OrderedChecks = summary.RootGroup.OrderedChecks

	for groupName, aggregatedData := range o.dataModel.Groups {
		report.Groups[groupName] = lib.NewReportMetrics()

		for _, metricData := range aggregatedData {
			storeMetric(
				report.Groups[groupName],
				metricData.Metric,
				metricData.Sink,
				summary.TestRunDuration,
				options.SummaryTrendStats,
			)
		}
	}

	for scenarioName, aggregatedData := range o.dataModel.Scenarios {
		report.Scenarios[scenarioName] = lib.NewReportMetrics()

		for _, metricData := range aggregatedData {
			storeMetric(
				report.Scenarios[scenarioName],
				metricData.Metric,
				metricData.Sink,
				summary.TestRunDuration,
				options.SummaryTrendStats,
			)
		}
	}

	return report
}
