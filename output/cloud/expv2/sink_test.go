package expv2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/metrics"
)

func TestNewSink(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mt  metrics.MetricType
		exp any
	}{
		{metrics.Counter, &counter{}},
		{metrics.Gauge, &gauge{}},
		{metrics.Rate, &rate{}},
		{metrics.Trend, newHistogram()},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.exp, newMetricValue(tc.mt))
	}
}

func TestCounterAdd(t *testing.T) {
	t.Parallel()

	c := counter{}
	c.Add(2.3)
	assert.Equal(t, 2.3, c.Sum)

	c.Add(1.78)
	assert.Equal(t, 4.08, c.Sum)
}

func TestGaugeAdd(t *testing.T) {
	t.Parallel()

	g := gauge{}
	g.Add(28)

	exp := gauge{
		Last:   28,
		Sum:    28,
		Min:    28,
		Max:    28,
		Avg:    28,
		Count:  1,
		minSet: true,
	}
	assert.Equal(t, exp, g)

	g.Add(8.28)
	exp = gauge{
		Last:   8.28,
		Sum:    36.28,
		Min:    8.28,
		Max:    28,
		Avg:    18.14,
		Count:  2,
		minSet: true,
	}
	assert.Equal(t, exp, g)
}

func TestRateAdd(t *testing.T) {
	t.Parallel()

	r := rate{}
	r.Add(91)

	exp := rate{
		NonZeroCount: 1,
		Total:        1,
	}
	assert.Equal(t, exp, r)

	r.Add(0)
	exp.Total = 2
	assert.Equal(t, exp, r)
}
