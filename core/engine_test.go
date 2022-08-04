package core

import (
	"context"
	"fmt"
	"net/url"
	"runtime"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/core/local"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/js"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/executor"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/testutils/httpmultibin"
	"go.k6.io/k6/lib/testutils/minirunner"
	"go.k6.io/k6/lib/testutils/mockoutput"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/loader"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
)

const isWindows = runtime.GOOS == "windows"

// TODO: completely rewrite all of these tests

type testStruct struct {
	engine    *Engine
	run       func() error
	runCancel func()
	wait      func()
	piState   *lib.TestPreInitState
}

func getTestPreInitState(tb testing.TB) *lib.TestPreInitState {
	reg := metrics.NewRegistry()
	return &lib.TestPreInitState{
		Logger:         testutils.NewLogger(tb),
		RuntimeOptions: lib.RuntimeOptions{},
		Registry:       reg,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(reg),
	}
}

func getTestRunState(
	tb testing.TB, piState *lib.TestPreInitState, options lib.Options, runner lib.Runner,
) *lib.TestRunState {
	require.Empty(tb, options.Validate())
	require.NoError(tb, runner.SetOptions(options))
	return &lib.TestRunState{
		TestPreInitState: piState,
		Options:          options,
		Runner:           runner,
	}
}

// Wrapper around NewEngine that applies a logger and manages the options.
func newTestEngineWithTestPreInitState( //nolint:golint
	t *testing.T, runTimeout *time.Duration, runner lib.Runner, outputs []output.Output,
	opts lib.Options, piState *lib.TestPreInitState,
) *testStruct {
	if runner == nil {
		runner = &minirunner.MiniRunner{}
	}

	newOpts, err := executor.DeriveScenariosFromShortcuts(lib.Options{
		MetricSamplesBufferSize: null.NewInt(200, false),
	}.Apply(runner.GetOptions()).Apply(opts), piState.Logger)
	require.NoError(t, err)

	testRunState := getTestRunState(t, piState, newOpts, runner)

	execScheduler, err := local.NewExecutionScheduler(testRunState)
	require.NoError(t, err)

	engine, err := NewEngine(testRunState, execScheduler, outputs)
	require.NoError(t, err)
	require.NoError(t, engine.OutputManager.StartOutputs())

	globalCtx, globalCancel := context.WithCancel(context.Background())
	var runCancel func()
	var runCtx context.Context
	if runTimeout != nil {
		runCtx, runCancel = context.WithTimeout(globalCtx, *runTimeout)
	} else {
		runCtx, runCancel = context.WithCancel(globalCtx)
	}
	run, waitFn, err := engine.Init(globalCtx, runCtx)
	require.NoError(t, err)

	var test *testStruct
	test = &testStruct{
		engine:    engine,
		run:       run,
		runCancel: runCancel,
		wait: func() {
			test.runCancel()
			globalCancel()
			waitFn()
			engine.OutputManager.StopOutputs()
		},
		piState: piState,
	}
	return test
}

func newTestEngine(
	t *testing.T, runTimeout *time.Duration, runner lib.Runner, outputs []output.Output, opts lib.Options,
) *testStruct {
	return newTestEngineWithTestPreInitState(t, runTimeout, runner, outputs, opts, getTestPreInitState(t))
}

