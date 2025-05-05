package summary

import (
	"strings"
	"sync/atomic"
	"time"

	"go.k6.io/k6/internal/lib/consts"
	"go.k6.io/k6/internal/lib/summary"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"

	"github.com/sirupsen/logrus"
)

const flushPeriod = 200 * time.Millisecond // TODO: make this configurable

var _ output.Output = &Output{}

// Output implements the lib.Output interface for collecting metrics' data to be displayed in the end-of-test summary.
type Output struct {
	output.SampleBuffer

	periodicFlusher *output.PeriodicFlusher
	logger          logrus.FieldLogger

	dataModel   dataModel
	summaryMode summary.Mode
}

// New returns a new summary output.
func New(params output.Params) (*Output, error) {
	sm, err := summary.ValidateMode(params.RuntimeOptions.SummaryMode.String)
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

// OutputName is the name of the output.
const OutputName = "summary"

// Description returns a human-readable description of the output.
func (o *Output) Description() string {
	return OutputName
}

// Start starts a new output.PeriodicFlusher to collect and flush metrics that will be
// rendered in the end-of-test summary.
func (o *Output) Start() error {
	pf, err := output.NewPeriodicFlusher(flushPeriod, o.flushMetrics)
	if err != nil {
		return err
	}
	o.logger.Debug("Started!")
	o.periodicFlusher = pf
	return nil
}

// Stop flushes any remaining metrics and stops the goroutine.
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

	skipGroupSamples := o.summaryMode == summary.ModeCompact || o.summaryMode == summary.ModeLegacy
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
		if normalizedGroupName == consts.SetupFn || normalizedGroupName == consts.TeardownFn {
			return // Essentially skip samples tagged with group: setup/teardown.
		}
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

// Summary returns a lib.Summary of the test run.
func (o *Output) Summary(
	testRunDuration time.Duration,
	observedMetrics map[string]*metrics.Metric,
	options lib.Options,
) *summary.Summary {
	s := summary.New()
	s.TestRunDuration = testRunDuration

	summaryTrendStats := options.SummaryTrendStats

	// Process the observed metrics. This is necessary to ensure that we have collected
	// all metrics, even those that have no samples, so that we can render them in the summary.
	o.processObservedMetrics(observedMetrics)

	// Populate the thresholds.
	s.Thresholds = summaryThresholds(o.dataModel.thresholds, testRunDuration, summaryTrendStats)

	// Populate root group and nested groups recursively.
	populateSummaryGroup(
		o.summaryMode,
		&s.Group,
		o.dataModel.aggregatedGroupData,
		testRunDuration,
		summaryTrendStats,
	)

	// Populate scenario groups and nested groups recursively.
	for scenarioName, scenarioData := range o.dataModel.scenarios {
		scenarioSummaryGroup := summary.NewGroup()
		populateSummaryGroup(
			o.summaryMode,
			&scenarioSummaryGroup,
			scenarioData,
			testRunDuration,
			summaryTrendStats,
		)
		s.Scenarios[scenarioName] = scenarioSummaryGroup
	}

	return s
}

// storeSample relays the sample to the k6 metrics registry relevant metric.
//
// If it's a check-specific metric, it will also update the check's pass/fail counters.
func (o *Output) storeSample(sample metrics.Sample) {
	// If it's the first time we see this metric, we relay the metric from the sample
	// and, we store the thresholds for that particular metric, and its sub-metrics.
	if _, exists := o.dataModel.aggregatedMetrics[sample.Metric.Name]; !exists {
		o.dataModel.aggregatedMetrics[sample.Metric.Name] = relayAggregatedMetricFrom(sample.Metric)

		o.dataModel.storeThresholdsFor(sample.Metric)
		for _, sub := range sample.Metric.Submetrics {
			o.dataModel.storeThresholdsFor(sub.Metric)
		}
	}

	checkName, hasCheckTag := sample.Tags.Get(metrics.TagCheck.String())
	if hasCheckTag && sample.Metric.Name == metrics.ChecksName {
		check := o.dataModel.checks.checkFor(checkName)
		if sample.Value == 0 {
			atomic.AddInt64(&check.Fails, 1)
		} else {
			atomic.AddInt64(&check.Passes, 1)
		}
	}
}

// processObservedMetrics is responsible for ensuring that we have collected
// all metrics, even those that have no samples, so that we can render them in the summary.
func (o *Output) processObservedMetrics(observedMetrics map[string]*metrics.Metric) {
	for _, m := range observedMetrics {
		if _, exists := o.dataModel.aggregatedMetrics[m.Name]; !exists {
			o.dataModel.aggregatedMetrics[m.Name] = relayAggregatedMetricFrom(m)
			o.dataModel.storeThresholdsFor(m)
		}
	}
}
