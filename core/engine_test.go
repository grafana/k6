/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package core

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/core/local"
	"github.com/loadimpact/k6/js"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/testutils/httpmultibin"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/dummy"
)

// Apply a null logger to the engine and return the hook.
func applyNullLogger(e *Engine) *logtest.Hook {
	logger, hook := logtest.NewNullLogger()
	e.SetLogger(logger)
	return hook
}

// Wrapper around newEngine that applies a null logger.
func newTestEngine(ex lib.Executor, opts lib.Options) (*Engine, error) {
	if !opts.MetricSamplesBufferSize.Valid {
		opts.MetricSamplesBufferSize = null.IntFrom(200)
	}
	e, err := NewEngine(ex, opts)
	if err != nil {
		return e, err
	}
	applyNullLogger(e)
	return e, nil
}

func LF(fn func(ctx context.Context, out chan<- stats.SampleContainer) error) lib.Executor {
	return local.New(&lib.MiniRunner{Fn: fn})
}

func TestNewEngine(t *testing.T) {
	_, err := newTestEngine(nil, lib.Options{})
	assert.NoError(t, err)
}

func TestNewEngineOptions(t *testing.T) {
	t.Run("Duration", func(t *testing.T) {
		e, err := newTestEngine(nil, lib.Options{
			Duration: types.NullDurationFrom(10 * time.Second),
		})
		assert.NoError(t, err)
		assert.Nil(t, e.Executor.GetStages())
		assert.Equal(t, types.NullDurationFrom(10*time.Second), e.Executor.GetEndTime())

		t.Run("Infinite", func(t *testing.T) {
			e, err := newTestEngine(nil, lib.Options{Duration: types.NullDuration{}})
			assert.NoError(t, err)
			assert.Nil(t, e.Executor.GetStages())
			assert.Equal(t, types.NullDuration{}, e.Executor.GetEndTime())
		})
	})
	t.Run("Stages", func(t *testing.T) {
		e, err := newTestEngine(nil, lib.Options{
			Stages: []lib.Stage{
				{Duration: types.NullDurationFrom(10 * time.Second), Target: null.IntFrom(10)},
			},
		})
		assert.NoError(t, err)
		if assert.Len(t, e.Executor.GetStages(), 1) {
			assert.Equal(t, e.Executor.GetStages()[0], lib.Stage{Duration: types.NullDurationFrom(10 * time.Second), Target: null.IntFrom(10)})
		}
	})
	t.Run("Stages/Duration", func(t *testing.T) {
		e, err := newTestEngine(nil, lib.Options{
			Duration: types.NullDurationFrom(60 * time.Second),
			Stages: []lib.Stage{
				{Duration: types.NullDurationFrom(10 * time.Second), Target: null.IntFrom(10)},
			},
		})
		assert.NoError(t, err)
		if assert.Len(t, e.Executor.GetStages(), 1) {
			assert.Equal(t, e.Executor.GetStages()[0], lib.Stage{Duration: types.NullDurationFrom(10 * time.Second), Target: null.IntFrom(10)})
		}
		assert.Equal(t, types.NullDurationFrom(60*time.Second), e.Executor.GetEndTime())
	})
	t.Run("Iterations", func(t *testing.T) {
		e, err := newTestEngine(nil, lib.Options{Iterations: null.IntFrom(100)})
		assert.NoError(t, err)
		assert.Equal(t, null.IntFrom(100), e.Executor.GetEndIterations())
	})
	t.Run("VUsMax", func(t *testing.T) {
		t.Run("not set", func(t *testing.T) {
			e, err := newTestEngine(nil, lib.Options{})
			assert.NoError(t, err)
			assert.Equal(t, int64(0), e.Executor.GetVUsMax())
			assert.Equal(t, int64(0), e.Executor.GetVUs())
		})
		t.Run("set", func(t *testing.T) {
			e, err := newTestEngine(nil, lib.Options{
				VUsMax: null.IntFrom(10),
			})
			assert.NoError(t, err)
			assert.Equal(t, int64(10), e.Executor.GetVUsMax())
			assert.Equal(t, int64(0), e.Executor.GetVUs())
		})
	})
	t.Run("VUs", func(t *testing.T) {
		t.Run("no max", func(t *testing.T) {
			_, err := newTestEngine(nil, lib.Options{
				VUs: null.IntFrom(10),
			})
			assert.EqualError(t, err, "can't raise vu count (to 10) above vu cap (0)")
		})
		t.Run("negative max", func(t *testing.T) {
			_, err := newTestEngine(nil, lib.Options{
				VUsMax: null.IntFrom(-1),
			})
			assert.EqualError(t, err, "vu cap can't be negative")
		})
		t.Run("max too low", func(t *testing.T) {
			_, err := newTestEngine(nil, lib.Options{
				VUsMax: null.IntFrom(1),
				VUs:    null.IntFrom(10),
			})
			assert.EqualError(t, err, "can't raise vu count (to 10) above vu cap (1)")
		})
		t.Run("max higher", func(t *testing.T) {
			e, err := newTestEngine(nil, lib.Options{
				VUsMax: null.IntFrom(10),
				VUs:    null.IntFrom(1),
			})
			assert.NoError(t, err)
			assert.Equal(t, int64(10), e.Executor.GetVUsMax())
			assert.Equal(t, int64(1), e.Executor.GetVUs())
		})
		t.Run("max just right", func(t *testing.T) {
			e, err := newTestEngine(nil, lib.Options{
				VUsMax: null.IntFrom(10),
				VUs:    null.IntFrom(10),
			})
			assert.NoError(t, err)
			assert.Equal(t, int64(10), e.Executor.GetVUsMax())
			assert.Equal(t, int64(10), e.Executor.GetVUs())
		})
	})
	t.Run("Paused", func(t *testing.T) {
		t.Run("not set", func(t *testing.T) {
			e, err := newTestEngine(nil, lib.Options{})
			assert.NoError(t, err)
			assert.False(t, e.Executor.IsPaused())
		})
		t.Run("false", func(t *testing.T) {
			e, err := newTestEngine(nil, lib.Options{
				Paused: null.BoolFrom(false),
			})
			assert.NoError(t, err)
			assert.False(t, e.Executor.IsPaused())
		})
		t.Run("true", func(t *testing.T) {
			e, err := newTestEngine(nil, lib.Options{
				Paused: null.BoolFrom(true),
			})
			assert.NoError(t, err)
			assert.True(t, e.Executor.IsPaused())
		})
	})
	t.Run("thresholds", func(t *testing.T) {
		e, err := newTestEngine(nil, lib.Options{
			Thresholds: map[string]stats.Thresholds{
				"my_metric": {},
			},
		})
		assert.NoError(t, err)
		assert.Contains(t, e.thresholds, "my_metric")

		t.Run("submetrics", func(t *testing.T) {
			e, err := newTestEngine(nil, lib.Options{
				Thresholds: map[string]stats.Thresholds{
					"my_metric{tag:value}": {},
				},
			})
			assert.NoError(t, err)
			assert.Contains(t, e.thresholds, "my_metric{tag:value}")
			assert.Contains(t, e.submetrics, "my_metric")
		})
	})
}

