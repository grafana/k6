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

	for groupName, aggregatedData := range o.dataModel.Groups {
		o.logger.Warning(groupName)

		for metricName, sink := range aggregatedData.Metrics {
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
			o.storeSample(sample)
		}
	}
}

func (o *Output) storeSample(sample metrics.Sample) {
	// First, we store the sample data into the global metrics.
	o.dataModel.Metrics.storeSample(sample)

	// Then, we'll proceed to store the sample data into each group
	// metrics. However, we need to determine whether the groups tree
	// is within a scenario or not.
	groupData := o.dataModel.aggregatedGroupData
	if scenarioName, hasScenario := sample.Tags.Get("scenario"); hasScenario {
		groupData = o.dataModel.groupDataFor(scenarioName)
		groupData.Metrics.addSample(sample)
	}

	if groupTag, exists := sample.Tags.Get("group"); exists && len(groupTag) > 0 {
		normalizedGroupName := strings.TrimPrefix(groupTag, "::")
		groupNames := strings.Split(normalizedGroupName, "::")

		for i, groupName := range groupNames {
			groupData.groupDataFor(groupName)
			groupData.Groups[groupName].Metrics.addSample(sample)

			if i < len(groupNames)-1 {
				groupData = groupData.Groups[groupName]
			}
		}
	}
}

func (o *Output) MetricsReport(summary *lib.Summary, options lib.Options) lib.Report {
	report := lib.NewReport()

	// Populate report checks.
	populateReportChecks(&report, summary, options)

	// Populate root group and nested groups recursively.
	populateReportGroup(&report.ReportGroup, o.dataModel.aggregatedGroupData, summary, options)

	// Populate scenario groups and nested groups recursively.
	for scenarioName, scenarioData := range o.dataModel.scenarios {
		scenarioReportGroup := lib.NewReportGroup()
		populateReportGroup(&scenarioReportGroup, scenarioData, summary, options)
		report.Scenarios[scenarioName] = scenarioReportGroup
	}

	return report
}
