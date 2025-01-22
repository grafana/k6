package influxdb

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
)

func TestBadConcurrentWrites(t *testing.T) {
	t.Parallel()
	logger := testutils.NewLogger(t)
	t.Run("0", func(t *testing.T) {
		t.Parallel()
		_, err := New(output.Params{
			Logger:         logger,
			ConfigArgument: "?concurrentWrites=0",
		})
		require.Error(t, err)
		require.Equal(t, err.Error(), "influxdb's ConcurrentWrites must be a positive number")
	})

	t.Run("-2", func(t *testing.T) {
		t.Parallel()
		_, err := New(output.Params{
			Logger:         logger,
			ConfigArgument: "?concurrentWrites=-2",
		})
		require.Error(t, err)
		require.Equal(t, err.Error(), "influxdb's ConcurrentWrites must be a positive number")
	})

	t.Run("2", func(t *testing.T) {
		t.Parallel()
		_, err := New(output.Params{
			Logger:         logger,
			ConfigArgument: "?concurrentWrites=2",
		})
		require.NoError(t, err)
	})
}

func testOutputCycle(t testing.TB, handler http.HandlerFunc, body func(testing.TB, *Output)) {
	s := &http.Server{
		Addr:              ":",
		Handler:           handler,
		MaxHeaderBytes:    1 << 20,
		ReadHeaderTimeout: time.Second,
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() {
		_ = l.Close()
	}()

	defer func() {
		require.NoError(t, s.Shutdown(context.Background()))
	}()

	go func() {
		require.Equal(t, http.ErrServerClosed, s.Serve(l))
	}()

	c, err := newOutput(output.Params{
		Logger:         testutils.NewLogger(t),
		ConfigArgument: "http://" + l.Addr().String(),
	})
	require.NoError(t, err)

	require.NoError(t, c.Start())
	body(t, c)

	require.NoError(t, c.Stop())
}

func TestOutput(t *testing.T) {
	t.Parallel()

	registry := metrics.NewRegistry()
	metric, err := registry.NewMetric("test_gauge", metrics.Gauge)
	require.NoError(t, err)

	var samplesRead int
	defer func() {
		require.Equal(t, samplesRead, 20)
	}()

	testOutputCycle(t, func(rw http.ResponseWriter, r *http.Request) {
		b := bytes.NewBuffer(nil)
		_, _ = io.Copy(b, r.Body)
		for {
			s, err := b.ReadString('\n')
			if len(s) > 0 {
				samplesRead++
			}
			if err != nil {
				break
			}
		}

		rw.WriteHeader(http.StatusNoContent)
	}, func(_ testing.TB, c *Output) {
		samples := make(metrics.Samples, 10)
		for i := 0; i < len(samples); i++ {
			samples[i] = metrics.Sample{
				TimeSeries: metrics.TimeSeries{
					Metric: metric,
					Tags: registry.RootTagSet().WithTagsFromMap(map[string]string{
						"something": "else",
						"VU":        "21",
						"else":      "something",
					}),
				},
				Time:  time.Now(),
				Value: 2.0,
			}
		}
		c.AddMetricSamples([]metrics.SampleContainer{samples})
		c.AddMetricSamples([]metrics.SampleContainer{samples})
	})
}

func TestOutputFlushMetricsConcurrency(t *testing.T) {
	t.Parallel()

	var (
		requests = int32(0)
		block    = make(chan struct{})
	)

	wg := sync.WaitGroup{}
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		// block all the received requests
		// so concurrency will be needed
		// to not block the flush
		atomic.AddInt32(&requests, 1)
		wg.Done()
		block <- struct{}{}
	}))
	defer func() {
		// unlock the server
		for i := 0; i < 4; i++ {
			<-block
		}
		close(block)
		ts.Close()
	}()

	registry := metrics.NewRegistry()
	metric, err := registry.NewMetric("test_gauge", metrics.Gauge)
	require.NoError(t, err)

	o, err := newOutput(output.Params{
		Logger:         testutils.NewLogger(t),
		ConfigArgument: ts.URL,
	})
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		select {
		case o.semaphoreCh <- struct{}{}:
			<-o.semaphoreCh
			wg.Add(1)
			o.AddMetricSamples([]metrics.SampleContainer{metrics.Samples{
				metrics.Sample{
					TimeSeries: metrics.TimeSeries{
						Metric: metric,
						Tags:   registry.RootTagSet(),
					},
					Time:  time.Now(),
					Value: 2.0,
				},
			}})
			o.flushMetrics()
		default:
			// the 5th request should be rate limited
			assert.Equal(t, 5, i+1)
		}
	}
	wg.Wait()
	assert.Equal(t, 4, int(atomic.LoadInt32(&requests)))
}

func TestExtractTagsToValues(t *testing.T) {
	t.Parallel()
	o, err := newOutput(output.Params{
		Logger:         testutils.NewLogger(t),
		ConfigArgument: "?tagsAsFields=stringField&tagsAsFields=stringField2:string&tagsAsFields=boolField:bool&tagsAsFields=floatField:float&tagsAsFields=intField:int",
	})
	require.NoError(t, err)
	tags := map[string]string{
		"stringField":  "string",
		"stringField2": "string2",
		"boolField":    "true",
		"floatField":   "3.14",
		"intField":     "12345",
	}
	values := o.extractTagsToValues(tags, map[string]interface{}{})

	require.Equal(t, "string", values["stringField"])
	require.Equal(t, "string2", values["stringField2"])
	require.Equal(t, true, values["boolField"])
	require.Equal(t, 3.14, values["floatField"])
	require.Equal(t, int64(12345), values["intField"])
}
