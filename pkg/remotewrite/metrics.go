package remotewrite

import (
	"fmt"

	"github.com/prometheus/prometheus/pkg/timestamp"
	"github.com/prometheus/prometheus/prompb"
	"go.k6.io/k6/metrics"
)

// Note: k6 Registry is not used here since Output is getting
// samples only from k6 engine, hence we assume they are already vetted.

// metricsStorage is an in-memory gather point for metrics
type metricsStorage struct {
	m map[string]*metrics.Metric
}

func newMetricsStorage() *metricsStorage {
	return &metricsStorage{
		m: make(map[string]*metrics.Metric),
	}
}

// update modifies metricsStorage and returns updated sample
// so that the stored metric and the returned metric hold the same value
func (ms *metricsStorage) update(sample metrics.Sample, add func(*metrics.Metric, metrics.Sample)) *metrics.Metric {
	m, ok := ms.m[sample.Metric.Name]
	if !ok {
		var sink metrics.Sink
		switch sample.Metric.Type {
		case metrics.Counter:
			sink = &metrics.CounterSink{}
		case metrics.Gauge:
			sink = &metrics.GaugeSink{}
		case metrics.Trend:
			sink = &metrics.TrendSink{}
		case metrics.Rate:
			sink = &metrics.RateSink{}
		default:
			panic("the Metric Type is not supported")
		}

		m = &metrics.Metric{
			Name:     sample.Metric.Name,
			Type:     sample.Metric.Type,
			Contains: sample.Metric.Contains,
			Sink:     sink,
		}

		ms.m[m.Name] = m
	}

	// TODO: https://github.com/grafana/xk6-output-prometheus-remote/issues/11
	//
	// Sometimes remote write endpoint throws an error about duplicates even if the values
	// sent were different. By current observations, this is a hard to repeat case and
	// potentially a bug.
	// Related: https://github.com/prometheus/prometheus/issues/9210

	// TODO: Trend is the unique type that benefits from this logic.
	// so this logic can be removed just creating
	// a new implementation in this extension
	// for TrendSink and its Add method.
	if add == nil {
		m.Sink.Add(sample)
	} else {
		add(m, sample)
	}

	return m
}

// transform k6 sample into TimeSeries for remote-write
func (ms *metricsStorage) transform(mapping Mapping, sample metrics.Sample, labels []prompb.Label) ([]prompb.TimeSeries, error) {
	var newts []prompb.TimeSeries

	switch sample.Metric.Type {
	case metrics.Counter:
		newts = mapping.MapCounter(ms, sample, labels)

	case metrics.Gauge:
		newts = mapping.MapGauge(ms, sample, labels)

	case metrics.Rate:
		newts = mapping.MapRate(ms, sample, labels)

	case metrics.Trend:
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
	MapCounter(ms *metricsStorage, sample metrics.Sample, labels []prompb.Label) []prompb.TimeSeries
	MapGauge(ms *metricsStorage, sample metrics.Sample, labels []prompb.Label) []prompb.TimeSeries
	MapRate(ms *metricsStorage, sample metrics.Sample, labels []prompb.Label) []prompb.TimeSeries
	MapTrend(ms *metricsStorage, sample metrics.Sample, labels []prompb.Label) []prompb.TimeSeries

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

func (rm *RawMapping) MapCounter(ms *metricsStorage, sample metrics.Sample, labels []prompb.Label) []prompb.TimeSeries {
	return rm.processSample(sample, labels)
}

func (rm *RawMapping) MapGauge(ms *metricsStorage, sample metrics.Sample, labels []prompb.Label) []prompb.TimeSeries {
	return rm.processSample(sample, labels)
}

func (rm *RawMapping) MapRate(ms *metricsStorage, sample metrics.Sample, labels []prompb.Label) []prompb.TimeSeries {
	return rm.processSample(sample, labels)
}

func (rm *RawMapping) MapTrend(ms *metricsStorage, sample metrics.Sample, labels []prompb.Label) []prompb.TimeSeries {
	return rm.processSample(sample, labels)
}

func (rm *RawMapping) processSample(sample metrics.Sample, labels []prompb.Label) []prompb.TimeSeries {
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
