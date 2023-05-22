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
		{metrics.Trend, &histogram{}},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.exp, newMetricValue(tc.mt))
	}
}
