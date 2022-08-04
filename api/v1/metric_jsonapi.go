package v1

import (
	"time"

	"go.k6.io/k6/metrics"
)

// MetricsJSONAPI is JSON API envelop for metrics
type MetricsJSONAPI struct {
	Data []metricData `json:"data"`
}

type metricJSONAPI struct {
	Data metricData `json:"data"`
}

type metricData struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Attributes Metric `json:"attributes"`
}

func newMetricEnvelope(m *metrics.Metric, t time.Duration) metricJSONAPI {
	return metricJSONAPI{
		Data: newMetricData(m, t),
	}
}

func newMetricsJSONAPI(list map[string]*metrics.Metric, t time.Duration) MetricsJSONAPI {
	metrics := make([]metricData, 0, len(list))

	for _, m := range list {
		metrics = append(metrics, newMetricData(m, t))
	}

	return MetricsJSONAPI{
		Data: metrics,
	}
}

func newMetricData(m *metrics.Metric, t time.Duration) metricData {
	metric := NewMetric(m, t)

	return metricData{
		Type:       "metrics",
		ID:         metric.Name,
		Attributes: metric,
	}
}

// Metrics extract the []v1.Metric from the JSON API envelop
func (m MetricsJSONAPI) Metrics() []Metric {
	list := make([]Metric, 0, len(m.Data))

	for _, metric := range m.Data {
		m := metric.Attributes
		m.Name = metric.ID
		list = append(list, m)
	}

	return list
}
