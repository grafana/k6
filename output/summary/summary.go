package summary

import (
	"fmt"
	"strings"
	"sync/atomic"
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

	dataModel dataModel
}

// New returns a new JSON output.
func New(params output.Params) (*Output, error) {
	return &Output{
		logger: params.Logger.WithFields(logrus.Fields{
			"output": "summary",
		}),
		dataModel: newDataModel(),
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
	return nil
}

func (o *Output) Stop() error {
	o.periodicFlusher.Stop()

	for groupName, aggregatedData := range o.dataModel.groupsData {
		o.logger.Warning(groupName)

		for metricName, sink := range aggregatedData.aggregatedMetrics {
			o.logger.Warning(fmt.Sprintf("  %s: %+v", metricName, sink))
		}
	}

	return nil
}

func (o *Output) flushMetrics() {
	samples := o.GetBufferedSamples()
	for _, sc := range samples {
		samples := sc.GetSamples()
		for _, sample := range samples {
			o.flushSample(sample)
		}
	}
}

func (o *Output) flushSample(sample metrics.Sample) {
	// First, we store the sample data into the metrics stored at the k6 metrics registry level.
	o.storeSample(sample)

	hasThresholds := func(metric *metrics.Metric) bool {
		return metric.Thresholds.Thresholds != nil && len(metric.Thresholds.Thresholds) > 0
	}

	printThresholds := func(metric *metrics.Metric) {
		for _, threshold := range metric.Thresholds.Thresholds {
			fmt.Printf("Metric=%s, Threshold=%+v\n", metric.Name, threshold)
		}
	}

	if hasThresholds(sample.Metric) {
		printThresholds(sample.Metric)
	}

	for _, submetric := range sample.Metric.Submetrics {
		if hasThresholds(submetric.Metric) {
			printThresholds(submetric.Metric)
		}
	}

	// Then, we'll proceed to store the sample data into each group
	// metrics. However, we need to determine whether the groups tree
	// is within a scenario or not.
	groupData := o.dataModel.aggregatedGroupData
	if scenarioName, hasScenario := sample.Tags.Get("scenario"); hasScenario {
		groupData = o.dataModel.groupDataFor(scenarioName)
		groupData.addSample(sample)
	}

	if groupTag, exists := sample.Tags.Get("group"); exists && len(groupTag) > 0 {
		normalizedGroupName := strings.TrimPrefix(groupTag, "::")
		groupNames := strings.Split(normalizedGroupName, "::")

		for i, groupName := range groupNames {
			groupData.groupDataFor(groupName)
			groupData.groupsData[groupName].addSample(sample)

			if i < len(groupNames)-1 {
				groupData = groupData.groupsData[groupName]
			}
		}
	}
}

func (o *Output) MetricsReport(summary *lib.Summary, options lib.Options) lib.Report {
	report := lib.NewReport()

	testRunDuration := summary.TestRunDuration
	summaryTrendStats := options.SummaryTrendStats

	// Populate root group and nested groups recursively.
	populateReportGroup(
		&report.ReportGroup,
		o.dataModel.aggregatedGroupData,
		testRunDuration,
		summaryTrendStats,
	)

	// Populate scenario groups and nested groups recursively.
	for scenarioName, scenarioData := range o.dataModel.scenarios {
		scenarioReportGroup := lib.NewReportGroup()
		populateReportGroup(
			&scenarioReportGroup,
			scenarioData,
			testRunDuration,
			summaryTrendStats,
		)
		report.Scenarios[scenarioName] = scenarioReportGroup
	}

	return report
}

// storeSample relays the sample to the k6 metrics registry relevant metric.
//
// If it's a check-specific metric, it will also update the check's pass/fail counters.
func (o *Output) storeSample(sample metrics.Sample) {
	o.dataModel.aggregatedMetrics.relayMetricFrom(sample)

	if checkName, hasCheckTag := sample.Tags.Get(metrics.TagCheck.String()); hasCheckTag && sample.Metric.Name == metrics.ChecksName {
		check := o.dataModel.checks.checkFor(checkName)
		if sample.Value == 0 {
			atomic.AddInt64(&check.Fails, 1)
		} else {
			atomic.AddInt64(&check.Passes, 1)
		}
	}
}