func TestEngineRun(t *testing.T) {
	t.Parallel()
	logrus.SetLevel(logrus.DebugLevel)
	t.Run("exits with context", func(t *testing.T) {
		t.Parallel()
		done := make(chan struct{})
		runner := &minirunner.MiniRunner{
			Fn: func(ctx context.Context, _ *lib.State, _ chan<- metrics.SampleContainer) error {
				<-ctx.Done()
				close(done)
				return nil
			},
		}

		duration := 100 * time.Millisecond
		test := newTestEngine(t, &duration, runner, nil, lib.Options{})
		defer test.wait()

		startTime := time.Now()
		assert.NoError(t, test.run())
		assert.WithinDuration(t, startTime.Add(duration), time.Now(), 100*time.Millisecond)
		<-done
	})
	t.Run("exits with executor", func(t *testing.T) {
		t.Parallel()
		test := newTestEngine(t, nil, nil, nil, lib.Options{
			VUs:        null.IntFrom(10),
			Iterations: null.IntFrom(100),
		})
		defer test.wait()
		assert.NoError(t, test.run())
		assert.Equal(t, uint64(100), test.engine.ExecutionScheduler.GetState().GetFullIterationCount())
	})
	// Make sure samples are discarded after context close (using "cutoff" timestamp in local.go)
	t.Run("collects samples", func(t *testing.T) {
		t.Parallel()

		piState := getTestPreInitState(t)
		testMetric, err := piState.Registry.NewMetric("test_metric", metrics.Trend)
		require.NoError(t, err)

		signalChan := make(chan interface{})

		runner := &minirunner.MiniRunner{
			Fn: func(ctx context.Context, _ *lib.State, out chan<- metrics.SampleContainer) error {
				metrics.PushIfNotDone(ctx, out, metrics.Sample{Metric: testMetric, Time: time.Now(), Value: 1})
				close(signalChan)
				<-ctx.Done()
				metrics.PushIfNotDone(ctx, out, metrics.Sample{Metric: testMetric, Time: time.Now(), Value: 1})
				return nil
			},
		}

		mockOutput := mockoutput.New()
		test := newTestEngineWithTestPreInitState(t, nil, runner, []output.Output{mockOutput}, lib.Options{
			VUs:        null.IntFrom(1),
			Iterations: null.IntFrom(1),
		}, piState)

		errC := make(chan error)
		go func() { errC <- test.run() }()
		<-signalChan
		test.runCancel()
		assert.NoError(t, <-errC)
		test.wait()

		found := 0
		for _, s := range mockOutput.Samples {
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
	t.Parallel()
	test := newTestEngine(t, nil, nil, nil, lib.Options{
		VUs:      null.IntFrom(2),
		Duration: types.NullDurationFrom(20 * time.Second),
	})
	defer test.wait()

	assert.NoError(t, test.run())
}

func TestEngineStopped(t *testing.T) {
	t.Parallel()
	test := newTestEngine(t, nil, nil, nil, lib.Options{
		VUs:      null.IntFrom(1),
		Duration: types.NullDurationFrom(20 * time.Second),
	})
	defer test.wait()

	assert.NoError(t, test.run())
	assert.Equal(t, false, test.engine.IsStopped(), "engine should be running")
	test.engine.Stop()
	assert.Equal(t, true, test.engine.IsStopped(), "engine should be stopped")
	test.engine.Stop() // test that a second stop doesn't panic
}

func TestEngineOutput(t *testing.T) {
	t.Parallel()

	piState := getTestPreInitState(t)
	testMetric, err := piState.Registry.NewMetric("test_metric", metrics.Trend)
	require.NoError(t, err)

	runner := &minirunner.MiniRunner{
		Fn: func(_ context.Context, _ *lib.State, out chan<- metrics.SampleContainer) error {
			out <- metrics.Sample{Metric: testMetric}
			return nil
		},
	}

	mockOutput := mockoutput.New()
	test := newTestEngineWithTestPreInitState(t, nil, runner, []output.Output{mockOutput}, lib.Options{
		VUs:        null.IntFrom(1),
		Iterations: null.IntFrom(1),
	}, piState)

	assert.NoError(t, test.run())
	test.wait()

	cSamples := []metrics.Sample{}
	for _, sample := range mockOutput.Samples {
		if sample.Metric == testMetric {
			cSamples = append(cSamples, sample)
		}
	}
	metric := test.engine.MetricsEngine.ObservedMetrics["test_metric"]
	if assert.NotNil(t, metric) {
		sink := metric.Sink.(*metrics.TrendSink) //nolint:forcetypeassert
		if assert.NotNil(t, sink) {
			numOutputSamples := len(cSamples)
			numEngineSamples := len(sink.Values)
			assert.Equal(t, numEngineSamples, numOutputSamples)
		}
	}
}

func TestEngine_processSamples(t *testing.T) {
	t.Parallel()

	t.Run("metric", func(t *testing.T) {
		t.Parallel()

		piState := getTestPreInitState(t)
		metric, err := piState.Registry.NewMetric("my_metric", metrics.Gauge)
		require.NoError(t, err)

		done := make(chan struct{})
		runner := &minirunner.MiniRunner{
			Fn: func(ctx context.Context, _ *lib.State, out chan<- metrics.SampleContainer) error {
				out <- metrics.Sample{Metric: metric, Value: 1.25, Tags: metrics.IntoSampleTags(&map[string]string{"a": "1"})}
				close(done)
				return nil
			},
		}
		test := newTestEngineWithTestPreInitState(t, nil, runner, nil, lib.Options{}, piState)

		go func() {
			assert.NoError(t, test.run())
		}()

		select {
		case <-done:
			return
		case <-time.After(10 * time.Second):
			assert.Fail(t, "Test should have completed within 10 seconds")
		}

		test.wait()

		assert.IsType(t, &metrics.GaugeSink{}, test.engine.MetricsEngine.ObservedMetrics["my_metric"].Sink)
	})
	t.Run("submetric", func(t *testing.T) {
		t.Parallel()

		piState := getTestPreInitState(t)
		metric, err := piState.Registry.NewMetric("my_metric", metrics.Gauge)
		require.NoError(t, err)

		ths := metrics.NewThresholds([]string{`value<2`})
		gotParseErr := ths.Parse()
		require.NoError(t, gotParseErr)

		done := make(chan struct{})
		runner := &minirunner.MiniRunner{
			Fn: func(ctx context.Context, _ *lib.State, out chan<- metrics.SampleContainer) error {
				out <- metrics.Sample{Metric: metric, Value: 1.25, Tags: metrics.IntoSampleTags(&map[string]string{"a": "1", "b": "2"})}
				close(done)
				return nil
			},
		}
		test := newTestEngineWithTestPreInitState(t, nil, runner, nil, lib.Options{
			Thresholds: map[string]metrics.Thresholds{
				"my_metric{a:1}": ths,
			},
		}, piState)

		go func() {
			assert.NoError(t, test.run())
		}()

		select {
		case <-done:
			return
		case <-time.After(10 * time.Second):
			assert.Fail(t, "Test should have completed within 10 seconds")
		}
		test.wait()

		assert.Len(t, test.engine.MetricsEngine.ObservedMetrics, 2)
		sms := test.engine.MetricsEngine.ObservedMetrics["my_metric{a:1}"]
		assert.EqualValues(t, map[string]string{"a": "1"}, sms.Sub.Tags.CloneTags())

		assert.IsType(t, &metrics.GaugeSink{}, test.engine.MetricsEngine.ObservedMetrics["my_metric"].Sink)
		assert.IsType(t, &metrics.GaugeSink{}, test.engine.MetricsEngine.ObservedMetrics["my_metric{a:1}"].Sink)
	})
}

func TestEngineThresholdsWillAbort(t *testing.T) {
	t.Parallel()

	piState := getTestPreInitState(t)
	metric, err := piState.Registry.NewMetric("my_metric", metrics.Gauge)
	require.NoError(t, err)

	// The incoming samples for the metric set it to 1.25. Considering
	// the metric is of type Gauge, value > 1.25 should always fail, and
	// trigger an abort.
	ths := metrics.NewThresholds([]string{"value>1.25"})
	gotParseErr := ths.Parse()
	require.NoError(t, gotParseErr)
	ths.Thresholds[0].AbortOnFail = true

	thresholds := map[string]metrics.Thresholds{metric.Name: ths}

	done := make(chan struct{})
	runner := &minirunner.MiniRunner{
		Fn: func(ctx context.Context, _ *lib.State, out chan<- metrics.SampleContainer) error {
			out <- metrics.Sample{Metric: metric, Value: 1.25, Tags: metrics.IntoSampleTags(&map[string]string{"a": "1"})}
			close(done)
			return nil
		},
	}
	test := newTestEngineWithTestPreInitState(t, nil, runner, nil, lib.Options{Thresholds: thresholds}, piState)

	go func() {
		assert.NoError(t, test.run())
	}()

	select {
	case <-done:
		return
	case <-time.After(10 * time.Second):
		assert.Fail(t, "Test should have completed within 10 seconds")
	}
	test.wait()
	assert.True(t, test.engine.thresholdsTainted)
}

func TestEngineAbortedByThresholds(t *testing.T) {
	t.Parallel()

	piState := getTestPreInitState(t)
	metric, err := piState.Registry.NewMetric("my_metric", metrics.Gauge)
	require.NoError(t, err)

	// The MiniRunner sets the value of the metric to 1.25. Considering
	// the metric is of type Gauge, value > 1.25 should always fail, and
	// trigger an abort.
	// **N.B**: a threshold returning an error, won't trigger an abort.
	ths := metrics.NewThresholds([]string{"value>1.25"})
	gotParseErr := ths.Parse()
	require.NoError(t, gotParseErr)
	ths.Thresholds[0].AbortOnFail = true

	thresholds := map[string]metrics.Thresholds{metric.Name: ths}

	done := make(chan struct{})
	runner := &minirunner.MiniRunner{
		Fn: func(ctx context.Context, _ *lib.State, out chan<- metrics.SampleContainer) error {
			out <- metrics.Sample{Metric: metric, Value: 1.25, Tags: metrics.IntoSampleTags(&map[string]string{"a": "1"})}
			<-ctx.Done()
			close(done)
			return nil
		},
	}

	test := newTestEngineWithTestPreInitState(t, nil, runner, nil, lib.Options{Thresholds: thresholds}, piState)
	defer test.wait()

	go func() {
		assert.NoError(t, test.run())
	}()

	select {
	case <-done:
		return
	case <-time.After(10 * time.Second):
		assert.Fail(t, "Test should have completed within 10 seconds")
	}
}

func TestEngine_processThresholds(t *testing.T) {
	t.Parallel()

	testdata := map[string]struct {
		pass bool
		ths  map[string][]string
	}{
		"passing": {true, map[string][]string{"my_metric": {"value<2"}}},
		"failing": {false, map[string][]string{"my_metric": {"value>1.25"}}},

		"submetric,match,passing":   {true, map[string][]string{"my_metric{a:1}": {"value<2"}}},
		"submetric,match,failing":   {false, map[string][]string{"my_metric{a:1}": {"value>1.25"}}},
		"submetric,nomatch,passing": {true, map[string][]string{"my_metric{a:2}": {"value<2"}}},
		"submetric,nomatch,failing": {false, map[string][]string{"my_metric{a:2}": {"value>1.25"}}},

		"unused,passing":      {true, map[string][]string{"unused_counter": {"count==0"}}},
		"unused,failing":      {false, map[string][]string{"unused_counter": {"count>1"}}},
		"unused,subm,passing": {true, map[string][]string{"unused_counter{a:2}": {"count<1"}}},
		"unused,subm,failing": {false, map[string][]string{"unused_counter{a:2}": {"count>1"}}},

		"used,passing":               {true, map[string][]string{"used_counter": {"count==2"}}},
		"used,failing":               {false, map[string][]string{"used_counter": {"count<1"}}},
		"used,subm,passing":          {true, map[string][]string{"used_counter{b:1}": {"count==2"}}},
		"used,not-subm,passing":      {true, map[string][]string{"used_counter{b:2}": {"count==0"}}},
		"used,invalid-subm,passing1": {true, map[string][]string{"used_counter{c:''}": {"count==0"}}},
		"used,invalid-subm,failing1": {false, map[string][]string{"used_counter{c:''}": {"count>0"}}},
		"used,invalid-subm,passing2": {true, map[string][]string{"used_counter{c:}": {"count==0"}}},
		"used,invalid-subm,failing2": {false, map[string][]string{"used_counter{c:}": {"count>0"}}},
	}

	for name, data := range testdata {
		name, data := name, data
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			piState := getTestPreInitState(t)
			gaugeMetric, err := piState.Registry.NewMetric("my_metric", metrics.Gauge)
			require.NoError(t, err)
			counterMetric, err := piState.Registry.NewMetric("used_counter", metrics.Counter)
			require.NoError(t, err)
			_, err = piState.Registry.NewMetric("unused_counter", metrics.Counter)
			require.NoError(t, err)

			thresholds := make(map[string]metrics.Thresholds, len(data.ths))
			for m, srcs := range data.ths {
				ths := metrics.NewThresholds(srcs)
				gotParseErr := ths.Parse()
				require.NoError(t, gotParseErr)
				thresholds[m] = ths
			}

			runner := &minirunner.MiniRunner{}
			test := newTestEngineWithTestPreInitState(
				t, nil, runner, nil, lib.Options{Thresholds: thresholds}, piState,
			)

			test.engine.OutputManager.AddMetricSamples(
				[]metrics.SampleContainer{
					metrics.Sample{Metric: gaugeMetric, Value: 1.25, Tags: metrics.IntoSampleTags(&map[string]string{"a": "1"})},
					metrics.Sample{Metric: counterMetric, Value: 2, Tags: metrics.IntoSampleTags(&map[string]string{"b": "1"})},
				},
			)

			require.NoError(t, test.run())
			test.wait()

			assert.Equal(t, data.pass, !test.engine.IsTainted())
		})
	}
}

func getMetricSum(mo *mockoutput.MockOutput, name string) (result float64) {
	for _, sc := range mo.SampleContainers {
		for _, s := range sc.GetSamples() {
			if s.Metric.Name == name {
				result += s.Value
			}
		}
	}
	return
}

func getMetricCount(mo *mockoutput.MockOutput, name string) (result uint) {
	for _, sc := range mo.SampleContainers {
		for _, s := range sc.GetSamples() {
			if s.Metric.Name == name {
				result++
			}
		}
	}
	return
}

func getMetricMax(mo *mockoutput.MockOutput, name string) (result float64) {
	for _, sc := range mo.SampleContainers {
		for _, s := range sc.GetSamples() {
			if s.Metric.Name == name && s.Value > result {
				result = s.Value
			}
		}
	}
	return
}

const expectedHeaderMaxLength = 550

// FIXME: This test is too brittle, consider simplifying.
func TestSentReceivedMetrics(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
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
		// NOTE: This needs to be improved, in the case of HTTPS IN URL
		// it's highly possible to meet the case when data received is out
		// of in the possible interval
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
		// See https://github.com/k6io/k6/pull/1149
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
			getTestPreInitState(t),
			&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: []byte(ts.Code)},
			nil,
		)
		require.NoError(t, err)

		mockOutput := mockoutput.New()
		test := newTestEngine(t, nil, r, []output.Output{mockOutput}, lib.Options{
			Iterations:            null.IntFrom(tc.Iterations),
			VUs:                   null.IntFrom(tc.VUs),
			Hosts:                 tb.Dialer.Hosts,
			InsecureSkipTLSVerify: null.BoolFrom(true),
			NoVUConnectionReuse:   null.BoolFrom(noConnReuse),
			Batch:                 null.IntFrom(20),
		})

		errC := make(chan error)
		go func() { errC <- test.run() }()

		select {
		case <-time.After(10 * time.Second):
			t.Fatal("Test timed out")
		case err := <-errC:
			require.NoError(t, err)
		}
		test.wait()

		checkData := func(name string, expected int64) float64 {
			data := getMetricSum(mockOutput, name)
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

		return checkData(metrics.DataSentName, ts.ExpectedDataSent),
			checkData(metrics.DataReceivedName, ts.ExpectedDataReceived)
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
		t.Parallel()
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

	expectedRunTags := map[string]string{"foo": "bar", "test": "mest", "over": "written"}

	// it copies the map so in the case the runner will overwrite
	// some run tags' values it doesn't affect the assertion.
	runTags := make(map[string]string)
	for k, v := range expectedRunTags {
		runTags[k] = v
	}

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
		getTestPreInitState(t),
		&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: script},
		nil,
	)
	require.NoError(t, err)

	mockOutput := mockoutput.New()
	test := newTestEngine(t, nil, r, []output.Output{mockOutput}, lib.Options{
		Iterations:            null.IntFrom(3),
		VUs:                   null.IntFrom(2),
		Hosts:                 tb.Dialer.Hosts,
		RunTags:               runTags,
		SystemTags:            &metrics.DefaultSystemTagSet,
		InsecureSkipTLSVerify: null.BoolFrom(true),
	})

	errC := make(chan error)
	go func() { errC <- test.run() }()

	select {
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out")
	case err := <-errC:
		require.NoError(t, err)
	}
	test.wait()

	systemMetrics := []string{
		metrics.VUsName, metrics.VUsMaxName, metrics.IterationsName, metrics.IterationDurationName,
		metrics.GroupDurationName, metrics.DataSentName, metrics.DataReceivedName,
	}

	getExpectedOverVal := func(metricName string) string {
		for _, sysMetric := range systemMetrics {
			if sysMetric == metricName {
				return expectedRunTags["over"]
			}
		}
		return "the rainbow"
	}

	for _, s := range mockOutput.Samples {
		for key, expVal := range expectedRunTags {
			val, ok := s.Tags.Get(key)

			if key == "over" {
				expVal = getExpectedOverVal(s.Metric.Name)
			}

			assert.True(t, ok)
			assert.Equalf(t, expVal, val, "Wrong tag value in sample for metric %#v", s.Metric)
		}
	}
}