func TestEngineRun(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	t.Run("exits with context", func(t *testing.T) {
		duration := 100 * time.Millisecond
		e, err := newTestEngine(nil, lib.Options{})
		assert.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), duration)
		defer cancel()
		startTime := time.Now()
		assert.NoError(t, e.Run(ctx))
		assert.WithinDuration(t, startTime.Add(duration), time.Now(), 100*time.Millisecond)
	})
	t.Run("exits with executor", func(t *testing.T) {
		e, err := newTestEngine(nil, lib.Options{
			VUs:        null.IntFrom(10),
			VUsMax:     null.IntFrom(10),
			Iterations: null.IntFrom(100),
		})
		assert.NoError(t, err)
		assert.NoError(t, e.Run(context.Background()))
		assert.Equal(t, int64(100), e.Executor.GetIterations())
	})

	// Make sure samples are discarded after context close (using "cutoff" timestamp in local.go)
	t.Run("collects samples", func(t *testing.T) {
		testMetric := stats.New("test_metric", stats.Trend)

		signalChan := make(chan interface{})
		var e *Engine
		e, err := newTestEngine(LF(func(ctx context.Context, samples chan<- stats.SampleContainer) error {
			samples <- stats.Sample{Metric: testMetric, Time: time.Now(), Value: 1}
			close(signalChan)
			<-ctx.Done()

			// HACK(robin): Add a sleep here to temporarily workaround two problems with this test:
			// 1. The sample times are compared against the `cutoff` in core/local/local.go and sometimes the
			//    second sample (below) gets a `Time` smaller than `cutoff` because the lines below get executed
			//    before the `<-ctx.Done()` select in local.go:Run() on multi-core systems where
			//    goroutines can run in parallel.
			// 2. Sometimes the `case samples := <-vuOut` gets selected before the `<-ctx.Done()` in
			//    core/local/local.go:Run() causing all samples from this mocked "RunOnce()" function to be accepted.
			time.Sleep(time.Millisecond * 10)
			samples <- stats.Sample{Metric: testMetric, Time: time.Now(), Value: 2}
			return nil
		}), lib.Options{
			VUs:        null.IntFrom(1),
			VUsMax:     null.IntFrom(1),
			Iterations: null.IntFrom(1),
		})
		if !assert.NoError(t, err) {
			return
		}

		c := &dummy.Collector{}
		e.Collectors = []lib.Collector{c}

		ctx, cancel := context.WithCancel(context.Background())
		errC := make(chan error)
		go func() { errC <- e.Run(ctx) }()
		<-signalChan
		cancel()
		assert.NoError(t, <-errC)

		found := 0
		for _, s := range c.Samples {
			if s.Metric != testMetric {
				continue
			}
			found++
			assert.Equal(t, 1.0, s.Value, "wrong value")
		}
		assert.Equal(t, 1, found, "wrong number of samples")
	})
}

