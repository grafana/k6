package json

import (
	"time"

	"go.k6.io/k6/metrics"
)

//go:generate easyjson -pkg -no_std_marshalers -gen_build_flags -mod=mod .

//easyjson:json
type sampleEnvelope struct {
	Metric string `json:"metric"`
	Type   string `json:"type"`
	Data   struct {
		Time     time.Time         `json:"time"`
		Value    float64           `json:"value"`
		Tags     *metrics.TagSet   `json:"tags"`
		Metadata map[string]string `json:"metadata,omitempty"`
	} `json:"data"`
}

// wrapSample is used to package a metric sample in a way that's nice to export
// to JSON and backwards-compatible.
func wrapSample(sample metrics.Sample) sampleEnvelope {
	s := sampleEnvelope{
		Type:   "Point",
		Metric: sample.Metric.Name,
	}
	s.Data.Time = sample.Time
	s.Data.Value = sample.Value
	s.Data.Tags = sample.Tags
	s.Data.Metadata = sample.Metadata
	return s
}

//easyjson:json
type metricEnvelope struct {
	Type string `json:"type"`
	Data struct {
		Name       string               `json:"name"`
		Type       metrics.MetricType   `json:"type"`
		Contains   metrics.ValueType    `json:"contains"`
		Thresholds metrics.Thresholds   `json:"thresholds"`
		Submetrics []*metrics.Submetric `json:"submetrics"`
	} `json:"data"`
	Metric string `json:"metric"`
}
