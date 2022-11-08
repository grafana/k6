package remotewrite

import (
	"fmt"
	"time"

	"github.com/mstoykov/atlas"
	prompb "go.buf.build/grpc/go/prometheus/prometheus"
	"go.k6.io/k6/metrics"
)

func MapTagSet(t *metrics.TagSet) []*prompb.Label {
	n := (*atlas.Node)(t)
	if n.Len() < 1 {
		return nil
	}
	labels := make([]*prompb.Label, 0, n.Len())
	for !n.IsRoot() {
		prev, key, value := n.Data()
		labels = append(labels, &prompb.Label{Name: key, Value: value})
		n = prev
	}
	return labels
}

func MapSeries(ts metrics.TimeSeries) prompb.TimeSeries {
	return prompb.TimeSeries{
		Labels: append(MapTagSet(ts.Tags), &prompb.Label{
			Name:  "__name__",
			Value: fmt.Sprintf("%s%s", defaultMetricPrefix, ts.Metric.Name),
		}),
	}
}

func MapTrend(series metrics.TimeSeries, t time.Time, sink *trendSink) []*prompb.TimeSeries {
	// Prometheus metric system does not support Trend so this mapping will
	// store a counter for the number of reported values and gauges to keep
	// track of aggregated values. Also store a sum of the values to allow
	// the calculation of moving averages.
	// TODO: when Prometheus implements support for sparse histograms, re-visit this implementation

	labels := MapTagSet(series.Tags)
	timestamp := t.UnixMilli()

	return []*prompb.TimeSeries{
		{
			Labels: append(labels, &prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_count", defaultMetricPrefix, series.Metric.Name),
			}),
			Samples: []*prompb.Sample{
				{
					Value:     float64(sink.Count),
					Timestamp: timestamp,
				},
			},
		},
		{
			Labels: append(labels, &prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_sum", defaultMetricPrefix, series.Metric.Name),
			}),
			Samples: []*prompb.Sample{
				{
					Value:     sink.Sum,
					Timestamp: timestamp,
				},
			},
		},
		{
			Labels: append(labels, &prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_min", defaultMetricPrefix, series.Metric.Name),
			}),
			Samples: []*prompb.Sample{
				{
					Value:     sink.Min,
					Timestamp: timestamp,
				},
			},
		},
		{
			Labels: append(labels, &prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_max", defaultMetricPrefix, series.Metric.Name),
			}),
			Samples: []*prompb.Sample{
				{
					Value:     sink.Max,
					Timestamp: timestamp,
				},
			},
		},
		{
			Labels: append(labels, &prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_avg", defaultMetricPrefix, series.Metric.Name),
			}),
			Samples: []*prompb.Sample{
				{
					Value:     sink.Avg,
					Timestamp: timestamp,
				},
			},
		},
		{
			Labels: append(labels, &prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_med", defaultMetricPrefix, series.Metric.Name),
			}),
			Samples: []*prompb.Sample{
				{
					Value:     sink.Med,
					Timestamp: timestamp,
				},
			},
		},
		{
			Labels: append(labels, &prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_p90", defaultMetricPrefix, series.Metric.Name),
			}),
			Samples: []*prompb.Sample{
				{
					Value:     sink.P(0.9),
					Timestamp: timestamp,
				},
			},
		},
		{
			Labels: append(labels, &prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_p95", defaultMetricPrefix, series.Metric.Name),
			}),
			Samples: []*prompb.Sample{
				{
					Value:     sink.P(0.95),
					Timestamp: timestamp,
				},
			},
		},
	}
}