func TestEngineAtTime(t *testing.T) {
	e, err := newTestEngine(nil, lib.Options{})
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	assert.NoError(t, e.Run(ctx))
}

func TestEngineCollector(t *testing.T) {
	testMetric := stats.New("test_metric", stats.Trend)

	e, err := newTestEngine(LF(func(ctx context.Context, out chan<- stats.SampleContainer) error {
		out <- stats.Sample{Metric: testMetric}
		return nil
	}), lib.Options{VUs: null.IntFrom(1), VUsMax: null.IntFrom(1), Iterations: null.IntFrom(1)})
	assert.NoError(t, err)

	c := &dummy.Collector{}
	e.Collectors = []lib.Collector{c}

	assert.NoError(t, e.Run(context.Background()))

	cSamples := []stats.Sample{}
	for _, sample := range c.Samples {
		if sample.Metric == testMetric {
			cSamples = append(cSamples, sample)
		}
	}
	metric := e.Metrics["test_metric"]
	if assert.NotNil(t, metric) {
		sink := metric.Sink.(*stats.TrendSink)
		if assert.NotNil(t, sink) {
			numCollectorSamples := len(cSamples)
			numEngineSamples := len(sink.Values)
			assert.Equal(t, numEngineSamples, numCollectorSamples)
		}
	}
}