func TestSetupException(t *testing.T) {
	t.Parallel()

	script := []byte(`
	import bar from "./bar.js";
	export function setup() {
		bar();
	};
	export default function() {
	};
	`)

	memfs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(memfs, "/bar.js", []byte(`
	export default function () {
        baz();
	}
	function baz() {
		        throw new Error("baz");
			}
	`), 0x666))
	runner, err := js.New(
		getTestPreInitState(t),
		&loader.SourceData{URL: &url.URL{Scheme: "file", Path: "/script.js"}, Data: script},
		map[string]afero.Fs{"file": memfs},
	)
	require.NoError(t, err)

	test := newTestEngine(t, nil, runner, nil, lib.Options{
		SystemTags:      &metrics.DefaultSystemTagSet,
		SetupTimeout:    types.NullDurationFrom(3 * time.Second),
		TeardownTimeout: types.NullDurationFrom(3 * time.Second),
		VUs:             null.IntFrom(3),
	})
	defer test.wait()

	errC := make(chan error)
	go func() { errC <- test.run() }()

	select {
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out")
	case err := <-errC:
		require.Error(t, err)
		var exception errext.Exception
		require.ErrorAs(t, err, &exception)
		require.Equal(t, "Error: baz\n\tat baz (file:///bar.js:6:16(3))\n"+
			"\tat file:///bar.js:3:8(3)\n\tat setup (file:///script.js:4:2(4))\n\tat native\n",
			err.Error())
	}
}

