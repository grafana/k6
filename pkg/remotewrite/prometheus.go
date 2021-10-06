package remotewrite

import (
	"fmt"

	"github.com/prometheus/prometheus/pkg/timestamp"
	"github.com/prometheus/prometheus/prompb"
	"go.k6.io/k6/stats"
)

type PrometheusMapping struct {
	histograms map[string]struct{} // to quickly look up previously created histograms
}

func NewPrometheusMapping() *PrometheusMapping {
	return &PrometheusMapping{
		make(map[string]struct{}),
	}
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
	return []prompb.TimeSeries{}
}
