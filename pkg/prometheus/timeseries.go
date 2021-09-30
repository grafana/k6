package prometheus

import (
	"github.com/prometheus/prometheus/pkg/timestamp"
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

func (ts *timeSeries) addSamples(samples []stats.Sample) {
	// Prometheus remote write treats each label array in TimeSeries as the same
	// for all Samples in those TimeSeries (https://github.com/prometheus/prometheus/blob/03d084f8629477907cab39fc3d314b375eeac010/storage/remote/write_handler.go#L75).
	// But K6 metrics can have different tags per each Sample so in order not to
	// lose info in tags or assign tags wrongly, let's store each Sample in a different TimeSeries, for now.
	// This approach also allows to avoid hard to replicate issues with duplicate timestamps.

	for _, sample := range samples {
		labelPairs := tagsToLabels(sample.Tags)
		labelPairs = append(labelPairs, prompb.Label{
			Name: "__name__",
			// we cannot use name tag as it can be absent or equal to URL in HTTP testing
			Value: sample.Metric.Name,
		})

		*ts = append(*ts, prompb.TimeSeries{
			Labels: labelPairs,
			Samples: []prompb.Sample{
				{
					Value:     sample.Value,
					Timestamp: timestamp.FromTime(sample.Time),
				},
			},
		})
	}
}

func tagsToLabels(tags *stats.SampleTags) []prompb.Label {
	tagsMap := tags.CloneTags()
	labelPairs := make([]prompb.Label, len(tagsMap))
	i := 0
	for name, value := range tagsMap {
		labelPairs[i].Name = name
		labelPairs[i].Value = value
		i++
	}

	return labelPairs
}
