package remotewrite

import (
	"fmt"

	"github.com/prometheus/prometheus/pkg/timestamp"
	"github.com/prometheus/prometheus/prompb"
	"go.k6.io/k6/stats"
)

// Note: k6 Registry is not used here since Output is getting
// samples only from k6 engine, hence we assume they are already vetted.

// metricsStorage is an in-memory gather point for metrics
type metricsStorage struct {
	m map[string]stats.Sample
}

func newMetricsStorage() *metricsStorage {
	return &metricsStorage{
		m: make(map[string]stats.Sample),
	}
}

// update modifies metricsStorage and returns updated sample
// so that the stored metric and the returned metric hold the same value
func (ms *metricsStorage) update(sample stats.Sample, add func(current, s stats.Sample) stats.Sample) stats.Sample {
	if current, ok := ms.m[sample.Metric.Name]; ok {
		if add == nil {
			current.Metric.Sink.Add(sample)
		} else {
			current = add(current, sample)
		}
		current.Time = sample.Time // to avoid duplicates in timestamps
		// Sometimes remote write endpoint throws an error about duplicates even if the values
		// sent were different. By current observations, this is a hard to repeat case and
		// potentially a bug.
		// Related: https://github.com/prometheus/prometheus/issues/9210

		ms.m[current.Metric.Name] = current
		return current
	} else {
		sample.Metric.Sink.Add(sample)
		ms.m[sample.Metric.Name] = sample
		return sample
	}
}

// transform k6 sample into TimeSeries for remote-write
func (ms *metricsStorage) transform(mapping Mapping, sample stats.Sample, labels []prompb.Label) ([]prompb.TimeSeries, error) {
	var newts []prompb.TimeSeries

	switch sample.Metric.Type {
	case stats.Counter:
		newts = mapping.MapCounter(ms, sample, labels)

	case stats.Gauge:
		newts = mapping.MapGauge(ms, sample, labels)

	case stats.Rate:
		newts = mapping.MapRate(ms, sample, labels)

	case stats.Trend:
		newts = mapping.MapTrend(ms, sample, labels)

	default:
		return nil, fmt.Errorf("Something is really off as I cannot recognize the type of metric %s: `%s`", sample.Metric.Name, sample.Metric.Type)
	}

	return newts, nil
}

// Mapping represents the specific way k6 metrics can be mapped to metrics of
// remote agent. As each remote agent can use different ways to store metrics as well as
// expect different values on remote write endpoint, they must have their own support.
type Mapping interface {
	MapCounter(ms *metricsStorage, sample stats.Sample, labels []prompb.Label) []prompb.TimeSeries
	MapGauge(ms *metricsStorage, sample stats.Sample, labels []prompb.Label) []prompb.TimeSeries
	MapRate(ms *metricsStorage, sample stats.Sample, labels []prompb.Label) []prompb.TimeSeries
	MapTrend(ms *metricsStorage, sample stats.Sample, labels []prompb.Label) []prompb.TimeSeries

	// AdjustLabels(labels []prompb.Label) []prompb.Label
}

func NewMapping(mapping string) Mapping {
	switch mapping {
	case "prometheus":
		return &PrometheusMapping{}
	default:
		return &RawMapping{}
	}
}

type RawMapping struct{}

func (rm *RawMapping) MapCounter(ms *metricsStorage, sample stats.Sample, labels []prompb.Label) []prompb.TimeSeries {
	return rm.processSample(sample, labels)
}

func (rm *RawMapping) MapGauge(ms *metricsStorage, sample stats.Sample, labels []prompb.Label) []prompb.TimeSeries {
	return rm.processSample(sample, labels)
}

func (rm *RawMapping) MapRate(ms *metricsStorage, sample stats.Sample, labels []prompb.Label) []prompb.TimeSeries {
	return rm.processSample(sample, labels)
}

func (rm *RawMapping) MapTrend(ms *metricsStorage, sample stats.Sample, labels []prompb.Label) []prompb.TimeSeries {
	return rm.processSample(sample, labels)
}

func (rm *RawMapping) processSample(sample stats.Sample, labels []prompb.Label) []prompb.TimeSeries {
	return []prompb.TimeSeries{
		{
			Labels: append(labels, prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("%s%s", defaultMetricPrefix, sample.Metric.Name),
			}),
			Samples: []prompb.Sample{
				{
					Value:     sample.Value,
					Timestamp: timestamp.FromTime(sample.Time),
				},
			},
		},
	}
}
