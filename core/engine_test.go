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
	"testing"
	"time"

	"github.com/loadimpact/k6/core/local"
	"github.com/loadimpact/k6/js"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/dummy"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"
)

type testErrorWithString string

func (e testErrorWithString) Error() string  { return string(e) }
func (e testErrorWithString) String() string { return string(e) }

// Apply a null logger to the engine and return the hook.
func applyNullLogger(e *Engine) *logtest.Hook {
	logger, hook := logtest.NewNullLogger()
	e.SetLogger(logger)
	return hook
}

// Wrapper around newEngine that applies a null logger.
func newTestEngine(ex lib.Executor, opts lib.Options) (*Engine, error, *logtest.Hook) {
	e, err := NewEngine(ex, opts)
	if err != nil {
		return e, err, nil
	}
	hook := applyNullLogger(e)
	return e, nil, hook
}

func L(r lib.Runner) lib.Executor {
	return local.New(r)
}

func LF(fn func(ctx context.Context) ([]stats.SampleContainer, error)) lib.Executor {
	return L(&lib.MiniRunner{Fn: fn})
}

func TestNewEngine(t *testing.T) {
	_, err, _ := newTestEngine(nil, lib.Options{})
	assert.NoError(t, err)
}

func TestNewEngineOptions(t *testing.T) {
	t.Run("Duration", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, lib.Options{
			Duration: types.NullDurationFrom(10 * time.Second),
		})
		assert.NoError(t, err)
		assert.Nil(t, e.Executor.GetStages())
		assert.Equal(t, types.NullDurationFrom(10*time.Second), e.Executor.GetEndTime())

		t.Run("Infinite", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, lib.Options{Duration: types.NullDuration{}})
			assert.NoError(t, err)
			assert.Nil(t, e.Executor.GetStages())
			assert.Equal(t, types.NullDuration{}, e.Executor.GetEndTime())
		})
	})
	t.Run("Stages", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, lib.Options{
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
		e, err, _ := newTestEngine(nil, lib.Options{
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
		e, err, _ := newTestEngine(nil, lib.Options{Iterations: null.IntFrom(100)})
		assert.NoError(t, err)
		assert.Equal(t, null.IntFrom(100), e.Executor.GetEndIterations())
	})
	t.Run("VUsMax", func(t *testing.T) {
		t.Run("not set", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, lib.Options{})
			assert.NoError(t, err)
			assert.Equal(t, int64(0), e.Executor.GetVUsMax())
			assert.Equal(t, int64(0), e.Executor.GetVUs())
		})
		t.Run("set", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, lib.Options{
				VUsMax: null.IntFrom(10),
			})
			assert.NoError(t, err)
			assert.Equal(t, int64(10), e.Executor.GetVUsMax())
			assert.Equal(t, int64(0), e.Executor.GetVUs())
		})
	})
	t.Run("VUs", func(t *testing.T) {
		t.Run("no max", func(t *testing.T) {
			_, err, _ := newTestEngine(nil, lib.Options{
				VUs: null.IntFrom(10),
			})
			assert.EqualError(t, err, "can't raise vu count (to 10) above vu cap (0)")
		})
		t.Run("negative max", func(t *testing.T) {
			_, err, _ := newTestEngine(nil, lib.Options{
				VUsMax: null.IntFrom(-1),
			})
			assert.EqualError(t, err, "vu cap can't be negative")
		})
		t.Run("max too low", func(t *testing.T) {
			_, err, _ := newTestEngine(nil, lib.Options{
				VUsMax: null.IntFrom(1),
				VUs:    null.IntFrom(10),
			})
			assert.EqualError(t, err, "can't raise vu count (to 10) above vu cap (1)")
		})
		t.Run("max higher", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, lib.Options{
				VUsMax: null.IntFrom(10),
				VUs:    null.IntFrom(1),
			})
			assert.NoError(t, err)
			assert.Equal(t, int64(10), e.Executor.GetVUsMax())
			assert.Equal(t, int64(1), e.Executor.GetVUs())
		})
		t.Run("max just right", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, lib.Options{
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
			e, err, _ := newTestEngine(nil, lib.Options{})
			assert.NoError(t, err)
			assert.False(t, e.Executor.IsPaused())
		})
		t.Run("false", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, lib.Options{
				Paused: null.BoolFrom(false),
			})
			assert.NoError(t, err)
			assert.False(t, e.Executor.IsPaused())
		})
		t.Run("true", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, lib.Options{
				Paused: null.BoolFrom(true),
			})
			assert.NoError(t, err)
			assert.True(t, e.Executor.IsPaused())
		})
	})
	t.Run("thresholds", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, lib.Options{
			Thresholds: map[string]stats.Thresholds{
				"my_metric": {},
			},
		})
		assert.NoError(t, err)
		assert.Contains(t, e.thresholds, "my_metric")

		t.Run("submetrics", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, lib.Options{
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
	log.SetLevel(log.DebugLevel)
	t.Run("exits with context", func(t *testing.T) {
		duration := 100 * time.Millisecond
		e, err, _ := newTestEngine(nil, lib.Options{})
		assert.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), duration)
		defer cancel()
		startTime := time.Now()
		assert.NoError(t, e.Run(ctx))
		assert.WithinDuration(t, startTime.Add(duration), time.Now(), 100*time.Millisecond)
	})
	t.Run("exits with executor", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, lib.Options{
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
		e, err, _ := newTestEngine(LF(func(ctx context.Context) (samples []stats.SampleContainer, err error) {
			samples = append(samples, stats.Sample{Metric: testMetric, Time: time.Now(), Value: 1})
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
			samples = append(samples, stats.Sample{Metric: testMetric, Time: time.Now(), Value: 2})
			return samples, err
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
	e, err, _ := newTestEngine(nil, lib.Options{})
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	assert.NoError(t, e.Run(ctx))
}

func TestEngineCollector(t *testing.T) {
	testMetric := stats.New("test_metric", stats.Trend)

	e, err, _ := newTestEngine(LF(func(ctx context.Context) ([]stats.SampleContainer, error) {
		return []stats.SampleContainer{stats.Sample{Metric: testMetric}}, nil
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
		e, err, _ := newTestEngine(nil, lib.Options{})
		assert.NoError(t, err)

		e.processSamples(
			stats.Sample{Metric: metric, Value: 1.25, Tags: stats.IntoSampleTags(&map[string]string{"a": "1"})},
		)

		assert.IsType(t, &stats.GaugeSink{}, e.Metrics["my_metric"].Sink)
	})
	t.Run("submetric", func(t *testing.T) {
		ths, err := stats.NewThresholds([]string{`1+1==2`})
		assert.NoError(t, err)

		e, err, _ := newTestEngine(nil, lib.Options{
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
			stats.Sample{Metric: metric, Value: 1.25, Tags: stats.IntoSampleTags(&map[string]string{"a": "1", "b": "2"})},
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
		e, err, _ := newTestEngine(nil, lib.Options{Thresholds: thresholds})
		assert.NoError(t, err)

		e.processSamples(
			stats.Sample{Metric: metric, Value: 1.25, Tags: stats.IntoSampleTags(&map[string]string{"a": "1"})},
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
		e, err, _ := newTestEngine(nil, lib.Options{Thresholds: thresholds})
		assert.NoError(t, err)

		e.processSamples(
			stats.Sample{Metric: metric, Value: 1.25, Tags: stats.IntoSampleTags(&map[string]string{"a": "1"})},
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

			e, err, _ := newTestEngine(nil, lib.Options{Thresholds: thresholds})
			assert.NoError(t, err)

			e.processSamples(
				stats.Sample{Metric: metric, Value: 1.25, Tags: stats.IntoSampleTags(&map[string]string{"a": "1"})},
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
func TestSentReceivedMetrics(t *testing.T) {
	t.Parallel()
	tb := testutils.NewHTTPMultiBin(t)
	defer tb.Cleanup()
	tr := tb.Replacer.Replace

	const expectedHeaderMaxLength = 500

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
		{tr(`import ws from "k6/ws";
			let data = "0123456789".repeat(100);
			export default function() {
				ws.connect("ws://HTTPBIN_IP:HTTPBIN_PORT/ws-echo", null, function (socket) {
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
			&lib.SourceData{Filename: "/script.js", Data: []byte(ts.Code)},
			afero.NewMemMapFs(),
			lib.RuntimeOptions{},
		)
		require.NoError(t, err)

		options := lib.Options{
			Iterations: null.IntFrom(tc.Iterations),
			VUs:        null.IntFrom(tc.VUs),
			VUsMax:     null.IntFrom(tc.VUs),
			Hosts:      tb.Dialer.Hosts,
			InsecureSkipTLSVerify: null.BoolFrom(true),
			NoVUConnectionReuse:   null.BoolFrom(noConnReuse),
		}

		r.SetOptions(options)
		engine, err := NewEngine(local.New(r), options)
		require.NoError(t, err)

		collector := &dummy.Collector{}
		engine.Collectors = []lib.Collector{collector}

		ctx, cancel := context.WithCancel(context.Background())
		errC := make(chan error)
		go func() { errC <- engine.Run(ctx) }()

		select {
		case <-time.After(10 * time.Second):
			cancel()
			t.Fatal("Test timed out")
		case err := <-errC:
			cancel()
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
				t.Errorf("noReuseSent=%f is greater than reuseSent=%f", noReuseSent, reuseSent)
			}
			if noReuseReceived < reuseReceived {
				t.Errorf("noReuseReceived=%f is greater than reuseReceived=%f", noReuseReceived, reuseReceived)
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
	tb := testutils.NewHTTPMultiBin(t)
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
				var response = ws.connect("wss://HTTPSBIN_IP:HTTPSBIN_PORT/ws-echo", params, function (socket) {
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
		&lib.SourceData{Filename: "/script.js", Data: script},
		afero.NewMemMapFs(),
		lib.RuntimeOptions{},
	)
	require.NoError(t, err)

	options := lib.Options{
		Iterations:            null.IntFrom(3),
		VUs:                   null.IntFrom(2),
		VUsMax:                null.IntFrom(2),
		Hosts:                 tb.Dialer.Hosts,
		RunTags:               runTags,
		SystemTags:            lib.GetTagSet(lib.DefaultSystemTagList...),
		InsecureSkipTLSVerify: null.BoolFrom(true),
	}

	r.SetOptions(options)
	engine, err := NewEngine(local.New(r), options)
	require.NoError(t, err)

	collector := &dummy.Collector{}
	engine.Collectors = []lib.Collector{collector}

	ctx, cancel := context.WithCancel(context.Background())
	errC := make(chan error)
	go func() { errC <- engine.Run(ctx) }()

	select {
	case <-time.After(10 * time.Second):
		cancel()
		t.Fatal("Test timed out")
	case err := <-errC:
		cancel()
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