func TestVuInitException(t *testing.T) {
	t.Parallel()

	script := []byte(`
		export let options = {
			vus: 3,
			iterations: 5,
		};

		export default function() {};

		if (__VU == 2) {
			throw new Error('oops in ' + __VU);
		}
	`)

	piState := getTestPreInitState(t)
	runner, err := js.New(
		piState,
		&loader.SourceData{URL: &url.URL{Scheme: "file", Path: "/script.js"}, Data: script},
		nil,
	)
	require.NoError(t, err)

	opts, err := executor.DeriveScenariosFromShortcuts(runner.GetOptions(), nil)
	require.NoError(t, err)

	testState := getTestRunState(t, piState, opts, runner)

	execScheduler, err := local.NewExecutionScheduler(testState)
	require.NoError(t, err)
	engine, err := NewEngine(testState, execScheduler, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, _, err = engine.Init(ctx, ctx) // no need for 2 different contexts

	require.Error(t, err)

	var exception errext.Exception
	require.ErrorAs(t, err, &exception)
	assert.Equal(t, "Error: oops in 2\n\tat file:///script.js:10:9(29)\n\tat native\n", err.Error())

	var errWithHint errext.HasHint
	require.ErrorAs(t, err, &errWithHint)
	assert.Equal(t, "error while initializing VU #2 (script exception)", errWithHint.Hint())
}

func TestEmittedMetricsWhenScalingDown(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

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
		getTestPreInitState(t),
		&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: script},
		nil,
	)
	require.NoError(t, err)

	mockOutput := mockoutput.New()
	test := newTestEngine(t, nil, runner, []output.Output{mockOutput}, lib.Options{})

	errC := make(chan error)
	go func() { errC <- test.run() }()

	select {
	case <-time.After(12 * time.Second):
		t.Fatal("Test timed out")
	case err := <-errC:
		require.NoError(t, err)
		test.wait()
		require.False(t, test.engine.IsTainted())
	}

	// The 3.1 sleep in the default function would cause the first VU to complete 2 full iterations
	// and stat executing its third one, while the second VU will only fully complete 1 iteration
	// and will be canceled in the middle of its second one.
	assert.Equal(t, 3.0, getMetricSum(mockOutput, metrics.IterationsName))

	// That means that we expect to see 8 HTTP requests in total, 3*2=6 from the complete iterations
	// and one each from the two iterations that would be canceled in the middle of their execution
	assert.Equal(t, 8.0, getMetricSum(mockOutput, metrics.HTTPReqsName))

	// And we expect to see the data_received for all 8 of those requests. Previously, the data for
	// the 8th request (the 3rd one in the first VU before the test ends) was cut off by the engine
	// because it was emitted after the test officially ended. But that was mostly an unintended
	// consequence of the fact that those metrics were emitted only after an iteration ended when
	// it was interrupted.
	dataReceivedExpectedMin := 15000.0 * 8
	dataReceivedExpectedMax := (15000.0 + expectedHeaderMaxLength) * 8
	dataReceivedActual := getMetricSum(mockOutput, metrics.DataReceivedName)
	if dataReceivedActual < dataReceivedExpectedMin || dataReceivedActual > dataReceivedExpectedMax {
		t.Errorf(
			"The data_received sum should be in the interval [%f, %f] but was %f",
			dataReceivedExpectedMin, dataReceivedExpectedMax, dataReceivedActual,
		)
	}

	// Also, the interrupted iterations shouldn't affect the average iteration_duration in any way, only
	// complete iterations should be taken into account
	durationCount := float64(getMetricCount(mockOutput, metrics.IterationDurationName))
	assert.Equal(t, 3.0, durationCount)
	durationSum := getMetricSum(mockOutput, metrics.IterationDurationName)
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
				getTestPreInitState(t),
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
			)
			require.NoError(t, err)

			mockOutput := mockoutput.New()
			test := newTestEngine(t, nil, runner, []output.Output{mockOutput}, runner.GetOptions())

			errC := make(chan error)
			go func() { errC <- test.run() }()

			select {
			case <-time.After(10 * time.Second):
				t.Fatal("Test timed out")
			case err := <-errC:
				require.NoError(t, err)
				test.wait()
				require.False(t, test.engine.IsTainted())
			}

			assert.Equal(t, tc.expIters, getMetricSum(mockOutput, metrics.IterationsName))
			assert.Equal(t, tc.expCount, getMetricSum(mockOutput, "testcounter"))
		})
	}
}

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
			t.Parallel()
			runner, err := js.New(
				getTestPreInitState(t),
				&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: []byte(tc.script)},
				nil,
			)
			require.NoError(t, err)

			test := newTestEngine(t, nil, runner, nil, runner.GetOptions())

			errC := make(chan error)
			go func() { errC <- test.run() }()
			select {
			case <-time.After(10 * time.Second):
				t.Fatal("Test timed out")
			case err := <-errC:
				require.NoError(t, err)
				test.wait()
				require.False(t, test.engine.IsTainted())
			}
		})
	}
}

