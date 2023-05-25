package expv2

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
	"gopkg.in/guregu/null.v3"
)

func TestNew(t *testing.T) {
	t.Parallel()

	logger, hook := testutils.NewLoggerWithHook(t)
	o, err := New(logger, cloudapi.Config{APIVersion: null.IntFrom(99)})
	require.NoError(t, err)
	require.NotNil(t, o)

	// assert the prefixed logging
	o.logger.Info("aaa")
	loglines := hook.Drain()
	require.Len(t, loglines, 1)
	assert.Equal(t, loglines[0].Data["output"], "cloudv2")

	// assert the config set
	assert.Equal(t, int64(99), o.config.APIVersion.Int64)
}

func TestOutputSetReferenceID(t *testing.T) {
	t.Parallel()
	o := Output{}
	o.SetReferenceID("my-test-run-id")
	assert.Equal(t, "my-test-run-id", o.referenceID)
}

func TestOutputSetTestRunStopCallback(t *testing.T) {
	t.Parallel()
	called := false
	o := Output{}
	o.SetTestRunStopCallback(func(e error) {
		assert.EqualError(t, e, "my new fake error")
		called = true
	})
	o.testStopFunc(errors.New("my new fake error"))
	assert.True(t, called)
}

func TestOutputCollectSamples(t *testing.T) {
	t.Parallel()
	o, err := New(testutils.NewLogger(t), cloudapi.Config{
		AggregationPeriod:     types.NewNullDuration(3*time.Second, true),
		AggregationWaitPeriod: types.NewNullDuration(5*time.Second, true),
		MetricPushInterval:    types.NewNullDuration(10*time.Second, true),
	})
	require.NoError(t, err)
	require.NoError(t, o.Start())

	// Manually control and trigger the various steps
	// instead to be time dependent
	o.periodicFlusher.Stop()

	o.periodicCollector.Stop()
	require.Empty(t, o.collector.bq.PopAll())

	o.collector.nowFunc = func() time.Time {
		// the cut off will be set to (22-1)
		return time.Date(2023, time.May, 1, 1, 1, 20, 0, time.UTC)
	}

	r := metrics.NewRegistry()
	m1 := r.MustNewMetric("metric1", metrics.Counter)
	ts := metrics.TimeSeries{
		Metric: m1,
		Tags:   r.RootTagSet().With("key1", "val1"),
	}

	s1 := metrics.Sample{
		TimeSeries: ts,
		Time:       time.Date(2023, time.May, 1, 1, 1, 15, 0, time.UTC),
		Value:      1.0,
	}

	s2 := metrics.Sample{
		TimeSeries: ts,
		Time:       time.Date(2023, time.May, 1, 1, 1, 18, 0, time.UTC),
		Value:      2.0,
	}

	s3 := metrics.Sample{
		TimeSeries: ts,
		Time:       time.Date(2023, time.May, 1, 1, 1, 15, 0, time.UTC),
		Value:      4.0,
	}

	o.collector.CollectSamples([]metrics.SampleContainer{
		metrics.Samples{s1},
		metrics.Samples{s2},
		metrics.Samples{s3},
	})
	buckets := o.collector.bq.PopAll()
	require.Len(t, buckets, 1)
	require.Contains(t, buckets[0].Sinks, ts)

	counter, ok := buckets[0].Sinks[ts].(*metrics.CounterSink)
	require.True(t, ok)
	assert.Equal(t, 5.0, counter.Value)

	expTime := time.Date(2023, time.May, 1, 1, 1, 15, 0, time.UTC)
	assert.Equal(t, expTime, buckets[0].Time)
}
