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
	"runtime"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/core/local"
	"github.com/loadimpact/k6/js"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/executor"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/testutils/httpmultibin"
	"github.com/loadimpact/k6/lib/testutils/minirunner"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/dummy"
)

const isWindows = runtime.GOOS == "windows"

// Wrapper around NewEngine that applies a logger and manages the options.
func newTestEngine( //nolint:golint
	t *testing.T, runCtx context.Context, runner lib.Runner, collectors []lib.Collector, opts lib.Options,
) (engine *Engine, run func() error, wait func()) {
	if runner == nil {
		runner = &minirunner.MiniRunner{}
	}
	globalCtx, globalCancel := context.WithCancel(context.Background())
	var runCancel func()
	if runCtx == nil {
		runCtx, runCancel = context.WithCancel(globalCtx)
	}

	newOpts, err := executor.DeriveScenariosFromShortcuts(lib.Options{
		MetricSamplesBufferSize: null.NewInt(200, false),
	}.Apply(runner.GetOptions()).Apply(opts))
	require.NoError(t, err)
	require.Empty(t, newOpts.Validate())

	require.NoError(t, runner.SetOptions(newOpts))

	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))

	execScheduler, err := local.NewExecutionScheduler(runner, logger)
	require.NoError(t, err)

	engine, err = NewEngine(execScheduler, opts, logger)
	require.NoError(t, err)

	engine.Collectors = collectors

	run, waitFn, err := engine.Init(globalCtx, runCtx)
	require.NoError(t, err)

	return engine, run, func() {
		if runCancel != nil {
			runCancel()
		}
		globalCancel()
		waitFn()
	}
}

func TestNewEngine(t *testing.T) {
	newTestEngine(t, nil, nil, nil, lib.Options{})
}

func TestEngineRun(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	t.Run("exits with context", func(t *testing.T) {
		done := make(chan struct{})
		runner := &minirunner.MiniRunner{Fn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
			<-ctx.Done()
			close(done)
			return nil
		}}

		duration := 100 * time.Millisecond
		ctx, cancel := context.WithTimeout(context.Background(), duration)
		defer cancel()

		_, run, wait := newTestEngine(t, ctx, runner, nil, lib.Options{})
		defer wait()

		startTime := time.Now()
		assert.NoError(t, run())
		assert.WithinDuration(t, startTime.Add(duration), time.Now(), 100*time.Millisecond)
		<-done
	})
	t.Run("exits with executor", func(t *testing.T) {
		e, run, wait := newTestEngine(t, nil, nil, nil, lib.Options{
			VUs:        null.IntFrom(10),
			Iterations: null.IntFrom(100),
		})
		defer wait()
		assert.NoError(t, run())
		assert.Equal(t, uint64(100), e.ExecutionScheduler.GetState().GetFullIterationCount())
	})
	// Make sure samples are discarded after context close (using "cutoff" timestamp in local.go)
	t.Run("collects samples", func(t *testing.T) {
		testMetric := stats.New("test_metric", stats.Trend)

		signalChan := make(chan interface{})

		runner := &minirunner.MiniRunner{Fn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
			stats.PushIfNotDone(ctx, out, stats.Sample{Metric: testMetric, Time: time.Now(), Value: 1})
			close(signalChan)
			<-ctx.Done()
			stats.PushIfNotDone(ctx, out, stats.Sample{Metric: testMetric, Time: time.Now(), Value: 1})
			return nil
		}}

		c := &dummy.Collector{}

		ctx, cancel := context.WithCancel(context.Background())
		_, run, wait := newTestEngine(t, ctx, runner, []lib.Collector{c}, lib.Options{
			VUs:        null.IntFrom(1),
			Iterations: null.IntFrom(1),
		})

		errC := make(chan error)
		go func() { errC <- run() }()
		<-signalChan
		cancel()
		assert.NoError(t, <-errC)
		wait()

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
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, run, wait := newTestEngine(t, ctx, nil, nil, lib.Options{
		VUs:      null.IntFrom(2),
		Duration: types.NullDurationFrom(20 * time.Second),
	})
	defer wait()

	assert.NoError(t, run())
}