func TestEngine_processSamples(t *testing.T) {
	metric := stats.New("my_metric", stats.Gauge)

	t.Run("metric", func(t *testing.T) {
		e, err := newTestEngine(nil, lib.Options{})
		assert.NoError(t, err)

		e.processSamples(
			[]stats.SampleContainer{stats.Sample{Metric: metric, Value: 1.25, Tags: stats.IntoSampleTags(&map[string]string{"a": "1"})}},
		)

		assert.IsType(t, &stats.GaugeSink{}, e.Metrics["my_metric"].Sink)
	})
	t.Run("submetric", func(t *testing.T) {
		ths, err := stats.NewThresholds([]string{`1+1==2`})
		assert.NoError(t, err)

		e, err := newTestEngine(nil, lib.Options{
			Thresholds: map[string]stats.Thresholds{
				"my_metric{a:1}": ths,
			},
		})
		assert.NoError(t, err)

		sms := e.submetrics["my_metric"]
		assert.Len(t, sms, 1)
		assert.Equal(t, "my_metric{a:1}", sms[0].Name)
		assert.EqualValues(t, map[string]string{"a": "1"}, sms[0].Tags.CloneTags())

		e.processSamples(
			[]stats.SampleContainer{stats.Sample{Metric: metric, Value: 1.25, Tags: stats.IntoSampleTags(&map[string]string{"a": "1", "b": "2"})}},
		)

		assert.IsType(t, &stats.GaugeSink{}, e.Metrics["my_metric"].Sink)
		assert.IsType(t, &stats.GaugeSink{}, e.Metrics["my_metric{a:1}"].Sink)
	})
}

func TestEngine_runThresholds(t *testing.T) {
	metric := stats.New("my_metric", stats.Gauge)
	thresholds := make(map[string]stats.Thresholds, 1)

	ths, err := stats.NewThresholds([]string{"1+1==3"})
	assert.NoError(t, err)

	t.Run("aborted", func(t *testing.T) {
		ths.Thresholds[0].AbortOnFail = true
		thresholds[metric.Name] = ths
		e, err := newTestEngine(nil, lib.Options{Thresholds: thresholds})
		assert.NoError(t, err)

		e.processSamples(
			[]stats.SampleContainer{stats.Sample{Metric: metric, Value: 1.25, Tags: stats.IntoSampleTags(&map[string]string{"a": "1"})}},
		)

		ctx, cancel := context.WithCancel(context.Background())
		aborted := false

		cancelFunc := func() {
			cancel()
			aborted = true
		}

		e.runThresholds(ctx, cancelFunc)

		assert.True(t, aborted)
	})

	t.Run("canceled", func(t *testing.T) {
		ths.Abort = false
		thresholds[metric.Name] = ths
		e, err := newTestEngine(nil, lib.Options{Thresholds: thresholds})
		assert.NoError(t, err)

		e.processSamples(
			[]stats.SampleContainer{stats.Sample{Metric: metric, Value: 1.25, Tags: stats.IntoSampleTags(&map[string]string{"a": "1"})}},
		)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		done := make(chan struct{})
		go func() {
			defer close(done)
			e.runThresholds(ctx, cancel)
		}()

		select {
		case <-done:
			return
		case <-time.After(1 * time.Second):
			assert.Fail(t, "Test should have completed within a second")
		}
	})
}

func TestEngine_processThresholds(t *testing.T) {
	metric := stats.New("my_metric", stats.Gauge)

	testdata := map[string]struct {
		pass  bool
		ths   map[string][]string
		abort bool
	}{
		"passing":  {true, map[string][]string{"my_metric": {"1+1==2"}}, false},
		"failing":  {false, map[string][]string{"my_metric": {"1+1==3"}}, false},
		"aborting": {false, map[string][]string{"my_metric": {"1+1==3"}}, true},

		"submetric,match,passing":   {true, map[string][]string{"my_metric{a:1}": {"1+1==2"}}, false},
		"submetric,match,failing":   {false, map[string][]string{"my_metric{a:1}": {"1+1==3"}}, false},
		"submetric,nomatch,passing": {true, map[string][]string{"my_metric{a:2}": {"1+1==2"}}, false},
		"submetric,nomatch,failing": {true, map[string][]string{"my_metric{a:2}": {"1+1==3"}}, false},
	}

	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			thresholds := make(map[string]stats.Thresholds, len(data.ths))
			for m, srcs := range data.ths {
				ths, err := stats.NewThresholds(srcs)
				assert.NoError(t, err)
				ths.Thresholds[0].AbortOnFail = data.abort
				thresholds[m] = ths
			}

			e, err := newTestEngine(nil, lib.Options{Thresholds: thresholds})
			assert.NoError(t, err)

			e.processSamples(
				[]stats.SampleContainer{stats.Sample{Metric: metric, Value: 1.25, Tags: stats.IntoSampleTags(&map[string]string{"a": "1"})}},
			)

			abortCalled := false

			abortFunc := func() {
				abortCalled = true
			}

			e.processThresholds(abortFunc)

			assert.Equal(t, data.pass, !e.IsTainted())
			if data.abort {
				assert.True(t, abortCalled)
			}
		})
	}
}

