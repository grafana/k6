package remotewrite

import (
	"fmt"
	"time"

	"github.com/prometheus/prometheus/pkg/timestamp"
	"github.com/prometheus/prometheus/prompb"
	"go.k6.io/k6/metrics"
)

func MapTagSet(t *metrics.SampleTags) []prompb.Label {
	tags := t.CloneTags()

	labels := make([]prompb.Label, 0, len(tags))
	for k, v := range tags {
		labels = append(labels, prompb.Label{Name: k, Value: v})
	}
	return labels
}

func MapSeries(ts TimeSeries) prompb.TimeSeries {
	return prompb.TimeSeries{
		Labels: append(MapTagSet(ts.Tags), prompb.Label{
			Name:  "__name__",
			Value: fmt.Sprintf("%s%s", defaultMetricPrefix, ts.Metric.Name),
		}),
	}
}

func MapTrend(series TimeSeries, t time.Time, sink *trendSink) []prompb.TimeSeries {
	// Prometheus metric system does not support Trend so this mapping will
	// store a counter for the number of reported values and gauges to keep
	// track of aggregated values. Also store a sum of the values to allow
	// the calculation of moving averages.
	// TODO: when Prometheus implements support for sparse histograms, re-visit this implementation

	labels := MapTagSet(series.Tags)
	timestamp := timestamp.FromTime(t)

	return []prompb.TimeSeries{
		{
			Labels: append(labels, prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_count", defaultMetricPrefix, series.Metric.Name),
			}),
			Samples: []prompb.Sample{
				{
					Value:     float64(sink.Count),
					Timestamp: timestamp,
				},
			},
		},
		{
			Labels: append(labels, prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_sum", defaultMetricPrefix, series.Metric.Name),
			}),
			Samples: []prompb.Sample{
				{
					Value:     sink.Sum,
					Timestamp: timestamp,
				},
			},
		},
		{
			Labels: append(labels, prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_min", defaultMetricPrefix, series.Metric.Name),
			}),
			Samples: []prompb.Sample{
				{
					Value:     sink.Min,
					Timestamp: timestamp,
				},
			},
		},
		{
			Labels: append(labels, prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_max", defaultMetricPrefix, series.Metric.Name),
			}),
			Samples: []prompb.Sample{
				{
					Value:     sink.Max,
					Timestamp: timestamp,
				},
			},
		},
		{
			Labels: append(labels, prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_avg", defaultMetricPrefix, series.Metric.Name),
			}),
			Samples: []prompb.Sample{
				{
					Value:     sink.Avg,
					Timestamp: timestamp,
				},
			},
		},
		{
			Labels: append(labels, prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_med", defaultMetricPrefix, series.Metric.Name),
			}),
			Samples: []prompb.Sample{
				{
					Value:     sink.Med,
					Timestamp: timestamp,
				},
			},
		},
		{
			Labels: append(labels, prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_p90", defaultMetricPrefix, series.Metric.Name),
			}),
			Samples: []prompb.Sample{
				{
					Value:     sink.P(0.9),
					Timestamp: timestamp,
				},
			},
		},
		{
			Labels: append(labels, prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_p95", defaultMetricPrefix, series.Metric.Name),
			}),
			Samples: []prompb.Sample{
				{
					Value:     sink.P(0.95),
					Timestamp: timestamp,
				},
			},
		},
	}
}
