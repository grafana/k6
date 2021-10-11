package remotewrite

import (
	"fmt"
	"sync"

	"github.com/prometheus/prometheus/prompb"
	"go.k6.io/k6/stats"
)

// Note: k6 Registry is not used here since Output is getting
// samples only from k6 engine, hence we assume they are already vetted.
type metricsStorage struct {
	m    map[string]stats.Sample
	lock sync.RWMutex
}

func newMetricsStorage() *metricsStorage {
	return &metricsStorage{
		m:    make(map[string]stats.Sample),
		lock: sync.RWMutex{},
	}
}

// update modifies metricsStorage and returns updated sample
// so that they hold the same value for the given metric
func (ms *metricsStorage) update(sample stats.Sample) stats.Sample {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	if current, ok := ms.m[sample.Metric.Name]; ok {
		current.Metric.Sink.Add(sample)
		current.Time = sample.Time // to avoid duplicates in timestamps
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

// TODO: add dummy RawMapping