func getMetricSum(collector *dummy.Collector, name string) (result float64) {
	for _, sc := range collector.SampleContainers {
		for _, s := range sc.GetSamples() {
			if s.Metric.Name == name {
				result += s.Value
			}
		}
	}
	return
}
func getMetricCount(collector *dummy.Collector, name string) (result uint) {
	for _, sc := range collector.SampleContainers {
		for _, s := range sc.GetSamples() {
			if s.Metric.Name == name {
				result++
			}
		}
	}
	return
}

const expectedHeaderMaxLength = 500

// FIXME: This test is too brittle, consider simplifying.
func TestSentReceivedMetrics(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()
	tr := tb.Replacer.Replace

	type testScript struct {
		Code                 string
		NumRequests          int64
		ExpectedDataSent     int64
		ExpectedDataReceived int64
	}
	testScripts := []testScript{
		{tr(`import http from "k6/http";
			export default function() {
				http.get("HTTPBIN_URL/bytes/15000");
			}`), 1, 0, 15000},
		{tr(`import http from "k6/http";
			export default function() {
				http.get("HTTPBIN_URL/bytes/5000");
				http.get("HTTPSBIN_URL/bytes/5000");
				http.batch(["HTTPBIN_URL/bytes/10000", "HTTPBIN_URL/bytes/20000", "HTTPSBIN_URL/bytes/10000"]);
			}`), 5, 0, 50000},
		{tr(`import http from "k6/http";
			let data = "0123456789".repeat(100);
			export default function() {
				http.post("HTTPBIN_URL/ip", {
					file: http.file(data, "test.txt")
				});
			}`), 1, 1000, 100},
		// NOTE(imiric): This needs to keep testing against /ws-echo-invalid because
		// this test is highly sensitive to metric data, and slightly differing
		// WS server implementations might introduce flakiness.
		// See https://github.com/loadimpact/k6/pull/1149
		{tr(`import ws from "k6/ws";
			let data = "0123456789".repeat(100);
			export default function() {
				ws.connect("WSBIN_URL/ws-echo-invalid", null, function (socket) {
					socket.on('open', function open() {
						socket.send(data);
					});
					socket.on('message', function (message) {
						socket.close();
					});
				});
			}`), 2, 1000, 1000},
	}

	type testCase struct{ Iterations, VUs int64 }
	testCases := []testCase{
		{1, 1}, {1, 2}, {2, 1}, {5, 2}, {25, 2}, {50, 5},
	}

	runTest := func(t *testing.T, ts testScript, tc testCase, noConnReuse bool) (float64, float64) {
		r, err := js.New(
			&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: []byte(ts.Code)},
			nil,
			lib.RuntimeOptions{},
		)
		require.NoError(t, err)

		options := lib.Options{
			Iterations:            null.IntFrom(tc.Iterations),
			VUs:                   null.IntFrom(tc.VUs),
			VUsMax:                null.IntFrom(tc.VUs),
			Hosts:                 tb.Dialer.Hosts,
			InsecureSkipTLSVerify: null.BoolFrom(true),
			NoVUConnectionReuse:   null.BoolFrom(noConnReuse),
		}

		r.SetOptions(options)
		engine, err := NewEngine(local.New(r), options)
		require.NoError(t, err)

		collector := &dummy.Collector{}
		engine.Collectors = []lib.Collector{collector}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errC := make(chan error)
		go func() { errC <- engine.Run(ctx) }()

		select {
		case <-time.After(10 * time.Second):
			t.Fatal("Test timed out")
		case err := <-errC:
			require.NoError(t, err)
		}

		checkData := func(name string, expected int64) float64 {
			data := getMetricSum(collector, name)
			expectedDataMin := float64(expected * tc.Iterations)
			expectedDataMax := float64((expected + ts.NumRequests*expectedHeaderMaxLength) * tc.Iterations)

			if data < expectedDataMin || data > expectedDataMax {
				t.Errorf(
					"The %s sum should be in the interval [%f, %f] but was %f",
					name, expectedDataMin, expectedDataMax, data,
				)
			}
			return data
		}

		return checkData(metrics.DataSent.Name, ts.ExpectedDataSent),
			checkData(metrics.DataReceived.Name, ts.ExpectedDataReceived)
	}

	getTestCase := func(t *testing.T, ts testScript, tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()
			noReuseSent, noReuseReceived := runTest(t, ts, tc, true)
			reuseSent, reuseReceived := runTest(t, ts, tc, false)

			if noReuseSent < reuseSent {
				t.Errorf("reuseSent=%f is greater than noReuseSent=%f", reuseSent, noReuseSent)
			}
			if noReuseReceived < reuseReceived {
				t.Errorf("reuseReceived=%f is greater than noReuseReceived=%f", reuseReceived, noReuseReceived)
			}
		}
	}

	// This Run will not return until the parallel subtests complete.
	t.Run("group", func(t *testing.T) {
		for tsNum, ts := range testScripts {
			for tcNum, tc := range testCases {
				t.Run(
					fmt.Sprintf("SentReceivedMetrics_script[%d]_case[%d](%d,%d)", tsNum, tcNum, tc.Iterations, tc.VUs),
					getTestCase(t, ts, tc),
				)
			}
		}
	})
}

