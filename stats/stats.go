package stats

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	counterString = `"counter"`
	gaugeString   = `"gauge"`
	trendString   = `"trend"`

	defaultString = `"default"`
	timeString    = `"time"`
)

// Possible values for MetricType.
const (
	NoType  = MetricType(iota) // No type; metrics like this are ignored
	Counter                    // A counter that sums its data points
	Gauge                      // A gauge that displays the latest value
	Trend                      // A trend, min/max/avg/med are interesting
)

// Possible values for ValueType.
const (
	Default = ValueType(iota) // Values are presented as-is
	Time                      // Values are timestamps (nanoseconds)
)

// The serialized metric type is invalid.
var ErrInvalidMetricType = errors.New("Invalid metric type")

// The serialized value type is invalid.
var ErrInvalidValueType = errors.New("Invalid value type")

// A MetricType specifies the type of a metric.
type MetricType int

// MarshalJSON serializes a MetricType as a human readable string.
func (t MetricType) MarshalJSON() ([]byte, error) {
	switch t {
	case Counter:
		return []byte(counterString), nil
	case Gauge:
		return []byte(gaugeString), nil
	case Trend:
		return []byte(trendString), nil
	default:
		return nil, ErrInvalidMetricType
	}
}

// UnmarshalJSON deserializes a MetricType from a string representation.
func (t *MetricType) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case counterString:
		*t = Counter
	case gaugeString:
		*t = Gauge
	case trendString:
		*t = Trend
	default:
		return ErrInvalidMetricType
	}

	return nil
}

func (t MetricType) String() string {
	switch t {
	case Counter:
		return counterString
	case Gauge:
		return gaugeString
	case Trend:
		return trendString
	default:
		return "[INVALID]"
	}
}

// The type of values a metric contains.
type ValueType int

// MarshalJSON serializes a ValueType as a human readable string.
func (t ValueType) MarshalJSON() ([]byte, error) {
	switch t {
	case Default:
		return []byte(defaultString), nil
	case Time:
		return []byte(timeString), nil
	default:
		return nil, ErrInvalidValueType
	}
}

// UnmarshalJSON deserializes a ValueType from a string representation.
func (t *ValueType) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case defaultString:
		*t = Default
	case timeString:
		*t = Time
	default:
		return ErrInvalidValueType
	}

	return nil
}

func (t ValueType) String() string {
	switch t {
	case Default:
		return defaultString
	case Time:
		return timeString
	default:
		return "[INVALID]"
	}
}

// A Sample is a single measurement.
type Sample struct {
	Metric *Metric
	Time   time.Time
	Tags   map[string]string
	Value  float64
}

// A Metric defines the shape of a set of data.
type Metric struct {
	Name     string     `json:"-"`
	Type     MetricType `json:"type"`
	Contains ValueType  `json:"contains"`

	// Filled in by the API when requested, the server side cannot count on its presence.
	Sample map[string]float64 `json:"sample"`
}

func New(name string, typ MetricType, t ...ValueType) *Metric {
	vt := Default
	if len(t) > 0 {
		vt = t[0]
	}
	return &Metric{Name: name, Type: typ, Contains: vt}
}

func (m Metric) Humanize() string {
	sample := m.Sample
	switch len(sample) {
	case 0:
		return ""
	case 1:
		for _, v := range sample {
			return m.HumanizeValue(v)
		}
		return ""
	default:
		parts := make([]string, 0, len(m.Sample))
		for key, val := range m.Sample {
			parts = append(parts, fmt.Sprintf("%s=%s", key, m.HumanizeValue(val)))
		}
		return strings.Join(parts, ", ")
	}
}

func (m Metric) HumanizeValue(v float64) string {
	switch m.Contains {
	case Time:
		return time.Duration(v).String()
	default:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
}

func (m Metric) GetID() string {
	return m.Name
}

func (m *Metric) SetID(id string) error {
	m.Name = id
	return nil
}
