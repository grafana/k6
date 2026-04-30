package expv2

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/internal/output/cloud/expv2/pbcloud"
	"go.k6.io/k6/v2/metrics"
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

	msb := newMetricSetBuilder("testrunid-123", 1)
	msb.addTimeSeries(1, timeSeries, &counter{})

	assert.Contains(t, msb.metrics, m1)
	require.Contains(t, msb.seriesIndex, timeSeries)
	assert.Equal(t, 0, msb.seriesIndex[timeSeries]) // TODO: assert with another number

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
			bq:                   bq,
			client:               pm,
			logger:               logger,
			discardedLabels:      make(map[string]struct{}),
			maxSeriesInBatch:     3,
			batchPushConcurrency: 5,
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
		err := mf.flush(context.Background())
		require.NoError(t, err)
		assert.Equal(t, tc.expFlushCalls, pm.timesCalled())
	}
}

func TestMetricsFlusherFlushInBatchAcrossBuckets(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		series       int
		expPushCalls int
	}{
		{series: 5, expPushCalls: 2},
		{series: 2, expPushCalls: 1},
	}

	r := metrics.NewRegistry()
	m1 := r.MustNewMetric("metric1", metrics.Counter)
	for _, tc := range testCases {
		logger, _ := testutils.NewLoggerWithHook(t)

		bq := &bucketQ{}
		pm := &pusherMock{}
		mf := metricsFlusher{
			bq:                   bq,
			client:               pm,
			logger:               logger,
			discardedLabels:      make(map[string]struct{}),
			maxSeriesInBatch:     3,
			batchPushConcurrency: 5,
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
		assert.Equal(t, tc.expPushCalls, pm.timesCalled())
	}
}