func TestRunTags(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	runTagsMap := map[string]string{"foo": "bar", "test": "mest", "over": "written"}
	runTags := stats.NewSampleTags(runTagsMap)

	script := []byte(tb.Replacer.Replace(`
		import http from "k6/http";
		import ws from "k6/ws";
		import { Counter } from "k6/metrics";
		import { group, check, fail } from "k6";

		let customTags =  { "over": "the rainbow" };
		let params = { "tags": customTags};
		let statusCheck = { "status is 200": (r) => r.status === 200 }

		let myCounter = new Counter("mycounter");

		export default function() {

			group("http", function() {
				check(http.get("HTTPSBIN_URL", params), statusCheck, customTags);
				check(http.get("HTTPBIN_URL/status/418", params), statusCheck, customTags);
			})

			group("websockets", function() {
				var response = ws.connect("WSBIN_URL/ws-echo", params, function (socket) {
					socket.on('open', function open() {
						console.log('ws open and say hello');
						socket.send("hello");
					});

					socket.on('message', function (message) {
						console.log('ws got message ' + message);
						if (message != "hello") {
							fail("Expected to receive 'hello' but got '" + message + "' instead !");
						}
						console.log('ws closing socket...');
						socket.close();
					});

					socket.on('close', function () {
						console.log('ws close');
					});

					socket.on('error', function (e) {
						console.log('ws error: ' + e.error());
					});
				});
				console.log('connect returned');
				check(response, { "status is 101": (r) => r && r.status === 101 }, customTags);
			})

			myCounter.add(1, customTags);
		}
	`))

	r, err := js.New(
		&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: script},
		nil,
		lib.RuntimeOptions{},
	)
	require.NoError(t, err)

	options := lib.Options{
		Iterations:            null.IntFrom(3),
		VUs:                   null.IntFrom(2),
		VUsMax:                null.IntFrom(2),
		Hosts:                 tb.Dialer.Hosts,
		RunTags:               runTags,
		SystemTags:            &stats.DefaultSystemTagSet,
		InsecureSkipTLSVerify: null.BoolFrom(true),
	}

	r.SetOptions(options)
	engine, err := NewEngine(local.New(r), options)
	require.NoError(t, err)

	collector := &dummy.Collector{}
	engine.Collectors = []lib.Collector{collector}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errC := make(chan error)
	go func() { errC <- engine.Run(ctx) }()

	select {
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out")
	case err := <-errC:
		require.NoError(t, err)
	}

	systemMetrics := []*stats.Metric{
		metrics.VUs, metrics.VUsMax, metrics.Iterations, metrics.IterationDuration,
		metrics.GroupDuration, metrics.DataSent, metrics.DataReceived,
	}

	getExpectedOverVal := func(metricName string) string {
		for _, sysMetric := range systemMetrics {
			if sysMetric.Name == metricName {
				return runTagsMap["over"]
			}
		}
		return "the rainbow"
	}

	for _, s := range collector.Samples {
		for key, expVal := range runTagsMap {
			val, ok := s.Tags.Get(key)

			if key == "over" {
				expVal = getExpectedOverVal(s.Metric.Name)
			}

			assert.True(t, ok)
			assert.Equalf(t, expVal, val, "Wrong tag value in sample for metric %#v", s.Metric)
		}
	}
}

