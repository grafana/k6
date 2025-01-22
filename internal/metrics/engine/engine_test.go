package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

func TestNewMetricsEngineWithThresholds(t *testing.T) {
	t.Parallel()

	trs := &lib.TestRunState{
		TestPreInitState: &lib.TestPreInitState{
			Logger:   testutils.NewLogger(t),
			Registry: metrics.NewRegistry(),
		},
		Options: lib.Options{
			Thresholds: map[string]metrics.Thresholds{
				"metric1": {Thresholds: []*metrics.Threshold{}},
				"metric2": {Thresholds: []*metrics.Threshold{}},
			},
		},
	}
	_, err := trs.Registry.NewMetric("metric1", metrics.Counter)
	require.NoError(t, err)

	_, err = trs.Registry.NewMetric("metric2", metrics.Counter)
	require.NoError(t, err)

	me, err := NewMetricsEngine(trs.Registry, trs.Logger)
	require.NoError(t, err)
	require.NotNil(t, me)

	require.NoError(t, me.InitSubMetricsAndThresholds(trs.Options, false))

	assert.Len(t, me.metricsWithThresholds, 2)
}

func TestMetricsEngineGetThresholdMetricOrSubmetricError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		metricDefinition string
		expErr           string
	}{
		{metricDefinition: "metric1{test:a", expErr: "missing ending bracket"},
		{metricDefinition: "metric2", expErr: "'metric2' does not exist in the script"},
		{metricDefinition: "metric1{}", expErr: "submetric criteria for metric 'metric1' cannot be empty"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()

			me := newTestMetricsEngine(t)
			_, err := me.registry.NewMetric("metric1", metrics.Counter)
			require.NoError(t, err)

			_, err = me.getThresholdMetricOrSubmetric(tc.metricDefinition)
			assert.ErrorContains(t, err, tc.expErr)
		})
	}
}

func TestNewMetricsEngineNoThresholds(t *testing.T) {
	t.Parallel()

	me := newTestMetricsEngine(t)
	require.NotNil(t, me)
	assert.Empty(t, me.metricsWithThresholds)
}

func TestMetricsEngineCreateIngester(t *testing.T) {
	t.Parallel()

	me := MetricsEngine{
		logger: testutils.NewLogger(t),
	}
	ingester := me.CreateIngester()
	assert.NotNil(t, ingester)
	require.NoError(t, ingester.Start())
	require.NoError(t, ingester.Stop())
}

func TestMetricsEngineEvaluateThresholdNoAbort(t *testing.T) {
	t.Parallel()

	cases := []struct {
		threshold   string
		abortOnFail bool
		expBreached []string
	}{
		{threshold: "count>5", expBreached: nil},
		{threshold: "count<5", expBreached: []string{"m1"}},
		{threshold: "count<5", expBreached: []string{"m1"}, abortOnFail: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.threshold, func(t *testing.T) {
			t.Parallel()
			me := newTestMetricsEngine(t)

			m1, err := me.registry.NewMetric("m1", metrics.Counter)
			require.NoError(t, err)
			m2, err := me.registry.NewMetric("m2", metrics.Counter)
			require.NoError(t, err)

			ths := metrics.NewThresholds([]string{tc.threshold})
			require.NoError(t, ths.Parse())
			m1.Thresholds = ths
			m1.Thresholds.Thresholds[0].AbortOnFail = tc.abortOnFail

			me.metricsWithThresholds = []*metrics.Metric{m1, m2}
			m1.Sink.Add(metrics.Sample{Value: 6.0})

			breached, abort := me.evaluateThresholds(false, zeroTestRunDuration)
			require.Equal(t, tc.abortOnFail, abort)
			assert.Equal(t, tc.expBreached, breached)
		})
	}
}

func TestMetricsEngineEvaluateIgnoreEmptySink(t *testing.T) {
	t.Parallel()

	me := newTestMetricsEngine(t)

	m1, err := me.registry.NewMetric("m1", metrics.Counter)
	require.NoError(t, err)
	m2, err := me.registry.NewMetric("m2", metrics.Counter)
	require.NoError(t, err)

	ths := metrics.NewThresholds([]string{"count>5"})
	require.NoError(t, ths.Parse())
	m1.Thresholds = ths
	m1.Thresholds.Thresholds[0].AbortOnFail = true

	me.metricsWithThresholds = []*metrics.Metric{m1, m2}

	breached, abort := me.evaluateThresholds(false, zeroTestRunDuration)
	require.True(t, abort)
	require.Equal(t, []string{"m1"}, breached)

	breached, abort = me.evaluateThresholds(true, zeroTestRunDuration)
	require.False(t, abort)
	assert.Empty(t, breached)
}

func newTestMetricsEngine(t *testing.T) *MetricsEngine {
	m, err := NewMetricsEngine(metrics.NewRegistry(), testutils.NewLogger(t))
	require.NoError(t, err)
	return m
}

func zeroTestRunDuration() time.Duration {
	return 0
}

/*
// FIXME: This test is too brittle,
// move them as e2e tests and consider to simplify.
//
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
			Hosts:                 types.NullHosts{Trie: tb.Dialer.Hosts, Valid: true},
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
*/
