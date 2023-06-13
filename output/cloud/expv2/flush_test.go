package expv2

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output/cloud/expv2/pbcloud"
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
		Time: 1,
		Sinks: map[metrics.TimeSeries]metricValue{
			timeSeries: &counter{},
		},
	}
	msb := newMetricSetBuilder("testrunid-123", 1)
	msb.addTimeBucket(tb)

	assert.Contains(t, msb.metrics, m1)
	require.Contains(t, msb.seriesIndex, timeSeries)
	assert.Equal(t, uint(0), msb.seriesIndex[timeSeries]) // TODO: assert with another number

	require.Len(t, msb.MetricSet.Metrics, 1)
	assert.Len(t, msb.MetricSet.Metrics[0].TimeSeries, 1)
}

func TestMetricsFlusherFlushChunk(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		series        int
		expFlushCalls int
	}{
		{series: 5, expFlushCalls: 2},
		{series: 2, expFlushCalls: 1},
	}

	r := metrics.NewRegistry()
	m1 := r.MustNewMetric("metric1", metrics.Counter)

	for _, tc := range testCases {
		bq := &bucketQ{}
		pm := &pusherMock{}
		mf := metricsFlusher{
			bq:                     bq,
			client:                 pm,
			maxSeriesInSingleBatch: 3,
		}

		bq.buckets = make([]timeBucket, 0, tc.series)
		for i := 0; i < tc.series; i++ {
			ts := metrics.TimeSeries{
				Metric: m1,
				Tags:   r.RootTagSet().With("key1", "val"+strconv.Itoa(i)),
			}
			bq.Push([]timeBucket{
				{
					Time: int64(i) + 1,
					Sinks: map[metrics.TimeSeries]metricValue{
						ts: &counter{Sum: float64(1)},
					},
				},
			})
		}
		require.Len(t, bq.buckets, tc.series)

		err := mf.flush(context.Background())
		require.NoError(t, err)
		assert.Equal(t, tc.expFlushCalls, pm.pushCalled)
	}
}

type pusherMock struct {
	pushCalled int
}

func (pm *pusherMock) push(_ string, _ *pbcloud.MetricSet) error {
	pm.pushCalled++
	return nil
}