func TestSetupTeardownThresholds(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	script := []byte(tb.Replacer.Replace(`
		import http from "k6/http";
		import { check } from "k6";
		import { Counter } from "k6/metrics";

		let statusCheck = { "status is 200": (r) => r.status === 200 }
		let myCounter = new Counter("setup_teardown");

		export let options = {
			iterations: 5,
			thresholds: {
				"setup_teardown": ["count == 2"],
				"iterations": ["count == 5"],
				"http_reqs": ["count == 7"],
			},
		};

		export function setup() {
			check(http.get("HTTPBIN_IP_URL"), statusCheck) && myCounter.add(1);
		};

		export default function () {
			check(http.get("HTTPBIN_IP_URL"), statusCheck);
		};

		export function teardown() {
			check(http.get("HTTPBIN_IP_URL"), statusCheck) && myCounter.add(1);
		};
	`))

	runner, err := js.New(
		&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: script},
		nil,
		lib.RuntimeOptions{},
	)
	require.NoError(t, err)
	runner.SetOptions(runner.GetOptions().Apply(lib.Options{
		SystemTags:      &stats.DefaultSystemTagSet,
		SetupTimeout:    types.NullDurationFrom(3 * time.Second),
		TeardownTimeout: types.NullDurationFrom(3 * time.Second),
		VUs:             null.IntFrom(3),
		VUsMax:          null.IntFrom(3),
	}))

	engine, err := NewEngine(local.New(runner), runner.GetOptions())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errC := make(chan error)
	go func() { errC <- engine.Run(ctx) }()

	select {
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out")
	case err := <-errC:
		require.NoError(t, err)
		require.False(t, engine.IsTainted())
	}
}

func TestEmittedMetricsWhenScalingDown(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	script := []byte(tb.Replacer.Replace(`
		import http from "k6/http";
		import { sleep } from "k6";

		export let options = {
			systemTags: ["iter", "vu", "url"],

			// Start with 2 VUs for 4 seconds and then quickly scale down to 1 for the next 4s and then quit
			vus: 2,
			vusMax: 2,
			stages: [
				{ duration: "4s", target: 2 },
				{ duration: "1s", target: 1 },
				{ duration: "3s", target: 1 },
			],
		};

		export default function () {
			console.log("VU " + __VU + " starting iteration #" + __ITER);
			http.get("HTTPBIN_IP_URL/bytes/15000");
			sleep(3.1);
			http.get("HTTPBIN_IP_URL/bytes/15000");
			console.log("VU " + __VU + " ending iteration #" + __ITER);
		};
	`))

	runner, err := js.New(
		&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: script},
		nil,
		lib.RuntimeOptions{},
	)
	require.NoError(t, err)

	engine, err := NewEngine(local.New(runner), runner.GetOptions())
	require.NoError(t, err)

	collector := &dummy.Collector{}
	engine.Collectors = []lib.Collector{collector}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errC := make(chan error)
	go func() { errC <- engine.Run(ctx) }()

	select {
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out")
	case err := <-errC:
		require.NoError(t, err)
		require.False(t, engine.IsTainted())
	}

	// The 1.7 sleep in the default function would cause the first VU to comlete 2 full iterations
	// and stat executing its third one, while the second VU will only fully complete 1 iteration
	// and will be canceled in the middle of its second one.
	assert.Equal(t, 3.0, getMetricSum(collector, metrics.Iterations.Name))

	// That means that we expect to see 8 HTTP requests in total, 3*2=6 from the complete iterations
	// and one each from the two iterations that would be canceled in the middle of their execution
	assert.Equal(t, 8.0, getMetricSum(collector, metrics.HTTPReqs.Name))

	// But we expect to only see the data_received for only 7 of those requests. The data for the 8th
	// request (the 3rd one in the first VU before the test ends) gets cut off by the engine because
	// it's emitted after the test officially ends
	dataReceivedExpectedMin := 15000.0 * 7
	dataReceivedExpectedMax := (15000.0 + expectedHeaderMaxLength) * 7
	dataReceivedActual := getMetricSum(collector, metrics.DataReceived.Name)
	if dataReceivedActual < dataReceivedExpectedMin || dataReceivedActual > dataReceivedExpectedMax {
		t.Errorf(
			"The data_received sum should be in the interval [%f, %f] but was %f",
			dataReceivedExpectedMin, dataReceivedExpectedMax, dataReceivedActual,
		)
	}

	// Also, the interrupted iterations shouldn't affect the average iteration_duration in any way, only
	// complete iterations should be taken into account
	durationCount := float64(getMetricCount(collector, metrics.IterationDuration.Name))
	assert.Equal(t, 3.0, durationCount)
	durationSum := getMetricSum(collector, metrics.IterationDuration.Name)
	assert.InDelta(t, 3.35, durationSum/(1000*durationCount), 0.25)
}

