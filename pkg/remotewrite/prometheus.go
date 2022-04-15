package remotewrite

import (
	"fmt"
	"math"
	"sort"

	"github.com/prometheus/prometheus/pkg/timestamp"
	"github.com/prometheus/prometheus/prompb"
	"go.k6.io/k6/metrics"
)

type PrometheusMapping struct{}

func (pm *PrometheusMapping) MapCounter(ms *metricsStorage, sample metrics.Sample, labels []prompb.Label) []prompb.TimeSeries {
	sample = ms.update(sample, nil)
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

func (pm *PrometheusMapping) MapGauge(ms *metricsStorage, sample metrics.Sample, labels []prompb.Label) []prompb.TimeSeries {
	sample = ms.update(sample, nil)
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

func (pm *PrometheusMapping) MapRate(ms *metricsStorage, sample metrics.Sample, labels []prompb.Label) []prompb.TimeSeries {
	sample = ms.update(sample, nil)
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

func (pm *PrometheusMapping) MapTrend(ms *metricsStorage, sample metrics.Sample, labels []prompb.Label) []prompb.TimeSeries {
	sample = ms.update(sample, trendAdd)

	s := sample.Metric.Sink.(*metrics.TrendSink)
	aggr := map[string]float64{
		"min":   s.Min,
		"max":   s.Max,
		"avg":   s.Avg,
		"med":   s.Med,
		"p(90)": p(s, 0.90),
		"p(95)": p(s, 0.95),
	}

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

// The following functions are an attempt to add ad-hoc optimization to TrendSink,
// and are a partial copy-paste from k6/metrics.
// TODO: re-write & refactor this once metrics refactoring progresses in k6.

func trendAdd(current, s metrics.Sample) metrics.Sample {
	t := current.Metric.Sink.(*metrics.TrendSink)

	// insert into sorted array instead of sorting anew on each addition
	index := sort.Search(len(t.Values), func(i int) bool {
		return t.Values[i] > s.Value
	})
	t.Values = append(t.Values, 0)
	copy(t.Values[index+1:], t.Values[index:])
	t.Values[index] = s.Value

	t.Count += 1
	t.Sum += s.Value
	t.Avg = t.Sum / float64(t.Count)

	if s.Value > t.Max {
		t.Max = s.Value
	}
	if s.Value < t.Min || t.Count == 1 {
		t.Min = s.Value
	}

	if (t.Count & 0x01) == 0 {
		t.Med = (t.Values[(t.Count/2)-1] + t.Values[(t.Count/2)]) / 2
	} else {
		t.Med = t.Values[t.Count/2]
	}

	current.Metric.Sink = t
	return current
}

func p(t *metrics.TrendSink, pct float64) float64 {
	switch t.Count {
	case 0:
		return 0
	case 1:
		return t.Values[0]
	default:
		// If percentile falls on a value in Values slice, we return that value.
		// If percentile does not fall on a value in Values slice, we calculate (linear interpolation)
		// the value that would fall at percentile, given the values above and below that percentile.
		i := pct * (float64(t.Count) - 1.0)
		j := t.Values[int(math.Floor(i))]
		k := t.Values[int(math.Ceil(i))]
		f := i - math.Floor(i)
		return j + (k-j)*f
	}
}
