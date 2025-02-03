package summary

import (
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

	dataModel   dataModel
	summaryMode lib.SummaryMode
}

// New returns a new JSON output.
func New(params output.Params) (*Output, error) {
	sm, err := lib.ValidateSummaryMode(params.RuntimeOptions.SummaryMode.String)
	if err != nil {
		return nil, err
	}

	return &Output{
		logger: params.Logger.WithFields(logrus.Fields{
			"output": "summary",
		}),
		dataModel:   newDataModel(),
		summaryMode: sm,
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
	// First, the sample data is stored into the metrics stored at the k6 metrics registry level.
	o.storeSample(sample)

	skipGroupSamples := o.summaryMode == lib.SummaryModeCompact || o.summaryMode == lib.SummaryModeLegacy
	if skipGroupSamples {
		return
	}

	// Then, if the extended mode is enabled, the sample data is stored into each group metrics.
	// However, we need to determine whether the groups tree is within a scenario or not.
	groupData := o.dataModel.aggregatedGroupData
	if scenarioName, hasScenario := sample.Tags.Get("scenario"); hasScenario && scenarioName != "default" {
		groupData = o.dataModel.groupDataFor(scenarioName)
		groupData.addSample(sample)
	}

	if groupTag, exists := sample.Tags.Get("group"); exists && len(groupTag) > 0 {
		normalizedGroupName := strings.TrimPrefix(groupTag, lib.GroupSeparator)
		groupNames := strings.Split(normalizedGroupName, lib.GroupSeparator)

		// We traverse over all the groups to create a nested structure,
		// but we only add the sample to the group the sample belongs to,
		// cause by definition every group is independent.
		for _, groupName := range groupNames {
			groupData.groupDataFor(groupName)
			groupData = groupData.groupsData[groupName]
		}
		groupData.addSample(sample)
	}
}

func (o *Output) MetricsReport(summary *lib.Summary, options lib.Options) lib.Report {
	report := lib.NewReport()

	testRunDuration := summary.TestRunDuration
	summaryTrendStats := options.SummaryTrendStats

	// Populate the thresholds.
	report.ReportThresholds = reportThresholds(o.dataModel.thresholds, testRunDuration, summaryTrendStats)

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
	// If it's the first time we see this metric, we relay the metric from the sample
	// and, we store the thresholds for that particular metric, and its sub-metrics.
	if _, exists := o.dataModel.aggregatedMetrics[sample.Metric.Name]; !exists {
		o.dataModel.aggregatedMetrics.relayMetricFrom(sample)

		o.dataModel.storeThresholdsFor(sample.Metric)
		for _, sub := range sample.Metric.Submetrics {
			o.dataModel.storeThresholdsFor(sub.Metric)
		}
	}

	if checkName, hasCheckTag := sample.Tags.Get(metrics.TagCheck.String()); hasCheckTag && sample.Metric.Name == metrics.ChecksName {
		check := o.dataModel.checks.checkFor(checkName)
		if sample.Value == 0 {
			atomic.AddInt64(&check.Fails, 1)
		} else {
			atomic.AddInt64(&check.Passes, 1)
		}
	}
}
