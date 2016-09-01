package stats

import (
	"sort"
	"time"
)

// The type of a metric.
type MetricType int

// Possible values for MetricType.
const (
	NoType  = MetricType(iota) // No type; metrics like this are ignored
	Counter                    // A counter that sums its data points
	Gauge                      // A gauge that displays the latest value
	Trend                      // A trend, min/max/avg/med are interesting
)

// The type of values a metric contains.
type ValueType int

// Possible values for ValueType.
const (
	Default = ValueType(iota) // Values are presented as-is
	Time                      // Values are timestamps (nanoseconds)
)

// A Sample is a single measurement.
type Sample struct {
	Time  time.Time
	Tags  map[string]string
	Value float64
}

// An MSample is a Sample tagged with a Metric, to make returning samples easier.
type FatSample struct {
	Sample
	Metric *Metric
}

// A Metric defines the shape of a set of data.
type Metric struct {
	Name     string
	Type     MetricType
	Contains ValueType
}

func New(name string, typ MetricType, t ...ValueType) *Metric {
	vt := Default
	if len(t) > 0 {
		vt = t[0]
	}
	return &Metric{Name: name, Type: typ, Contains: vt}
}

func (m *Metric) Format(samples []Sample) map[string]float64 {
	switch m.Type {
	case Counter:
		var total float64
		for _, s := range samples {
			total += s.Value
		}
		return map[string]float64{"value": total}
	case Gauge:
		l := len(samples)
		if l == 0 {
			return map[string]float64{"value": 0}
		}
		return map[string]float64{"value": samples[l-1].Value}
	case Trend:
		values := make([]float64, len(samples))
		for i, s := range samples {
			values[i] = s.Value
		}
		sort.Float64s(values)

		var min, max, avg, med, sum float64
		for i, v := range values {
			if v < min || i == 0 {
				min = v
			}
			if v > max {
				max = v
			}
			sum += v
		}

		l := len(values)
		switch l {
		case 0:
		case 1:
			avg = values[0]
			med = values[0]
		default:
			avg = sum / float64(l)
			med = values[l/2]

			// Median for an even number of values is the average of the middle two
			if (l & 0x01) == 0 {
				med = (med + values[(l/2)-1]) / 2
			}
		}

		return map[string]float64{
			"min": min,
			"max": max,
			"med": med,
			"avg": avg,
		}
	}

	return nil
}
