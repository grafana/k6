package prometheus

import (
	"fmt"

	"github.com/prometheus/prometheus/prompb"
	"go.k6.io/k6/stats"
)

// Remote write endpoint accepts TimeSeries structure defined in gRPC. It must:
// a) contain Labels array
// b) have a __name__ label: without it, metric might be unquerable or even rejected
// as a metric without a name. This behaviour depends on underlying storage used.
// c) not have duplicate timestamps within 1 timeseries, see https://github.com/prometheus/prometheus/issues/9210
// Prometheus write handler processes only some fields as of now, so here we'll add only them.

type timeSeries []prompb.TimeSeries

func newTimeSeries() timeSeries {
	return make([]prompb.TimeSeries, 0)
}

func (ts *timeSeries) addSample(sample *stats.Sample, labelPairs []prompb.Label) error {
	var newts []prompb.TimeSeries

	switch sample.Metric.Type {
	case stats.Counter:
		newts = getCounter(sample, labelPairs)

	case stats.Gauge:
		newts = getGauge(sample, labelPairs)

	case stats.Rate:
		newts = getHistogram(sample, labelPairs)

	case stats.Trend:
		// TODO temporary skipping
		return nil

	default:
		return fmt.Errorf("Something is really off as I cannot recognize the type of metric %s: `%s`", sample.Metric.Name, sample.Metric.Type)
	}

	*ts = append(*ts, newts...)

	return nil
}

func tagsToPrometheusLabels(tags *stats.SampleTags) ([]prompb.Label, error) {
	tagsMap := tags.CloneTags()
	labelPairs := make([]prompb.Label, 0, len(tagsMap))

	for name, value := range tagsMap {
		if len(name) < 1 || len(value) < 1 {
			continue
		}
		// TODO add checks:
		// - reserved underscore
		// - sorting
		// - duplicates?

		labelPairs = append(labelPairs, prompb.Label{
			Name:  name,
			Value: value,
		})
	}

	return labelPairs, nil
}