func TestEngineRunsTeardownEvenAfterTestRunIsAborted(t *testing.T) {
	t.Parallel()

	piState := getTestPreInitState(t)
	testMetric, err := piState.Registry.NewMetric("teardown_metric", metrics.Counter)
	require.NoError(t, err)

	var test *testStruct
	runner := &minirunner.MiniRunner{
		Fn: func(_ context.Context, _ *lib.State, _ chan<- metrics.SampleContainer) error {
			test.runCancel() // we cancel the run immediately after the test starts
			return nil
		},
		TeardownFn: func(_ context.Context, out chan<- metrics.SampleContainer) error {
			out <- metrics.Sample{Metric: testMetric, Value: 1}
			return nil
		},
	}

	mockOutput := mockoutput.New()
	test = newTestEngineWithTestPreInitState(t, nil, runner, []output.Output{mockOutput}, lib.Options{
		VUs: null.IntFrom(1), Iterations: null.IntFrom(1),
	}, piState)

	assert.NoError(t, test.run())
	test.wait()

	var count float64
	for _, sample := range mockOutput.Samples {
		if sample.Metric == testMetric {
			count += sample.Value
		}
	}
	assert.Equal(t, 1.0, count)
}

func TestActiveVUsCount(t *testing.T) {
	t.Parallel()

	script := []byte(`
		var sleep = require('k6').sleep;

		exports.options = {
			scenarios: {
				carr1: {
					executor: 'constant-arrival-rate',
					rate: 10,
					preAllocatedVUs: 1,
					maxVUs: 10,
					startTime: '0s',
					duration: '3s',
					gracefulStop: '0s',
				},
				carr2: {
					executor: 'constant-arrival-rate',
					rate: 10,
					preAllocatedVUs: 1,
					maxVUs: 10,
					duration: '3s',
					startTime: '3s',
					gracefulStop: '0s',
				},
				rarr: {
					executor: 'ramping-arrival-rate',
					startRate: 5,
					stages: [
						{ target: 10, duration: '2s' },
						{ target: 0, duration: '2s' },
					],
					preAllocatedVUs: 1,
					maxVUs: 10,
					startTime: '6s',
					gracefulStop: '0s',
				},
			}
		}

		exports.default = function () {
			sleep(5);
		}
	`)

	logger := testutils.NewLogger(t)
	logHook := testutils.SimpleLogrusHook{HookedLevels: logrus.AllLevels}
	logger.AddHook(&logHook)

	rtOpts := lib.RuntimeOptions{CompatibilityMode: null.StringFrom("base")}

	registry := metrics.NewRegistry()
	piState := &lib.TestPreInitState{
		Logger:         logger,
		Registry:       registry,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		RuntimeOptions: rtOpts,
	}
	runner, err := js.New(piState, &loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: script}, nil)
	require.NoError(t, err)

	mockOutput := mockoutput.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts, err := executor.DeriveScenariosFromShortcuts(lib.Options{
		MetricSamplesBufferSize: null.NewInt(200, false),
	}.Apply(runner.GetOptions()), nil)
	require.NoError(t, err)

	testState := getTestRunState(t, piState, opts, runner)
	execScheduler, err := local.NewExecutionScheduler(testState)
	require.NoError(t, err)
	engine, err := NewEngine(testState, execScheduler, []output.Output{mockOutput})
	require.NoError(t, err)
	require.NoError(t, engine.OutputManager.StartOutputs())
	run, waitFn, err := engine.Init(ctx, ctx) // no need for 2 different contexts
	require.NoError(t, err)

	errC := make(chan error)
	go func() { errC <- run() }()

	select {
	case <-time.After(15 * time.Second):
		t.Fatal("Test timed out")
	case err := <-errC:
		require.NoError(t, err)
		cancel()
		waitFn()
		engine.OutputManager.StopOutputs()
		require.False(t, engine.IsTainted())
	}

	assert.Equal(t, 10.0, getMetricMax(mockOutput, metrics.VUsName))
	assert.Equal(t, 10.0, getMetricMax(mockOutput, metrics.VUsMaxName))

	logEntries := logHook.Drain()
	assert.Len(t, logEntries, 3)
	for _, logEntry := range logEntries {
		assert.Equal(t, logrus.WarnLevel, logEntry.Level)
		assert.Equal(t, "Insufficient VUs, reached 10 active VUs and cannot initialize more", logEntry.Message)
	}
}
