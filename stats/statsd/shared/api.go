package statsd

import (
	"time"

	"github.com/loadimpact/k6/stats"
)

// Sample defines a sample type
type Sample struct {
	Type   stats.MetricType `json:"type"`
	Metric string           `json:"metric"`
	Data   SampleData       `json:"data"`
	Extra  ExtraData        `json:"extra_data"`
}

// SampleData defines a data sample type
type SampleData struct {
	Time  time.Time         `json:"time"`
	Value float64           `json:"value"`
	Tags  map[string]string `json:"tags,omitempty"`
}

// ExtraData defines extra sample data
type ExtraData struct {
	Raw       *stats.Metric `json:"raw_metric,omitempty"`
	Threshold Threshold     `json:"threshold,omitempty"`
	Group     string        `json:"group,omitempty"`
	Check     string        `json:"check,omitempty"`
}
