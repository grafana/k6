package expv2

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/cloudapi/insights"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output/cloud/expv2/pbcloud"
)

type mockWorkingInsightsClient struct {
	ingestRequestMetadatasBatchInvoked bool
	dataSent                           bool
	data                               insights.RequestMetadatas
}

func (c *mockWorkingInsightsClient) IngestRequestMetadatasBatch(ctx context.Context, data insights.RequestMetadatas) error {
	c.ingestRequestMetadatasBatchInvoked = true

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	c.dataSent = true
	c.data = data

	return nil
}

func (c *mockWorkingInsightsClient) Close() error {
	return nil
}

type mockFailingInsightsClient struct {
	err error
}

func (c *mockFailingInsightsClient) IngestRequestMetadatasBatch(_ context.Context, _ insights.RequestMetadatas) error {
	return c.err
}

func (c *mockFailingInsightsClient) Close() error {
	return nil
}

type mockRequestMetadatasCollector struct {
	data insights.RequestMetadatas
}

func (m *mockRequestMetadatasCollector) CollectRequestMetadatas(_ []metrics.SampleContainer) {
	panic("implement me")
}

func (m *mockRequestMetadatasCollector) PopAll() insights.RequestMetadatas {
	return m.data
}

func newMockRequestMetadatas() insights.RequestMetadatas {
	return insights.RequestMetadatas{
		{
			TraceID:        "test-trace-id-1",
			Start:          time.Unix(1337, 0),
			End:            time.Unix(1338, 0),
			TestRunLabels:  insights.TestRunLabels{ID: 1, Scenario: "test-scenario-1", Group: "test-group-1"},
			ProtocolLabels: insights.ProtocolHTTPLabels{URL: "test-url-1", Method: "test-method-1", StatusCode: 200},
		},
		{
			TraceID:        "test-trace-id-2",
			Start:          time.Unix(2337, 0),
			End:            time.Unix(2338, 0),
			TestRunLabels:  insights.TestRunLabels{ID: 1, Scenario: "test-scenario-2", Group: "test-group-2"},
			ProtocolLabels: insights.ProtocolHTTPLabels{URL: "test-url-2", Method: "test-method-2", StatusCode: 200},
		},
	}
}

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

	msb := newMetricSetBuilder("testrunid-123", 1)
	msb.addTimeSeries(1, timeSeries, &counter{})

	assert.Contains(t, msb.metrics, m1)
	require.Contains(t, msb.seriesIndex, timeSeries)
	assert.Equal(t, uint(0), msb.seriesIndex[timeSeries]) // TODO: assert with another number

	require.Len(t, msb.MetricSet.Metrics, 1)
	assert.Len(t, msb.MetricSet.Metrics[0].TimeSeries, 1)
}
func TestMetricsFlusherFlushInBatchWithinBucket(t *testing.T) {
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
		logger, _ := testutils.NewLoggerWithHook(t)

		bq := &bucketQ{}
		pm := &pusherMock{}
		mf := metricsFlusher{
			bq:               bq,
			client:           pm,
			logger:           logger,
			discardedLabels:  make(map[string]struct{}),
			maxSeriesInBatch: 3,
		}

		bq.buckets = make([]timeBucket, 0, tc.series)
		sinks := make(map[metrics.TimeSeries]metricValue)
		for i := 0; i < tc.series; i++ {
			ts := metrics.TimeSeries{
				Metric: m1,
				Tags:   r.RootTagSet().With("key1", "val"+strconv.Itoa(i)),
			}

			sinks[ts] = &counter{Sum: float64(1)}
		}
		require.Len(t, sinks, tc.series)

		bq.Push([]timeBucket{{Time: 1, Sinks: sinks}})
		err := mf.flush()
		require.NoError(t, err)
		assert.Equal(t, tc.expFlushCalls, pm.pushCalled)
	}
}

func TestMetricsFlusherFlushInBatchAcrossBuckets(t *testing.T) {
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
		logger, _ := testutils.NewLoggerWithHook(t)

		bq := &bucketQ{}
		pm := &pusherMock{}
		mf := metricsFlusher{
			bq:               bq,
			client:           pm,
			logger:           logger,
			discardedLabels:  make(map[string]struct{}),
			maxSeriesInBatch: 3,
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

		err := mf.flush()
		require.NoError(t, err)
		assert.Equal(t, tc.expFlushCalls, pm.pushCalled)
	}
}

