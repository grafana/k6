package summary

import (
	"fmt"
	"go.k6.io/k6/metrics"
	"strings"
	"time"

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
func New(params output.Params) (output.Output, error) {
	return &Output{
		logger: params.Logger.WithFields(logrus.Fields{
			"output":   "summary",
			"filename": params.ConfigArgument,
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

	Groups map[string]AggregatedMetricData
}

type DataModel struct {
	ScenarioData

	Scenarios map[string]AggregatedMetricData
}

type AggregatedMetricData map[string]metrics.Sink

func (a AggregatedMetricData) AddSample(sample metrics.Sample) {
	if _, exists := a[sample.Metric.Name]; !exists {
		a[sample.Metric.Name] = metrics.NewSink(sample.Metric.Type)
	}

	a[sample.Metric.Name].Add(sample)
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

			if groupName, exists := sample.Tags.Get("group"); exists {
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