func TestEngineStopped(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	e, run, wait := newTestEngine(t, ctx, nil, nil, lib.Options{
		VUs:      null.IntFrom(1),
		Duration: types.NullDurationFrom(20 * time.Second),
	})
	defer wait()

	assert.NoError(t, run())
	assert.Equal(t, false, e.IsStopped(), "engine should be running")
	e.Stop()
	assert.Equal(t, true, e.IsStopped(), "engine should be stopped")
	e.Stop() // test that a second stop doesn't panic
}

func TestEngineCollector(t *testing.T) {
	testMetric := stats.New("test_metric", stats.Trend)

	runner := &minirunner.MiniRunner{Fn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
		out <- stats.Sample{Metric: testMetric}
		return nil
	}}

	c := &dummy.Collector{}
	e, run, wait := newTestEngine(t, nil, runner, []lib.Collector{c}, lib.Options{
		VUs:        null.IntFrom(1),
		Iterations: null.IntFrom(1),
	})

	assert.NoError(t, run())
	wait()

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
		e, _, wait := newTestEngine(t, nil, nil, nil, lib.Options{})
		defer wait()

		e.processSamples(
			[]stats.SampleContainer{stats.Sample{Metric: metric, Value: 1.25, Tags: stats.IntoSampleTags(&map[string]string{"a": "1"})}},
		)

		assert.IsType(t, &stats.GaugeSink{}, e.Metrics["my_metric"].Sink)
	})
	t.Run("submetric", func(t *testing.T) {
		ths, err := stats.NewThresholds([]string{`1+1==2`})
		assert.NoError(t, err)

		e, _, wait := newTestEngine(t, nil, nil, nil, lib.Options{
			Thresholds: map[string]stats.Thresholds{
				"my_metric{a:1}": ths,
			},
		})
		defer wait()

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

func TestEngineThresholdsWillAbort(t *testing.T) {
	metric := stats.New("my_metric", stats.Gauge)

	ths, err := stats.NewThresholds([]string{"1+1==3"})
	assert.NoError(t, err)
	ths.Thresholds[0].AbortOnFail = true

	thresholds := map[string]stats.Thresholds{metric.Name: ths}

	e, _, wait := newTestEngine(t, nil, nil, nil, lib.Options{Thresholds: thresholds})
	defer wait()

	e.processSamples(
		[]stats.SampleContainer{stats.Sample{Metric: metric, Value: 1.25, Tags: stats.IntoSampleTags(&map[string]string{"a": "1"})}},
	)
	assert.True(t, e.processThresholds())
}

func TestEngineAbortedByThresholds(t *testing.T) {
	metric := stats.New("my_metric", stats.Gauge)

	ths, err := stats.NewThresholds([]string{"1+1==3"})
	assert.NoError(t, err)
	ths.Thresholds[0].AbortOnFail = true

	thresholds := map[string]stats.Thresholds{metric.Name: ths}

	done := make(chan struct{})
	runner := &minirunner.MiniRunner{Fn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
		out <- stats.Sample{Metric: metric, Value: 1.25, Tags: stats.IntoSampleTags(&map[string]string{"a": "1"})}
		<-ctx.Done()
		close(done)
		return nil
	}}

	_, run, wait := newTestEngine(t, nil, runner, nil, lib.Options{Thresholds: thresholds})
	defer wait()

	go func() {
		assert.NoError(t, run())
	}()

	select {
	case <-done:
		return
	case <-time.After(10 * time.Second):
		assert.Fail(t, "Test should have completed within 10 seconds")
	}
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
		name, data := name, data
		t.Run(name, func(t *testing.T) {
			thresholds := make(map[string]stats.Thresholds, len(data.ths))
			for m, srcs := range data.ths {
				ths, err := stats.NewThresholds(srcs)
				assert.NoError(t, err)
				ths.Thresholds[0].AbortOnFail = data.abort
				thresholds[m] = ths
			}

			e, _, wait := newTestEngine(t, nil, nil, nil, lib.Options{Thresholds: thresholds})
			defer wait()

			e.processSamples(
				[]stats.SampleContainer{stats.Sample{Metric: metric, Value: 1.25, Tags: stats.IntoSampleTags(&map[string]string{"a": "1"})}},
			)

			assert.Equal(t, data.abort, e.processThresholds())
			assert.Equal(t, data.pass, !e.IsTainted())
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
		{1, 1}, {2, 2}, {2, 1}, {5, 2}, {25, 2}, {50, 5},
	}

	runTest := func(t *testing.T, ts testScript, tc testCase, noConnReuse bool) (float64, float64) {
		r, err := js.New(
			testutils.NewLogger(t),
			&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: []byte(ts.Code)},
			nil,
			lib.RuntimeOptions{},
		)
		require.NoError(t, err)

		collector := &dummy.Collector{}
		_, run, wait := newTestEngine(t, nil, r, []lib.Collector{collector}, lib.Options{
			Iterations:            null.IntFrom(tc.Iterations),
			VUs:                   null.IntFrom(tc.VUs),
			Hosts:                 tb.Dialer.Hosts,
			InsecureSkipTLSVerify: null.BoolFrom(true),
			NoVUConnectionReuse:   null.BoolFrom(noConnReuse),
			Batch:                 null.IntFrom(20),
		})

		errC := make(chan error)
		go func() { errC <- run() }()

		select {
		case <-time.After(10 * time.Second):
			t.Fatal("Test timed out")
		case err := <-errC:
			require.NoError(t, err)
		}
		wait()

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
		testutils.NewLogger(t),
		&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: script},
		nil,
		lib.RuntimeOptions{},
	)
	require.NoError(t, err)

	collector := &dummy.Collector{}
	_, run, wait := newTestEngine(t, nil, r, []lib.Collector{collector}, lib.Options{
		Iterations:            null.IntFrom(3),
		VUs:                   null.IntFrom(2),
		Hosts:                 tb.Dialer.Hosts,
		RunTags:               runTags,
		SystemTags:            &stats.DefaultSystemTagSet,
		InsecureSkipTLSVerify: null.BoolFrom(true),
	})

	errC := make(chan error)
	go func() { errC <- run() }()

	select {
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out")
	case err := <-errC:
		require.NoError(t, err)
	}
	wait()

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
		testutils.NewLogger(t),
		&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: script},
		nil,
		lib.RuntimeOptions{},
	)
	require.NoError(t, err)

	engine, run, wait := newTestEngine(t, nil, runner, nil, lib.Options{
		SystemTags:      &stats.DefaultSystemTagSet,
		SetupTimeout:    types.NullDurationFrom(3 * time.Second),
		TeardownTimeout: types.NullDurationFrom(3 * time.Second),
		VUs:             null.IntFrom(3),
	})
	defer wait()

	errC := make(chan error)
	go func() { errC <- run() }()

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
			scenarios: {
				we_need_hard_stop_and_ramp_down: {
					executor: "ramping-vus",
					// Start with 2 VUs for 4 seconds and then quickly scale down to 1 for the next 4s and then quit
					startVUs: 2,
					stages: [
						{ duration: "4s", target: 2 },
						{ duration: "0s", target: 1 },
						{ duration: "4s", target: 1 },
					],
					gracefulStop: "0s",
					gracefulRampDown: "0s",
				},
			},
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
		testutils.NewLogger(t),
		&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: script},
		nil,
		lib.RuntimeOptions{},
	)
	require.NoError(t, err)

	collector := &dummy.Collector{}
	engine, run, wait := newTestEngine(t, nil, runner, []lib.Collector{collector}, lib.Options{})

	errC := make(chan error)
	go func() { errC <- run() }()

	select {
	case <-time.After(12 * time.Second):
		t.Fatal("Test timed out")
	case err := <-errC:
		require.NoError(t, err)
		wait()
		require.False(t, engine.IsTainted())
	}

	// The 3.1 sleep in the default function would cause the first VU to complete 2 full iterations
	// and stat executing its third one, while the second VU will only fully complete 1 iteration
	// and will be canceled in the middle of its second one.
	assert.Equal(t, 3.0, getMetricSum(collector, metrics.Iterations.Name))

	// That means that we expect to see 8 HTTP requests in total, 3*2=6 from the complete iterations
	// and one each from the two iterations that would be canceled in the middle of their execution
	assert.Equal(t, 8.0, getMetricSum(collector, metrics.HTTPReqs.Name))

	// And we expect to see the data_received for all 8 of those requests. Previously, the data for
	// the 8th request (the 3rd one in the first VU before the test ends) was cut off by the engine
	// because it was emitted after the test officially ended. But that was mostly an unintended
	// consequence of the fact that those metrics were emitted only after an iteration ended when
	// it was interrupted.
	dataReceivedExpectedMin := 15000.0 * 8
	dataReceivedExpectedMax := (15000.0 + expectedHeaderMaxLength) * 8
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