func TestFlushWithReservedLabels(t *testing.T) {
	t.Parallel()

	logger, hook := testutils.NewLoggerWithHook(t)

	collected := make([]*pbcloud.MetricSet, 0)

	bq := &bucketQ{}
	pm := &pusherMock{
		hook: func(ms *pbcloud.MetricSet) {
			collected = append(collected, ms)
		},
	}

	mf := metricsFlusher{
		bq:               bq,
		client:           pm,
		maxSeriesInBatch: 2,
		logger:           logger,
		discardedLabels:  make(map[string]struct{}),
	}

	r := metrics.NewRegistry()
	m1 := r.MustNewMetric("metric1", metrics.Counter)

	ts1 := metrics.TimeSeries{
		Metric: m1,
		Tags:   r.RootTagSet().With("key1", "val1").With("__name__", "val2").With("test_run_id", "testrunid-123"),
	}
	bq.Push([]timeBucket{
		{
			Time: 1,
			Sinks: map[metrics.TimeSeries]metricValue{
				ts1: &counter{Sum: float64(1)},
			},
		},
	})

	ts2 := metrics.TimeSeries{
		Metric: m1,
		Tags:   r.RootTagSet().With("key1", "val2").With("__name__", "val2"),
	}
	bq.Push([]timeBucket{
		{
			Time: 2,
			Sinks: map[metrics.TimeSeries]metricValue{
				ts2: &counter{Sum: float64(1)},
			},
		},
	})

	err := mf.flush()
	require.NoError(t, err)

	loglines := hook.Drain()
	assert.Equal(t, 1, len(collected))

	// check that warnings sown only once per label
	assert.Len(t, testutils.FilterEntries(loglines, logrus.WarnLevel, "Tag __name__ has been discarded since it is reserved for Cloud operations."), 1)
	assert.Len(t, testutils.FilterEntries(loglines, logrus.WarnLevel, "Tag test_run_id has been discarded since it is reserved for Cloud operations."), 1)

	// check that flusher is not sending labels with reserved names
	require.Len(t, collected[0].Metrics, 1)

	ts := collected[0].Metrics[0].TimeSeries
	require.Len(t, ts[0].Labels, 1)
	assert.Equal(t, "key1", ts[0].Labels[0].Name)
	assert.Equal(t, "val1", ts[0].Labels[0].Value)

	require.Len(t, ts[1].Labels, 1)
	assert.Equal(t, "key1", ts[1].Labels[0].Name)
	assert.Equal(t, "val2", ts[1].Labels[0].Value)
}

type pusherMock struct {
	hook       func(*pbcloud.MetricSet)
	pushCalled int
}

func (pm *pusherMock) push(ms *pbcloud.MetricSet) error {
	if pm.hook != nil {
		pm.hook(ms)
	}

	pm.pushCalled++
	return nil
}

func Test_tracesFlusher_Flush_ReturnsNoErrorWithWorkingInsightsClientAndNonCancelledContextAndNoData(t *testing.T) {
	t.Parallel()

	// Given
	data := insights.RequestMetadatas{}
	cli := &mockWorkingInsightsClient{}
	col := &mockRequestMetadatasCollector{data: data}
	flusher := newTracesFlusher(cli, col)

	// When
	err := flusher.flush()

	// Then
	require.NoError(t, err)
	require.False(t, cli.ingestRequestMetadatasBatchInvoked)
	require.False(t, cli.dataSent)
	require.Empty(t, cli.data)
}

func Test_tracesFlusher_Flush_ReturnsNoErrorWithWorkingInsightsClientAndNonCancelledContextAndData(t *testing.T) {
	t.Parallel()

	// Given
	data := newMockRequestMetadatas()
	cli := &mockWorkingInsightsClient{}
	col := &mockRequestMetadatasCollector{data: data}
	flusher := newTracesFlusher(cli, col)

	// When
	err := flusher.flush()

	// Then
	require.NoError(t, err)
	require.True(t, cli.ingestRequestMetadatasBatchInvoked)
	require.True(t, cli.dataSent)
	require.Equal(t, data, cli.data)
}

func Test_tracesFlusher_Flush_ReturnsErrorWithFailingInsightsClientAndNonCancelledContext(t *testing.T) {
	t.Parallel()

	// Given
	data := newMockRequestMetadatas()
	testErr := errors.New("test-error")
	cli := &mockFailingInsightsClient{err: testErr}
	col := &mockRequestMetadatasCollector{data: data}
	flusher := newTracesFlusher(cli, col)

	// When
	err := flusher.flush()

	// Then
	require.ErrorIs(t, err, testErr)
}
