package expv2

import (
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
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
		AggregationWaitPeriod: types.NewNullDuration(5*time.Second, true),
		// Manually control and trigger the various steps
		// instead to be time dependent
		AggregationPeriod:  types.NewNullDuration(1*time.Hour, true),
		MetricPushInterval: types.NewNullDuration(1*time.Hour, true),
	})
	require.NoError(t, err)
	require.NoError(t, o.Start())
	require.Empty(t, o.collector.bq.PopAll())

	o.collector.aggregationPeriod = 3 * time.Second
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
	require.Len(t, buckets[0].Sinks, 1)

	counter, ok := buckets[0].Sinks[ts].(*metrics.CounterSink)
	require.True(t, ok)
	assert.Equal(t, 5.0, counter.Value)

	expTime := time.Date(2023, time.May, 1, 1, 1, 15, 0, time.UTC)
	assert.Equal(t, expTime, buckets[0].Time)
}

func TestOutputHandleFlushError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                    string
		err                     error
		abort                   bool
		expStopMetricCollection bool
		expAborted              bool
	}{
		{
			name:                    "no stop on generic errors",
			err:                     errors.New("a fake unknown error"),
			abort:                   true,
			expStopMetricCollection: false,
			expAborted:              false,
		},
		{
			name: "error code equals 4 but no abort",
			err: cloudapi.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusForbidden},
				Code:     4,
			},
			abort:                   false,
			expStopMetricCollection: true,
			expAborted:              false,
		},
		{
			name: "error code equals 4 and abort",
			err: cloudapi.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusForbidden},
				Code:     4,
			},
			abort:                   true,
			expStopMetricCollection: true,
			expAborted:              true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stopFuncCalled := false
			stopMetricCollection := false

			o := Output{
				logger: testutils.NewLogger(t),
				testStopFunc: func(error) {
					stopFuncCalled = true
				},
				stopSamplesCollection: make(chan struct{}),
			}
			o.config.StopOnError = null.BoolFrom(tc.abort)

			done := make(chan struct{})
			stopped := make(chan struct{})

			go func() {
				defer close(stopped)

				<-done
				select {
				case <-o.stopSamplesCollection:
					stopMetricCollection = true
				default:
				}
			}()

			o.handleFlushError(tc.err)
			close(done)
			<-stopped

			assert.Equal(t, tc.expStopMetricCollection, stopMetricCollection)
			assert.Equal(t, tc.expAborted, stopFuncCalled)
		})
	}
}

// assert that the output does not stuck or panic
// when it is called doesn't stuck
func TestOutputHandleFlushErrorMultipleTimes(t *testing.T) {
	t.Parallel()
	var stopFuncCalled int
	o := Output{
		logger:                testutils.NewLogger(t),
		stopSamplesCollection: make(chan struct{}),
		testStopFunc: func(error) {
			stopFuncCalled++
		},
	}
	o.config.StopOnError = null.BoolFrom(true)

	er := cloudapi.ErrorResponse{
		Response: &http.Response{
			StatusCode: http.StatusForbidden,
		},
		Code: 4,
	}
	o.handleFlushError(fmt.Errorf("first error: %w", er))
	o.handleFlushError(fmt.Errorf("second error: %w", er))
	assert.Equal(t, 1, stopFuncCalled)
}

func TestOutputAddMetricSamples(t *testing.T) {
	t.Parallel()

	stopSamples := make(chan struct{})
	o := Output{
		stopSamplesCollection: stopSamples,
	}
	require.Empty(t, o.GetBufferedSamples())

	s := metrics.Sample{}
	o.AddMetricSamples([]metrics.SampleContainer{
		metrics.Samples([]metrics.Sample{s}),
	})
	require.Len(t, o.GetBufferedSamples(), 1)

	// Not accept samples anymore when the chan is closed
	close(stopSamples)
	o.AddMetricSamples([]metrics.SampleContainer{
		metrics.Samples([]metrics.Sample{s}),
	})
	require.Empty(t, o.GetBufferedSamples())
}

func TestOutputPeriodicInvoke(t *testing.T) {
	t.Parallel()

	stop := make(chan struct{})
	var called uint64
	cb := func() {
		updated := atomic.AddUint64(&called, 1)
		if updated == 2 {
			close(stop)
		}
	}
	o := Output{stop: stop}
	o.wg.Add(1)
	go o.periodicInvoke(time.Duration(1), cb) // loop
	<-stop
	assert.Greater(t, atomic.LoadUint64(&called), uint64(1))
}
