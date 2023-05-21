package expv2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/metrics"
)

// TODO: additional case
// case: add when the metric already exist
// case: add when the metric and the timeseries already exist

func TestMetricSetBuilderAddTimeBucket(t *testing.T) {
	t.Parallel()

	r := metrics.NewRegistry()
	m1 := r.MustNewMetric("metric1", metrics.Counter)
	timeSeries := metrics.TimeSeries{
		Metric: m1,
		Tags:   r.RootTagSet().With("key1", "val1"),
	}

	tb := timeBucket{
		Time: time.Unix(1, 0),
		Sinks: map[metrics.TimeSeries]metrics.Sink{
			timeSeries: &metrics.CounterSink{},
		},
	}
	msb := newMetricSetBuilder("testrunid-123", 1)
	msb.addTimeBucket(&tb)

	assert.Contains(t, msb.metrics, m1)
	require.Contains(t, msb.seriesIndex, timeSeries)
	assert.Equal(t, uint(0), msb.seriesIndex[timeSeries]) // TODO: assert with another number

	require.Len(t, msb.MetricSet.Metrics, 1)
	assert.Len(t, msb.MetricSet.Metrics[0].TimeSeries, 1)
}
