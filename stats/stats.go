/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package stats

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/guregu/null.v3"
)

const (
	counterString = `"counter"`
	gaugeString   = `"gauge"`
	trendString   = `"trend"`
	rateString    = `"rate"`

	defaultString = `"default"`
	timeString    = `"time"`
)

// Possible values for MetricType.
const (
	Counter = MetricType(iota) // A counter that sums its data points
	Gauge                      // A gauge that displays the latest value
	Trend                      // A trend, min/max/avg/med are interesting
	Rate                       // A rate, displays % of values that aren't 0
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
	case Rate:
		return []byte(rateString), nil
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
	case rateString:
		*t = Rate
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
	case Rate:
		return rateString
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
	Tainted  null.Bool  `json:"tainted"`

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

func (m Metric) NewSink() Sink {
	switch m.Type {
	case Counter:
		return &CounterSink{}
	case Gauge:
		return &GaugeSink{}
	case Trend:
		return &TrendSink{}
	case Rate:
		return &RateSink{}
	default:
		return nil
	}
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
		sort.Strings(parts)
		return strings.Join(parts, ", ")
	}
}

func (m Metric) HumanizeValue(v float64) string {
	switch m.Type {
	case Rate:
		return strconv.FormatFloat(100*v, 'f', 2, 64) + "%"
	default:
		switch m.Contains {
		case Time:
			d := ToD(v)
			switch {
			case d > time.Minute:
				d -= d % (1 * time.Second)
			case d > time.Second:
				d -= d % (10 * time.Millisecond)
			case d > time.Millisecond:
				d -= d % (10 * time.Microsecond)
			case d > time.Microsecond:
				d -= d % (10 * time.Nanosecond)
			}
			return d.String()
		default:
			return strconv.FormatFloat(v, 'f', -1, 64)
		}
	}
}

func (m Metric) GetID() string {
	return m.Name
}

func (m *Metric) SetID(id string) error {
	m.Name = id
	return nil
}
