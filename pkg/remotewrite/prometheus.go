package remotewrite

import (
	"fmt"

	"github.com/prometheus/prometheus/pkg/timestamp"
	"github.com/prometheus/prometheus/prompb"
	"go.k6.io/k6/stats"
)

type PrometheusMapping struct{}

func NewPrometheusMapping() *PrometheusMapping {
	return &PrometheusMapping{}
}

func (pm *PrometheusMapping) MapCounter(ms *metricsStorage, sample stats.Sample, labels []prompb.Label) []prompb.TimeSeries {
	sample = ms.update(sample)
	aggr := sample.Metric.Sink.Format(0)

	return []prompb.TimeSeries{
		{
			Labels: append(labels, prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s", defaultMetricPrefix, sample.Metric.Name),
			}),
			Samples: []prompb.Sample{
				{
					Value:     aggr["count"],
					Timestamp: timestamp.FromTime(sample.Time),
				},
			},
		},
	}
}

func (pm *PrometheusMapping) MapGauge(ms *metricsStorage, sample stats.Sample, labels []prompb.Label) []prompb.TimeSeries {
	sample = ms.update(sample)
	aggr := sample.Metric.Sink.Format(0)

	return []prompb.TimeSeries{
		{
			Labels: append(labels, prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s", defaultMetricPrefix, sample.Metric.Name),
			}),
			Samples: []prompb.Sample{
				{
					Value:     aggr["value"],
					Timestamp: timestamp.FromTime(sample.Time),
				},
			},
		},
	}
}

func (pm *PrometheusMapping) MapRate(ms *metricsStorage, sample stats.Sample, labels []prompb.Label) []prompb.TimeSeries {
	sample = ms.update(sample)
	aggr := sample.Metric.Sink.Format(0)

	return []prompb.TimeSeries{
		{
			Labels: append(labels, prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s", defaultMetricPrefix, sample.Metric.Name),
			}),
			Samples: []prompb.Sample{
				{
					Value:     aggr["rate"],
					Timestamp: timestamp.FromTime(sample.Time),
				},
			},
		},
	}
}

func (pm *PrometheusMapping) MapTrend(ms *metricsStorage, sample stats.Sample, labels []prompb.Label) []prompb.TimeSeries {
	sample = ms.update(sample)
	aggr := sample.Metric.Sink.Format(0)

	// Prometheus metric system does not support Trend so this mapping will store gauges
	// to keep track of key values.
	// TODO: when Prometheus implements support for sparse histograms, re-visit this implementation

	return []prompb.TimeSeries{
		{
			Labels: append(labels, prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_min", defaultMetricPrefix, sample.Metric.Name),
			}),
			Samples: []prompb.Sample{
				{
					Value:     aggr["min"],
					Timestamp: timestamp.FromTime(sample.Time),
				},
			},
		},
		{
			Labels: append(labels, prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_max", defaultMetricPrefix, sample.Metric.Name),
			}),
			Samples: []prompb.Sample{
				{
					Value:     aggr["max"],
					Timestamp: timestamp.FromTime(sample.Time),
				},
			},
		},
		{
			Labels: append(labels, prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_avg", defaultMetricPrefix, sample.Metric.Name),
			}),
			Samples: []prompb.Sample{
				{
					Value:     aggr["avg"],
					Timestamp: timestamp.FromTime(sample.Time),
				},
			},
		},
		{
			Labels: append(labels, prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_med", defaultMetricPrefix, sample.Metric.Name),
			}),
			Samples: []prompb.Sample{
				{
					Value:     aggr["med"],
					Timestamp: timestamp.FromTime(sample.Time),
				},
			},
		},
		{
			Labels: append(labels, prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_p90", defaultMetricPrefix, sample.Metric.Name),
			}),
			Samples: []prompb.Sample{
				{
					Value:     aggr["p(90)"],
					Timestamp: timestamp.FromTime(sample.Time),
				},
			},
		},
		{
			Labels: append(labels, prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s_p95", defaultMetricPrefix, sample.Metric.Name),
			}),
			Samples: []prompb.Sample{
				{
					Value:     aggr["p(95)"],
					Timestamp: timestamp.FromTime(sample.Time),
				},
			},
		},
	}
}