func TestMetricsEmission(t *testing.T) {
	if !isWindows {
		t.Parallel()
	}

	testCases := []struct {
		method             string
		minIterDuration    string
		defaultBody        string
		expCount, expIters float64
	}{
		// Since emission of Iterations happens before the minIterationDuration
		// sleep is done, we expect to receive metrics for all executions of
		// the `default` function, despite of the lower overall duration setting.
		{"minIterationDuration", `"300ms"`, "testCounter.add(1);", 16.0, 16.0},
		// With the manual sleep method and no minIterationDuration, the last
		// `default` execution will be cutoff by the duration setting, so only
		// 3 sets of metrics are expected.
		{"sleepBeforeCounterAdd", "null", "sleep(0.3); testCounter.add(1); ", 12.0, 12.0},
		// The counter should be sent, but the last iteration will be incomplete
		{"sleepAfterCounterAdd", "null", "testCounter.add(1); sleep(0.3); ", 16.0, 12.0},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.method, func(t *testing.T) {
			if !isWindows {
				t.Parallel()
			}
			runner, err := js.New(
				testutils.NewLogger(t),
				&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: []byte(fmt.Sprintf(`
				import { sleep } from "k6";
				import { Counter } from "k6/metrics";

				let testCounter = new Counter("testcounter");

				export let options = {
					scenarios: {
						we_need_hard_stop: {
							executor: "constant-vus",
							vus: 4,
							duration: "1s",
							gracefulStop: "0s",
						},
					},
					minIterationDuration: %s,
				};

				export default function() {
					%s
				}
				`, tc.minIterDuration, tc.defaultBody))},
				nil,
				lib.RuntimeOptions{},
			)
			require.NoError(t, err)

			collector := &dummy.Collector{}
			engine, run, wait := newTestEngine(t, nil, runner, []lib.Collector{collector}, runner.GetOptions())

			errC := make(chan error)
			go func() { errC <- run() }()

			select {
			case <-time.After(10 * time.Second):
				t.Fatal("Test timed out")
			case err := <-errC:
				require.NoError(t, err)
				wait()
				require.False(t, engine.IsTainted())
			}

			assert.Equal(t, tc.expIters, getMetricSum(collector, metrics.Iterations.Name))
			assert.Equal(t, tc.expCount, getMetricSum(collector, "testcounter"))
		})
	}
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
			scenarios: {
				we_need_hard_stop: {
					executor: "constant-vus",
					vus: 2,
					duration: "1.9s",
					gracefulStop: "0s",
				},
			},
			setupTimeout: "3s",
		};

		export default function () {
		};`
	teardownScript := `
		import { sleep } from "k6";

		export let options = {
			minIterationDuration: "2s",
			scenarios: {
				we_need_hard_stop: {
					executor: "constant-vus",
					vus: 2,
					duration: "1.9s",
					gracefulStop: "0s",
				},
			},
			teardownTimeout: "3s",
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
				testutils.NewLogger(t),
				&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: []byte(tc.script)},
				nil,
				lib.RuntimeOptions{},
			)
			require.NoError(t, err)

			engine, run, wait := newTestEngine(t, nil, runner, nil, runner.GetOptions())

			errC := make(chan error)
			go func() { errC <- run() }()
			select {
			case <-time.After(10 * time.Second):
				t.Fatal("Test timed out")
			case err := <-errC:
				require.NoError(t, err)
				wait()
				require.False(t, engine.IsTainted())
			}
		})
	}
}

func TestEngineRunsTeardownEvenAfterTestRunIsAborted(t *testing.T) {
	testMetric := stats.New("teardown_metric", stats.Counter)

	ctx, cancel := context.WithCancel(context.Background())

	runner := &minirunner.MiniRunner{
		Fn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
			cancel() // we cancel the runCtx immediately after the test starts
			return nil
		},
		TeardownFn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
			out <- stats.Sample{Metric: testMetric, Value: 1}
			return nil
		},
	}

	c := &dummy.Collector{}
	_, run, wait := newTestEngine(t, ctx, runner, []lib.Collector{c}, lib.Options{
		VUs: null.IntFrom(1), Iterations: null.IntFrom(1),
	})

	assert.NoError(t, run())
	wait()

	var count float64
	for _, sample := range c.Samples {
		if sample.Metric == testMetric {
			count += sample.Value
		}
	}
	assert.Equal(t, 1.0, count)
}
