package protocol

import "sync"

// MetricMap is a simple storage for name-indexed counter metrics.
type MetricMap struct {
	metrics map[string]uint
	mutex   sync.RWMutex
}

// NewMetricMap returns a MetricMap with the specified metrics initialized to zero.
func NewMetricMap(metrics ...string) *MetricMap {
	mm := &MetricMap{
		metrics: map[string]uint{},
	}

	for _, metric := range metrics {
		mm.metrics[metric] = 0
	}

	return mm
}

// Inc increases the value of the specified counter by one. If the metric hasn't been initialized or incremented before,
// it is set to 1.
func (m *MetricMap) Inc(name string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.metrics[name]++
}

// Map returns a map of the counters indexed by name. The returned map is a copy of the internal storage.
func (m *MetricMap) Map() map[string]uint {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	out := make(map[string]uint, len(m.metrics))
	for k, v := range m.metrics {
		out[k] = v
	}

	return out
}