func TestMinIterationDuration(t *testing.T) {
	t.Parallel()

	runner, err := js.New(
		&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: []byte(`
		import { Counter } from "k6/metrics";

		let testCounter = new Counter("testcounter");

		export let options = {
			minIterationDuration: "1s",
			vus: 2,
			vusMax: 2,
			duration: "1.9s",
		};

		export default function () {
			testCounter.add(1);
		};`)},
		nil,
		lib.RuntimeOptions{},
	)
	require.NoError(t, err)

	engine, err := NewEngine(local.New(runner), runner.GetOptions())
	require.NoError(t, err)

	collector := &dummy.Collector{}
	engine.Collectors = []lib.Collector{collector}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errC := make(chan error)
	go func() { errC <- engine.Run(ctx) }()

	select {
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out")
	case err := <-errC:
		require.NoError(t, err)
		require.False(t, engine.IsTainted())
	}

	// Only 2 full iterations are expected to be completed due to the 1 second minIterationDuration
	assert.Equal(t, 2.0, getMetricSum(collector, metrics.Iterations.Name))

	// But we expect the custom counter to be added to 4 times
	assert.Equal(t, 4.0, getMetricSum(collector, "testcounter"))
}

//nolint: funlen
func TestMinIterationDurationInSetupTeardownStage(t *testing.T) {
	t.Parallel()
	setupScript := `
		import { sleep } from "k6";

		export function setup() {
			sleep(1);
		}

		export let options = {
			minIterationDuration: "2s",
			duration: "2s",
			setupTimeout: "2s",
		};

		export default function () {
		};`
	teardownScript := `
		import { sleep } from "k6";

		export let options = {
			minIterationDuration: "2s",
			duration: "2s",
			teardownTimeout: "2s",
		};

		export default function () {
		};

		export function teardown() {
			sleep(1);
		}
`
	tests := []struct {
		name, script string
	}{
		{"Test setup", setupScript},
		{"Test teardown", teardownScript},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			runner, err := js.New(
				&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: []byte(tc.script)},
				nil,
				lib.RuntimeOptions{},
			)
			require.NoError(t, err)
			engine, err := NewEngine(local.New(runner), runner.GetOptions())
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			errC := make(chan error)
			go func() { errC <- engine.Run(ctx) }()
			select {
			case <-time.After(10 * time.Second):
				t.Fatal("Test timed out")
			case err := <-errC:
				require.NoError(t, err)
				require.False(t, engine.IsTainted())
			}
		})
	}
}