func TestFlushWithReservedLabels(t *testing.T) {
	t.Parallel()

	logger, hook := testutils.NewLoggerWithHook(t)

	mutex := sync.Mutex{}
	collected := make([]*pbcloud.MetricSet, 0)

	bq := &bucketQ{}
	pm := &pusherMock{
		hook: func(ms *pbcloud.MetricSet) {
			mutex.Lock()
			collected = append(collected, ms)
			mutex.Unlock()
		},
	}

	mf := metricsFlusher{
		bq:                   bq,
		client:               pm,
		maxSeriesInBatch:     2,
		logger:               logger,
		discardedLabels:      make(map[string]struct{}),
		batchPushConcurrency: 5,
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

	err := mf.flush(context.Background())
	require.NoError(t, err)

	loglines := hook.Drain()
	require.Len(t, collected, 1)

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

func TestFlushMaxSeriesInBatch(t *testing.T) {
	t.Parallel()

	logger := testutils.NewLogger(t)

	mutex := sync.Mutex{}
	collected := make([]*pbcloud.MetricSet, 0)

	bq := &bucketQ{}
	pm := &pusherMock{
		hook: func(ms *pbcloud.MetricSet) {
			mutex.Lock()
			collected = append(collected, ms)
			mutex.Unlock()
		},
	}

	mf := metricsFlusher{
		bq:                   bq,
		client:               pm,
		maxSeriesInBatch:     2,
		logger:               logger,
		discardedLabels:      make(map[string]struct{}),
		batchPushConcurrency: 5,
	}

	r := metrics.NewRegistry()
	m1 := r.MustNewMetric("metric1", metrics.Counter)

	ts1 := metrics.TimeSeries{
		Metric: m1,
		Tags:   r.RootTagSet().With("key1", "val1"),
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
		Tags:   r.RootTagSet().With("key1", "val2"),
	}
	bq.Push([]timeBucket{
		{
			Time: 2,
			Sinks: map[metrics.TimeSeries]metricValue{
				ts2: &counter{Sum: float64(2)},
			},
		},
	})

	ts3 := metrics.TimeSeries{
		Metric: m1,
		Tags:   r.RootTagSet().With("key1", "val3"),
	}
	bq.Push([]timeBucket{
		{
			Time: 3,
			Sinks: map[metrics.TimeSeries]metricValue{
				ts3: &counter{Sum: float64(3)},
			},
		},
	})
	err := mf.flush(context.Background())
	require.NoError(t, err)

	require.Len(t, collected, 2)

	// Collect all label values across all batches (order-independent)
	// to avoid flakiness from concurrent batch pushing
	allLabelValues := make(map[string]struct{})
	batchSizes := make([]int, 0, len(collected))

	for _, ms := range collected {
		require.Len(t, ms.Metrics, 1)
		ts := ms.Metrics[0].TimeSeries
		batchSizes = append(batchSizes, len(ts))

		for _, series := range ts {
			require.Len(t, series.Labels, 1)
			assert.Equal(t, "key1", series.Labels[0].Name)
			allLabelValues[series.Labels[0].Value] = struct{}{}
		}
	}

	// Verify all expected values were flushed
	assert.Contains(t, allLabelValues, "val1")
	assert.Contains(t, allLabelValues, "val2")
	assert.Contains(t, allLabelValues, "val3")
	assert.Len(t, allLabelValues, 3)

	// Verify batch sizes: one batch with 2 series, one with 1 series
	assert.ElementsMatch(t, []int{2, 1}, batchSizes)
}

type pusherMock struct {
	// hook is called when the push method is called.
	hook func(*pbcloud.MetricSet)
	// errFn if this defined, it is called at the end of push
	// and result error is returned.
	errFn      func() error
	pushCalled int64
}

func (pm *pusherMock) timesCalled() int {
	return int(atomic.LoadInt64(&pm.pushCalled))
}

func (pm *pusherMock) push(_ context.Context, ms *pbcloud.MetricSet) error {
	if pm.hook != nil {
		pm.hook(ms)
	}

	atomic.AddInt64(&pm.pushCalled, 1)

	if pm.errFn != nil {
		return pm.errFn()
	}

	return nil
}

// TestMetricsFlusherFlushReturnsOnContextCancel reproduces the production hang
// observed when k6agent SIGTERMs k6 during shutdown but a metric push blocks
// indefinitely (HTTP/2 read loop stuck on a TLS connection). The flush MUST
// return when the caller cancels the context, otherwise the whole shutdown
// hangs and k6agent has to SIGQUIT k6.
//
// See grafana/k6-cloud-agent#291.
func TestMetricsFlusherFlushReturnsOnContextCancel(t *testing.T) {
	t.Parallel()

	r := metrics.NewRegistry()
	m1 := r.MustNewMetric("metric1", metrics.Counter)

	logger, _ := testutils.NewLoggerWithHook(t)

	released := make(chan struct{})
	defer close(released)
	pm := &blockingPusher{released: released}

	bq := &bucketQ{}
	mf := metricsFlusher{
		bq:                   bq,
		client:               pm,
		logger:               logger,
		discardedLabels:      make(map[string]struct{}),
		maxSeriesInBatch:     1,
		batchPushConcurrency: 1,
	}

	for i := range 5 {
		ts := metrics.TimeSeries{
			Metric: m1,
			Tags:   r.RootTagSet().With("key1", "val"+strconv.Itoa(i)),
		}
		bq.Push([]timeBucket{
			{
				Time: int64(i) + 1,
				Sinks: map[metrics.TimeSeries]metricValue{
					ts: &counter{Sum: 1},
				},
			},
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	done := make(chan error, 1)
	go func() { done <- mf.flush(ctx) }()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("flush did not return after context was cancelled (production hang reproduced)")
	}
}

// blockingPusher blocks push() until released is closed. It honours context
// cancellation so flush can interrupt a stuck push.
type blockingPusher struct {
	released chan struct{}
	called   atomic.Int64
}

func (bp *blockingPusher) push(ctx context.Context, _ *pbcloud.MetricSet) error {
	bp.called.Add(1)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-bp.released:
		return nil
	}
}

func TestMetricsFlusherErrorCase(t *testing.T) {
	t.Parallel()

	r := metrics.NewRegistry()
	m1 := r.MustNewMetric("metric1", metrics.Counter)

	logger, _ := testutils.NewLoggerWithHook(t)

	bq := &bucketQ{}
	pm := &pusherMock{
		errFn: func() error {
			return errors.New("some error")
		},
	}
	mf := metricsFlusher{
		bq:                   bq,
		client:               pm,
		logger:               logger,
		discardedLabels:      make(map[string]struct{}),
		maxSeriesInBatch:     3,
		batchPushConcurrency: 2,
	}

	series := 7

	bq.buckets = make([]timeBucket, 0, series)
	for i := range series {
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
	require.Len(t, bq.buckets, series)

	err := mf.flush(context.Background())
	require.Error(t, err)
	// since the push happens concurrently the number of the calls could vary,
	// but at least one call should happen and it should be less than the
	// batchPushConcurrency
	assert.LessOrEqual(t, pm.timesCalled(), mf.batchPushConcurrency)
	assert.GreaterOrEqual(t, pm.timesCalled(), 1)
}
