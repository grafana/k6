package v1

import (
	"bytes"
	"encoding/json"
	"time"

	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/metrics"
)

// NullMetricType a nullable metric struct
type NullMetricType struct {
	Type  metrics.MetricType
	Valid bool
}

// MarshalJSON implements the json.Marshaler interface.
func (t NullMetricType) MarshalJSON() ([]byte, error) {
	if !t.Valid {
		return []byte("null"), nil
	}
	return t.Type.MarshalJSON()
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (t *NullMetricType) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		t.Valid = false
		return nil
	}
	t.Valid = true
	return json.Unmarshal(data, &t.Type)
}

// NullValueType a nullable metric value struct
type NullValueType struct {
	Type  metrics.ValueType
	Valid bool
}

// MarshalJSON implements the json.Marshaler interface.
func (t NullValueType) MarshalJSON() ([]byte, error) {
	if !t.Valid {
		return []byte("null"), nil
	}
	return t.Type.MarshalJSON()
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (t *NullValueType) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		t.Valid = false
		return nil
	}
	t.Valid = true
	return json.Unmarshal(data, &t.Type)
}

// Metric represents a metric that is being collected by k6.
type Metric struct {
	Name string `json:"-" yaml:"name"`

	Type     NullMetricType `json:"type" yaml:"type"`
	Contains NullValueType  `json:"contains" yaml:"contains"`
	Tainted  null.Bool      `json:"tainted" yaml:"tainted"`

	Sample map[string]float64 `json:"sample" yaml:"sample"`
}

// NewMetric constructs a new v1.Metric struct that is used for
// a metric representation in a k6 REST API.
// summaryTrendStats selects the trend columns; empty uses TrendSink.Format defaults.
func NewMetric(m *metrics.Metric, t time.Duration, summaryTrendStats []string) Metric {
	return Metric{
		Name:     m.Name,
		Type:     NullMetricType{m.Type, true},
		Contains: NullValueType{m.Contains, true},
		Tainted:  m.Tainted,
		Sample:   formatSample(m.Sink, t, summaryTrendStats),
	}
}

func formatSample(sink metrics.Sink, t time.Duration, summaryTrendStats []string) map[string]float64 {
	ts, ok := sink.(*metrics.TrendSink)
	if !ok || len(summaryTrendStats) == 0 {
		return sink.Format(t)
	}
	resolvers, err := metrics.GetResolversForTrendColumns(summaryTrendStats)
	if err != nil {
		return sink.Format(t)
	}
	sample := make(map[string]float64, len(summaryTrendStats))
	for _, col := range summaryTrendStats {
		sample[col] = resolvers[col](ts)
	}
	return sample
}
