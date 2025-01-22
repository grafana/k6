package execution_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/internal/execution"
	"go.k6.io/k6/internal/execution/local"
	"go.k6.io/k6/internal/js"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/internal/lib/testutils/httpmultibin"
	"go.k6.io/k6/internal/lib/testutils/minirunner"
	"go.k6.io/k6/internal/lib/testutils/mockresolver"
	"go.k6.io/k6/internal/loader"
	"go.k6.io/k6/internal/usage"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/executor"
	"go.k6.io/k6/lib/netext"
	"go.k6.io/k6/lib/netext/httpext"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
)

func getTestPreInitState(tb testing.TB) *lib.TestPreInitState {
	reg := metrics.NewRegistry()
	return &lib.TestPreInitState{
		Logger:         testutils.NewLogger(tb),
		RuntimeOptions: lib.RuntimeOptions{},
		Registry:       reg,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(reg),
		Usage:          usage.New(),
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
		RunTags:          piState.Registry.RootTagSet().WithTagsFromMap(options.RunTags),
	}
}

func newTestScheduler(
	t *testing.T, runner lib.Runner, logger logrus.FieldLogger, opts lib.Options,
) (ctx context.Context, cancel func(), execScheduler *execution.Scheduler, samples chan metrics.SampleContainer) {
	if runner == nil {
		runner = &minirunner.MiniRunner{}
	}
	ctx, cancel = context.WithCancel(context.Background())
	newOpts, err := executor.DeriveScenariosFromShortcuts(lib.Options{
		MetricSamplesBufferSize: null.NewInt(200, false),
	}.Apply(runner.GetOptions()).Apply(opts), nil)
	require.NoError(t, err)

	testRunState := getTestRunState(t, getTestPreInitState(t), newOpts, runner)
	if logger != nil {
		testRunState.Logger = logger
	}

	execScheduler, err = execution.NewScheduler(testRunState, local.NewController())
	require.NoError(t, err)

	samples = make(chan metrics.SampleContainer, newOpts.MetricSamplesBufferSize.Int64)
	go func() {
		for {
			select {
			case <-samples:
			case <-ctx.Done():
				return
			}
		}
	}()

	stopEmission, err := execScheduler.Init(ctx, samples)
	require.NoError(t, err)
	t.Cleanup(func() {
		stopEmission()
		close(samples)
	})

	return ctx, cancel, execScheduler, samples
}

func TestSchedulerRun(t *testing.T) {
	t.Parallel()
	ctx, cancel, execScheduler, samples := newTestScheduler(t, nil, nil, lib.Options{})
	defer cancel()

	err := make(chan error, 1)
	go func() { err <- execScheduler.Run(ctx, ctx, samples) }()
	assert.NoError(t, <-err)
}

func TestSchedulerRunNonDefault(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name, script string
	}{
		{"defaultOK", `export default function () {}`},
		{"nonDefaultOK", `
	export let options = {
		scenarios: {
			per_vu_iters: {
				executor: "per-vu-iterations",
				vus: 1,
				iterations: 1,
				exec: "nonDefault",
			},
		}
	}
	export function nonDefault() {}`},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			piState := getTestPreInitState(t)
			runner, err := js.New(
				piState, &loader.SourceData{
					URL: &url.URL{Path: "/script.js"}, Data: []byte(tc.script),
				}, nil)
			require.NoError(t, err)

			testRunState := getTestRunState(t, piState, runner.GetOptions(), runner)

			execScheduler, err := execution.NewScheduler(testRunState, local.NewController())
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			done := make(chan struct{})
			samples := make(chan metrics.SampleContainer)
			defer close(samples)

			stopEmission, err := execScheduler.Init(ctx, samples)
			require.NoError(t, err)

			go func() {
				defer close(done)
				defer stopEmission()
				assert.NoError(t, execScheduler.Run(ctx, ctx, samples))
			}()
			for {
				select {
				case <-samples:
				case <-done:
					return
				}
			}
		})
	}
}

func TestSchedulerRunEnv(t *testing.T) {
	t.Parallel()

	scriptTemplate := `
	import { Counter } from "k6/metrics";

	let errors = new Counter("errors");

	export let options = {
		scenarios: {
			executor: {
				executor: "%[1]s",
				%[2]s
			}
		}
	}

	export default function () {
		if (__ENV.TESTVAR !== "%[3]s") {
		    console.error('Wrong env var value. Expected: %[3]s, actual: ', __ENV.TESTVAR);
			errors.add(1);
		}
	}`

	executorConfigs := map[string]string{
		"constant-arrival-rate": `
			rate: 1,
			timeUnit: "1s",
			duration: "1s",
			preAllocatedVUs: 1,
			maxVUs: 2,
			gracefulStop: "0.5s",`,
		"constant-vus": `
			vus: 1,
			duration: "1s",
			gracefulStop: "0.5s",`,
		"externally-controlled": `
			vus: 1,
			duration: "1s",`,
		"per-vu-iterations": `
			vus: 1,
			iterations: 1,
			gracefulStop: "0.5s",`,
		"shared-iterations": `
			vus: 1,
			iterations: 1,
			gracefulStop: "0.5s",`,
		"ramping-arrival-rate": `
			startRate: 1,
			timeUnit: "0.5s",
			preAllocatedVUs: 1,
			maxVUs: 2,
			stages: [ { target: 1, duration: "1s" } ],
			gracefulStop: "0.5s",`,
		"ramping-vus": `
			startVUs: 1,
			stages: [ { target: 1, duration: "1s" } ],
			gracefulStop: "0.5s",`,
	}

	testCases := []struct{ name, script string }{}

	// Generate tests using global env and with env override
	for ename, econf := range executorConfigs {
		testCases = append(testCases, struct{ name, script string }{
			"global/" + ename, fmt.Sprintf(scriptTemplate, ename, econf, "global"),
		})
		configWithEnvOverride := econf + "env: { TESTVAR: 'overridden' }"
		testCases = append(testCases, struct{ name, script string }{
			"override/" + ename, fmt.Sprintf(scriptTemplate, ename, configWithEnvOverride, "overridden"),
		})
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			piState := getTestPreInitState(t)
			piState.RuntimeOptions = lib.RuntimeOptions{Env: map[string]string{"TESTVAR": "global"}}
			runner, err := js.New(
				piState, &loader.SourceData{
					URL:  &url.URL{Path: "/script.js"},
					Data: []byte(tc.script),
				}, nil,
			)
			require.NoError(t, err)

			testRunState := getTestRunState(t, piState, runner.GetOptions(), runner)
			execScheduler, err := execution.NewScheduler(testRunState, local.NewController())
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			done := make(chan struct{})
			samples := make(chan metrics.SampleContainer)
			defer close(samples)

			stopEmission, err := execScheduler.Init(ctx, samples)
			require.NoError(t, err)

			go func() {
				defer close(done)
				defer stopEmission()
				assert.NoError(t, execScheduler.Run(ctx, ctx, samples))
			}()
			for {
				select {
				case sample := <-samples:
					if s, ok := sample.(metrics.Sample); ok && s.Metric.Name == "errors" {
						assert.FailNow(t, "received error sample from test")
					}
				case <-done:
					return
				}
			}
		})
	}
}

func TestSchedulerSystemTags(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	script := sr(`
	import http from "k6/http";

	export let options = {
		scenarios: {
			per_vu_test: {
				executor: "per-vu-iterations",
				gracefulStop: "0s",
				vus: 1,
				iterations: 1,
			},
			shared_test: {
				executor: "shared-iterations",
				gracefulStop: "0s",
				vus: 1,
				iterations: 1,
			}
		}
	}

	export default function () {
		http.get("HTTPBIN_IP_URL/");
	}`)

	piState := getTestPreInitState(t)
	runner, err := js.New(
		piState, &loader.SourceData{
			URL:  &url.URL{Path: "/script.js"},
			Data: []byte(script),
		}, nil)
	require.NoError(t, err)

	require.NoError(t, runner.SetOptions(runner.GetOptions().Apply(lib.Options{
		SystemTags: &metrics.DefaultSystemTagSet,
	})))

	testRunState := getTestRunState(t, piState, runner.GetOptions(), runner)
	execScheduler, err := execution.NewScheduler(testRunState, local.NewController())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	samples := make(chan metrics.SampleContainer)
	defer close(samples)

	stopEmission, err := execScheduler.Init(ctx, samples)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer stopEmission()
		require.NoError(t, execScheduler.Run(ctx, ctx, samples))
	}()

	expCommonTrailTags := testRunState.RunTags.
		With("group", "").
		With("method", "GET").
		With("name", sr("HTTPBIN_IP_URL/")).
		With("url", sr("HTTPBIN_IP_URL/")).
		With("proto", "HTTP/1.1").
		With("status", "200").
		With("expected_response", "true")

	expTrailPVUTags := expCommonTrailTags.With("scenario", "per_vu_test")
	expTrailSITags := expCommonTrailTags.With("scenario", "shared_test")
	expDataSentPVUTags := testRunState.RunTags.With("group", "").With("scenario", "per_vu_test")
	expDataSentSITags := testRunState.RunTags.With("group", "").With("scenario", "shared_test")

	var gotCorrectTags int
	for {
		select {
		case sample := <-samples:
			switch s := sample.(type) {
			case *httpext.Trail:
				if s.Tags == expTrailPVUTags || s.Tags == expTrailSITags {
					gotCorrectTags++
				}
			default:
				for _, sample := range s.GetSamples() {
					if sample.Metric.Name == metrics.DataSentName {
						if sample.Tags == expDataSentPVUTags || sample.Tags == expDataSentSITags {
							gotCorrectTags++
						}
					}
				}
			}

		case <-done:
			assert.Equal(t, 4, gotCorrectTags, "received wrong amount of samples with expected tags")
			return
		}
	}
}

func TestSchedulerRunCustomTags(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	scriptTemplate := sr(`
	import http from "k6/http";

	export let options = {
		scenarios: {
			executor: {
				executor: "%s",
				%s
			}
		}
	}

	export default function () {
		http.get("HTTPBIN_IP_URL/");
	}`)

	executorConfigs := map[string]string{
		"constant-arrival-rate": `
			rate: 1,
			timeUnit: "1s",
			duration: "1s",
			preAllocatedVUs: 1,
			maxVUs: 2,
			gracefulStop: "0.5s",`,
		"constant-vus": `
			vus: 1,
			duration: "1s",
			gracefulStop: "0.5s",`,
		"externally-controlled": `
			vus: 1,
			duration: "1s",`,
		"per-vu-iterations": `
			vus: 1,
			iterations: 1,
			gracefulStop: "0.5s",`,
		"shared-iterations": `
			vus: 1,
			iterations: 1,
			gracefulStop: "0.5s",`,
		"ramping-arrival-rate": `
			startRate: 5,
			timeUnit: "0.5s",
			preAllocatedVUs: 1,
			maxVUs: 2,
			stages: [ { target: 10, duration: "1s" } ],
			gracefulStop: "0.5s",`,
		"ramping-vus": `
			startVUs: 1,
			stages: [ { target: 1, duration: "0.5s" } ],
			gracefulStop: "0.5s",`,
	}

	testCases := []struct{ name, script string }{}

	// Generate tests using custom tags
	for ename, econf := range executorConfigs {
		configWithCustomTag := econf + "tags: { customTag: 'value' }"
		testCases = append(testCases, struct{ name, script string }{
			ename, fmt.Sprintf(scriptTemplate, ename, configWithCustomTag),
		})
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			piState := getTestPreInitState(t)
			runner, err := js.New(
				piState, &loader.SourceData{
					URL:  &url.URL{Path: "/script.js"},
					Data: []byte(tc.script),
				}, nil,
			)
			require.NoError(t, err)

			testRunState := getTestRunState(t, piState, runner.GetOptions(), runner)
			execScheduler, err := execution.NewScheduler(testRunState, local.NewController())
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			done := make(chan struct{})
			samples := make(chan metrics.SampleContainer)
			defer close(samples)

			stopEmission, err := execScheduler.Init(ctx, samples)
			require.NoError(t, err)

			go func() {
				defer close(done)
				defer stopEmission()
				require.NoError(t, execScheduler.Run(ctx, ctx, samples))
			}()
			var gotTrailTag, gotDataSentTag bool
			for {
				select {
				case sample := <-samples:
					if trail, ok := sample.(*httpext.Trail); ok && !gotTrailTag {
						tags := trail.Tags.Map()
						if v, ok := tags["customTag"]; ok && v == "value" {
							gotTrailTag = true
						}
					}
					for _, s := range sample.GetSamples() {
						if s.Metric.Name == metrics.DataSentName && !gotDataSentTag {
							tags := s.Tags.Map()
							if v, ok := tags["customTag"]; ok && v == "value" {
								gotDataSentTag = true
							}
						}
					}
				case <-done:
					if !gotTrailTag || !gotDataSentTag {
						assert.FailNow(t, "a sample with expected tag wasn't received")
					}
					return
				}
			}
		})
	}
}

// Ensure that custom executor settings are unique per executor and
// that there's no "crossover"/"pollution" between executors.
// Also test that custom tags are properly set on checks and groups metrics.
func TestSchedulerRunCustomConfigNoCrossover(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	script := tb.Replacer.Replace(`
	import http from "k6/http";
	import ws from 'k6/ws';
	import { Counter } from 'k6/metrics';
	import { check, group } from 'k6';

	let errors = new Counter('errors');

	export let options = {
		// Required for WS tests
		hosts: { 'httpbin.local': '127.0.0.1' },
		scenarios: {
			scenario1: {
				executor: 'per-vu-iterations',
				vus: 1,
				iterations: 1,
				gracefulStop: '0s',
				maxDuration: '1s',
				exec: 's1func',
				env: { TESTVAR1: 'scenario1' },
				tags: { testtag1: 'scenario1' },
			},
			scenario2: {
				executor: 'shared-iterations',
				vus: 1,
				iterations: 1,
				gracefulStop: '1s',
				startTime: '0.5s',
				maxDuration: '2s',
				exec: 's2func',
				env: { TESTVAR2: 'scenario2' },
				tags: { testtag2: 'scenario2' },
			},
			scenario3: {
				executor: 'per-vu-iterations',
				vus: 1,
				iterations: 1,
				gracefulStop: '1s',
				exec: 's3funcWS',
				env: { TESTVAR3: 'scenario3' },
				tags: { testtag3: 'scenario3' },
			},
		}
	}

	function checkVar(name, expected) {
		if (__ENV[name] !== expected) {
		    console.error('Wrong ' + name + " env var value. Expected: '"
						+ expected + "', actual: '" + __ENV[name] + "'");
			errors.add(1);
		}
	}

	export function s1func() {
		checkVar('TESTVAR1', 'scenario1');
		checkVar('TESTVAR2', undefined);
		checkVar('TESTVAR3', undefined);
		checkVar('TESTGLOBALVAR', 'global');

		// Intentionally try to pollute the env
		__ENV.TESTVAR2 = 'overridden';

		http.get('HTTPBIN_IP_URL/', { tags: { reqtag: 'scenario1' }});
	}

	export function s2func() {
		checkVar('TESTVAR1', undefined);
		checkVar('TESTVAR2', 'scenario2');
		checkVar('TESTVAR3', undefined);
		checkVar('TESTGLOBALVAR', 'global');

		http.get('HTTPBIN_IP_URL/', { tags: { reqtag: 'scenario2' }});
	}

	export function s3funcWS() {
		checkVar('TESTVAR1', undefined);
		checkVar('TESTVAR2', undefined);
		checkVar('TESTVAR3', 'scenario3');
		checkVar('TESTGLOBALVAR', 'global');

		const customTags = { wstag: 'scenario3' };
		group('wsgroup', function() {
			const response = ws.connect('WSBIN_URL/ws-echo', { tags: customTags },
				function (socket) {
					socket.on('open', function() {
						socket.send('hello');
					});
					socket.on('message', function(msg) {
						if (msg != 'hello') {
						    console.error("Expected to receive 'hello' but got '" + msg + "' instead!");
							errors.add(1);
						}
						socket.close()
					});
					socket.on('error', function (e) {
						console.log('ws error: ' + e.error());
						errors.add(1);
					});
				}
			);
			check(response, { 'status is 101': (r) => r && r.status === 101 }, customTags);
		});
	}
`)

	piState := getTestPreInitState(t)
	piState.RuntimeOptions.Env = map[string]string{"TESTGLOBALVAR": "global"}
	runner, err := js.New(
		piState, &loader.SourceData{
			URL:  &url.URL{Path: "/script.js"},
			Data: []byte(script),
		},
		nil,
	)
	require.NoError(t, err)

	testRunState := getTestRunState(t, piState, runner.GetOptions(), runner)
	execScheduler, err := execution.NewScheduler(testRunState, local.NewController())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	samples := make(chan metrics.SampleContainer)

	stopEmission, err := execScheduler.Init(ctx, samples)
	require.NoError(t, err)

	go func() {
		assert.NoError(t, execScheduler.Run(ctx, ctx, samples))
		stopEmission()
		close(samples)
	}()

	expectedTrailTags := []map[string]string{
		{"testtag1": "scenario1", "reqtag": "scenario1"},
		{"testtag2": "scenario2", "reqtag": "scenario2"},
	}
	expectedNetTrailTags := []map[string]string{
		{"testtag1": "scenario1"},
		{"testtag2": "scenario2"},
	}
	expectedConnSampleTags := map[string]string{
		"testtag3": "scenario3", "wstag": "scenario3",
	}
	expectedPlainSampleTags := []map[string]string{
		{"testtag3": "scenario3"},
		{"testtag3": "scenario3", "wstag": "scenario3"},
	}
	var gotSampleTags int
	for sample := range samples {
		switch s := sample.(type) {
		case metrics.Sample:
			if s.Metric.Name == "errors" {
				assert.FailNow(t, "received error sample from test")
			}
			if s.Metric.Name == "checks" || s.Metric.Name == "group_duration" {
				tags := s.Tags.Map()
				for _, expTags := range expectedPlainSampleTags {
					if reflect.DeepEqual(expTags, tags) {
						gotSampleTags++
					}
				}
			}
		case *httpext.Trail:
			tags := s.Tags.Map()
			for _, expTags := range expectedTrailTags {
				if reflect.DeepEqual(expTags, tags) {
					gotSampleTags++
				}
			}
		case metrics.Samples:
			for _, sample := range s.GetSamples() {
				if sample.Metric.Name == metrics.DataSentName {
					tags := sample.Tags.Map()
					for _, expTags := range expectedNetTrailTags {
						if reflect.DeepEqual(expTags, tags) {
							gotSampleTags++
						}
					}
				}
			}
		case metrics.ConnectedSamples:
			for _, sm := range s.Samples {
				tags := sm.Tags.Map()
				if reflect.DeepEqual(expectedConnSampleTags, tags) {
					gotSampleTags++
				}
			}
		}
	}
	require.Equal(t, 8, gotSampleTags, "received wrong amount of samples with expected tags")
}

func TestSchedulerSetupTeardownRun(t *testing.T) {
	t.Parallel()
	t.Run("Normal", func(t *testing.T) {
		t.Parallel()
		setupC := make(chan struct{})
		teardownC := make(chan struct{})
		runner := &minirunner.MiniRunner{
			SetupFn: func(_ context.Context, _ chan<- metrics.SampleContainer) ([]byte, error) {
				close(setupC)
				return nil, nil
			},
			TeardownFn: func(_ context.Context, _ chan<- metrics.SampleContainer) error {
				close(teardownC)
				return nil
			},
		}
		ctx, cancel, execScheduler, samples := newTestScheduler(t, runner, nil, lib.Options{})

		err := make(chan error, 1)
		go func() { err <- execScheduler.Run(ctx, ctx, samples) }()
		defer cancel()
		<-setupC
		<-teardownC
		assert.NoError(t, <-err)
	})
	t.Run("Setup Error", func(t *testing.T) {
		t.Parallel()
		runner := &minirunner.MiniRunner{
			SetupFn: func(_ context.Context, _ chan<- metrics.SampleContainer) ([]byte, error) {
				return nil, errors.New("setup error")
			},
		}
		ctx, cancel, execScheduler, samples := newTestScheduler(t, runner, nil, lib.Options{})
		defer cancel()
		assert.EqualError(t, execScheduler.Run(ctx, ctx, samples), "setup error")
	})
	t.Run("Don't Run Setup", func(t *testing.T) {
		t.Parallel()
		runner := &minirunner.MiniRunner{
			SetupFn: func(_ context.Context, _ chan<- metrics.SampleContainer) ([]byte, error) {
				return nil, errors.New("setup _")
			},
			TeardownFn: func(_ context.Context, _ chan<- metrics.SampleContainer) error {
				return errors.New("teardown error")
			},
		}
		ctx, cancel, execScheduler, samples := newTestScheduler(t, runner, nil, lib.Options{
			NoSetup:    null.BoolFrom(true),
			VUs:        null.IntFrom(1),
			Iterations: null.IntFrom(1),
		})
		defer cancel()
		assert.EqualError(t, execScheduler.Run(ctx, ctx, samples), "teardown error")
	})

	t.Run("Teardown Error", func(t *testing.T) {
		t.Parallel()
		runner := &minirunner.MiniRunner{
			SetupFn: func(_ context.Context, _ chan<- metrics.SampleContainer) ([]byte, error) {
				return nil, nil
			},
			TeardownFn: func(_ context.Context, _ chan<- metrics.SampleContainer) error {
				return errors.New("teardown error")
			},
		}
		ctx, cancel, execScheduler, samples := newTestScheduler(t, runner, nil, lib.Options{
			VUs:        null.IntFrom(1),
			Iterations: null.IntFrom(1),
		})
		defer cancel()

		assert.EqualError(t, execScheduler.Run(ctx, ctx, samples), "teardown error")
	})
	t.Run("Don't Run Teardown", func(t *testing.T) {
		t.Parallel()
		runner := &minirunner.MiniRunner{
			SetupFn: func(_ context.Context, _ chan<- metrics.SampleContainer) ([]byte, error) {
				return nil, nil
			},
			TeardownFn: func(_ context.Context, _ chan<- metrics.SampleContainer) error {
				return errors.New("teardown error")
			},
		}
		ctx, cancel, execScheduler, samples := newTestScheduler(t, runner, nil, lib.Options{
			NoTeardown: null.BoolFrom(true),
			VUs:        null.IntFrom(1),
			Iterations: null.IntFrom(1),
		})
		defer cancel()
		assert.NoError(t, execScheduler.Run(ctx, ctx, samples))
	})
}

func TestSchedulerStages(t *testing.T) {
	t.Parallel()
	testdata := map[string]struct {
		Duration time.Duration
		Stages   []lib.Stage
	}{
		"one": {
			1 * time.Second,
			[]lib.Stage{{Duration: types.NullDurationFrom(1 * time.Second), Target: null.IntFrom(1)}},
		},
		"two": {
			2 * time.Second,
			[]lib.Stage{
				{Duration: types.NullDurationFrom(1 * time.Second), Target: null.IntFrom(1)},
				{Duration: types.NullDurationFrom(1 * time.Second), Target: null.IntFrom(2)},
			},
		},
		"four": {
			4 * time.Second,
			[]lib.Stage{
				{Duration: types.NullDurationFrom(1 * time.Second), Target: null.IntFrom(5)},
				{Duration: types.NullDurationFrom(3 * time.Second), Target: null.IntFrom(10)},
			},
		},
	}

	for name, data := range testdata {
		data := data
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runner := &minirunner.MiniRunner{
				Fn: func(_ context.Context, _ *lib.State, _ chan<- metrics.SampleContainer) error {
					time.Sleep(100 * time.Millisecond)
					return nil
				},
			}
			ctx, cancel, execScheduler, samples := newTestScheduler(t, runner, nil, lib.Options{
				VUs:    null.IntFrom(1),
				Stages: data.Stages,
			})
			defer cancel()
			assert.NoError(t, execScheduler.Run(ctx, ctx, samples))
			assert.True(t, execScheduler.GetState().GetCurrentTestRunDuration() >= data.Duration)
		})
	}
}

func TestSchedulerEndTime(t *testing.T) {
	t.Parallel()
	runner := &minirunner.MiniRunner{
		Fn: func(_ context.Context, _ *lib.State, _ chan<- metrics.SampleContainer) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		},
	}
	ctx, cancel, execScheduler, samples := newTestScheduler(t, runner, nil, lib.Options{
		VUs:      null.IntFrom(10),
		Duration: types.NullDurationFrom(1 * time.Second),
	})
	defer cancel()

	endTime, isFinal := lib.GetEndOffset(execScheduler.GetExecutionPlan())
	assert.Equal(t, 31*time.Second, endTime) // because of the default 30s gracefulStop
	assert.True(t, isFinal)

	startTime := time.Now()
	assert.NoError(t, execScheduler.Run(ctx, ctx, samples))
	runTime := time.Since(startTime)
	assert.True(t, runTime > 1*time.Second, "test did not take 1s")
	assert.True(t, runTime < 10*time.Second, "took more than 10 seconds")
}

func TestSchedulerRuntimeErrors(t *testing.T) {
	t.Parallel()
	runner := &minirunner.MiniRunner{
		Fn: func(_ context.Context, _ *lib.State, _ chan<- metrics.SampleContainer) error {
			time.Sleep(10 * time.Millisecond)
			return errors.New("hi")
		},
		Options: lib.Options{
			VUs:      null.IntFrom(10),
			Duration: types.NullDurationFrom(1 * time.Second),
		},
	}
	logger, hook := logtest.NewNullLogger()
	ctx, cancel, execScheduler, samples := newTestScheduler(t, runner, logger, lib.Options{})
	defer cancel()

	endTime, isFinal := lib.GetEndOffset(execScheduler.GetExecutionPlan())
	assert.Equal(t, 31*time.Second, endTime) // because of the default 30s gracefulStop
	assert.True(t, isFinal)

	startTime := time.Now()
	require.NoError(t, execScheduler.Run(ctx, ctx, samples))
	runTime := time.Since(startTime)
	assert.True(t, runTime > 1*time.Second, "test did not take 1s")
	assert.True(t, runTime < 10*time.Second, "took more than 10 seconds")

	assert.NotEmpty(t, hook.Entries)
	for _, e := range hook.Entries {
		assert.Equal(t, "hi", e.Message)
	}
}

func TestSchedulerEndErrors(t *testing.T) {
	t.Parallel()

	exec := executor.NewConstantVUsConfig("we_need_hard_stop")
	exec.VUs = null.IntFrom(10)
	exec.Duration = types.NullDurationFrom(1 * time.Second)
	exec.GracefulStop = types.NullDurationFrom(0 * time.Second)

	runner := &minirunner.MiniRunner{
		Fn: func(ctx context.Context, _ *lib.State, _ chan<- metrics.SampleContainer) error {
			<-ctx.Done()
			return errors.New("hi")
		},
		Options: lib.Options{
			Scenarios: lib.ScenarioConfigs{exec.GetName(): exec},
		},
	}
	logger, hook := logtest.NewNullLogger()
	ctx, cancel, execScheduler, samples := newTestScheduler(t, runner, logger, lib.Options{})
	defer cancel()

	endTime, isFinal := lib.GetEndOffset(execScheduler.GetExecutionPlan())
	assert.Equal(t, 1*time.Second, endTime) // because of the 0s gracefulStop
	assert.True(t, isFinal)

	startTime := time.Now()
	assert.NoError(t, execScheduler.Run(ctx, ctx, samples))
	runTime := time.Since(startTime)
	assert.True(t, runTime > 1*time.Second, "test did not take 1s")
	assert.True(t, runTime < 10*time.Second, "took more than 10 seconds")

	assert.Empty(t, hook.Entries)
}

func TestSchedulerEndIterations(t *testing.T) {
	t.Parallel()
	registry := metrics.NewRegistry()
	metric := registry.MustNewMetric("test_metric", metrics.Counter)
	ts := metrics.TimeSeries{
		Metric: metric,
		Tags:   registry.RootTagSet(),
	}

	options, err := executor.DeriveScenariosFromShortcuts(lib.Options{
		VUs:        null.IntFrom(1),
		Iterations: null.IntFrom(100),
	}, nil)
	require.NoError(t, err)
	require.Empty(t, options.Validate())

	var i int64
	runner := &minirunner.MiniRunner{
		Fn: func(ctx context.Context, _ *lib.State, out chan<- metrics.SampleContainer) error {
			select {
			case <-ctx.Done():
			default:
				atomic.AddInt64(&i, 1)
			}
			out <- metrics.Sample{
				TimeSeries: ts,
				Value:      1.0,
			}
			return nil
		},
		Options: options,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testRunState := getTestRunState(t, getTestPreInitState(t), runner.GetOptions(), runner)
	execScheduler, err := execution.NewScheduler(testRunState, local.NewController())
	require.NoError(t, err)

	samples := make(chan metrics.SampleContainer, 300)
	defer close(samples)

	stopEmission, err := execScheduler.Init(ctx, samples)
	require.NoError(t, err)
	defer stopEmission()

	require.NoError(t, execScheduler.Run(ctx, ctx, samples))

	assert.Equal(t, uint64(100), execScheduler.GetState().GetFullIterationCount())
	assert.Equal(t, uint64(0), execScheduler.GetState().GetPartialIterationCount())
	assert.Equal(t, int64(100), i)
	require.Equal(t, 100, len(samples)) // TODO: change to 200 https://github.com/k6io/k6/issues/1250
	for i := 0; i < 100; i++ {
		mySample, ok := <-samples
		require.True(t, ok)
		assert.Equal(t, metrics.Sample{TimeSeries: ts, Value: 1.0}, mySample)
	}
}

func TestSchedulerIsRunning(t *testing.T) {
	t.Parallel()
	runner := &minirunner.MiniRunner{
		Fn: func(ctx context.Context, _ *lib.State, _ chan<- metrics.SampleContainer) error {
			<-ctx.Done()
			return nil
		},
	}
	ctx, cancel, execScheduler, _ := newTestScheduler(t, runner, nil, lib.Options{})
	state := execScheduler.GetState()

	err := make(chan error)
	go func() { err <- execScheduler.Run(ctx, ctx, nil) }()
	for !state.HasStarted() {
		time.Sleep(10 * time.Microsecond)
	}
	cancel()
	for !state.HasEnded() {
		time.Sleep(10 * time.Microsecond)
	}
	assert.NoError(t, <-err)
}

// TestDNSResolverCache checks the DNS resolution behavior at the Scheduler level.
func TestDNSResolverCache(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace
	script := sr(`
		import http from "k6/http";
		import { sleep } from "k6";

		export let options = {
			vus: 1,
			iterations: 8,
			noConnectionReuse: true,
		}

		export default function () {
			const res = http.get("http://myhost:HTTPBIN_PORT/", { timeout: 50 });
			sleep(0.7);  // somewhat uneven multiple of 0.5 to minimize races with asserts
		}`)

	testCases := map[string]struct {
		opts lib.Options
		// Amount of expected resolutions after the IP change.
		// This will demonstrate how the cache is being used, depending on the
		// configured TTL.
		expNewResolutions uint32
	}{
		"default": {
			// IPs are cached for 5m, so no new resolution attempts will be made.
			opts:              lib.Options{DNS: types.DefaultDNSConfig()},
			expNewResolutions: 0,
		},
		"0ok": {
			// The cache is disabled, so every request does a DNS lookup.
			opts: lib.Options{DNS: types.DNSConfig{
				TTL:    null.StringFrom("0"),
				Select: types.NullDNSSelect{DNSSelect: types.DNSfirst, Valid: true},
				Policy: types.NullDNSPolicy{DNSPolicy: types.DNSpreferIPv4, Valid: false},
			}},
			expNewResolutions: 7,
		},
		"1700": {
			// IPs are cached for 1.7s.
			// This also checks that unitless values are interpreted as ms.
			opts: lib.Options{DNS: types.DNSConfig{
				TTL:    null.StringFrom("1700"),
				Select: types.NullDNSSelect{DNSSelect: types.DNSfirst, Valid: true},
				Policy: types.NullDNSPolicy{DNSPolicy: types.DNSpreferIPv4, Valid: false},
			}},
			expNewResolutions: 2,
		},
		"3s": {
			opts: lib.Options{DNS: types.DNSConfig{
				TTL:    null.StringFrom("3s"),
				Select: types.NullDNSSelect{DNSSelect: types.DNSfirst, Valid: true},
				Policy: types.NullDNSPolicy{DNSPolicy: types.DNSpreferIPv4, Valid: false},
			}},
			expNewResolutions: 1,
		},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			logger := logrus.New()
			logger.SetOutput(io.Discard)
			logHook := testutils.NewLogHook(logrus.WarnLevel)
			logger.AddHook(logHook)

			registry := metrics.NewRegistry()
			builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
			runner, err := js.New(
				&lib.TestPreInitState{
					Logger:         logger,
					BuiltinMetrics: builtinMetrics,
					Registry:       registry,
					Usage:          usage.New(),
				},
				&loader.SourceData{
					URL: &url.URL{Path: "/script.js"}, Data: []byte(script),
				}, nil)
			require.NoError(t, err)

			mr := mockresolver.New(map[string][]net.IP{"myhost": {net.ParseIP(sr("HTTPBIN_IP"))}})
			var newResolutions uint32
			resolveHook := func(_ string, result []net.IP) {
				require.Len(t, result, 1)
				if result[0].String() == "127.0.0.1" {
					mr.Set("myhost", "127.0.0.254")
				} else {
					atomic.AddUint32(&newResolutions, 1)
				}
			}
			mr.ResolveHook = resolveHook
			runner.ActualResolver = mr.LookupIPAll

			ctx, cancel, execScheduler, samples := newTestScheduler(t, runner, logger, tc.opts)
			defer cancel()

			errCh := make(chan error, 1)
			go func() { errCh <- execScheduler.Run(ctx, ctx, samples) }()

			select {
			case err := <-errCh:
				require.NoError(t, err)
				assert.Equal(t, tc.expNewResolutions, atomic.LoadUint32(&newResolutions))
			case <-time.After(10 * time.Second):
				t.Fatal("timed out")
			}
		})
	}
}

func TestRealTimeAndSetupTeardownMetrics(t *testing.T) {
	t.Parallel()
	script := []byte(`
	import { Counter } from "k6/metrics";
	import { sleep } from "k6";

	var counter = new Counter("test_counter");

	export function setup() {
		console.log("setup(), sleeping for 1 second");
		counter.add(1, { place: "setupBeforeSleep" });
		sleep(1);
		console.log("setup sleep is done");
		counter.add(2, { place: "setupAfterSleep" });
		return { "some": ["data"], "v": 1 };
	}

	export function teardown(data) {
		console.log("teardown(" + JSON.stringify(data) + "), sleeping for 1 second");
		counter.add(3, { place: "teardownBeforeSleep" });
		sleep(1);
		if (!data || data.v != 1) {
			throw new Error("incorrect data: " + JSON.stringify(data));
		}
		console.log("teardown sleep is done");
		counter.add(4, { place: "teardownAfterSleep" });
	}

	export default function (data) {
		console.log("default(" + JSON.stringify(data) + ") with ENV=" + JSON.stringify(__ENV) + " for in ITER " + __ITER + " and VU " + __VU);
		counter.add(5, { place: "defaultBeforeSleep" });
		if (!data || data.v != 1) {
			throw new Error("incorrect data: " + JSON.stringify(data));
		}
		sleep(1);
		console.log("default() for in ITER " + __ITER + " and VU " + __VU + " done!");
		counter.add(6, { place: "defaultAfterSleep" });
	}`)

	piState := getTestPreInitState(t)
	runner, err := js.New(piState, &loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: script}, nil)
	require.NoError(t, err)

	options, err := executor.DeriveScenariosFromShortcuts(runner.GetOptions().Apply(lib.Options{
		Iterations:      null.IntFrom(2),
		VUs:             null.IntFrom(1),
		SystemTags:      &metrics.DefaultSystemTagSet,
		SetupTimeout:    types.NullDurationFrom(4 * time.Second),
		TeardownTimeout: types.NullDurationFrom(4 * time.Second),
	}), nil)
	require.NoError(t, err)

	testRunState := getTestRunState(t, piState, options, runner)
	execScheduler, err := execution.NewScheduler(testRunState, local.NewController())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	samples := make(chan metrics.SampleContainer)
	defer close(samples)

	stopEmission, err := execScheduler.Init(ctx, samples)
	require.NoError(t, err)

	go func() {
		defer close(done)
		defer stopEmission()
		assert.NoError(t, execScheduler.Run(ctx, ctx, samples))
	}()

	expectIn := func(from, to time.Duration, expected metrics.SampleContainer) {
		start := time.Now()
		from *= time.Millisecond
		to *= time.Millisecond
		for {
			select {
			case sampleContainer := <-samples:
				gotVus := false
				for _, s := range sampleContainer.GetSamples() {
					if s.Metric == piState.BuiltinMetrics.VUs || s.Metric == piState.BuiltinMetrics.VUsMax {
						gotVus = true
						break
					}
				}
				if gotVus {
					continue
				}

				now := time.Now()
				elapsed := now.Sub(start)
				if elapsed < from {
					t.Errorf("Received sample earlier (%s) than expected (%s)", elapsed, from)
					return
				}
				assert.IsType(t, expected, sampleContainer)
				expSamples := expected.GetSamples()
				gotSamples := sampleContainer.GetSamples()
				if assert.Len(t, gotSamples, len(expSamples)) {
					for i, s := range gotSamples {
						expS := expSamples[i]
						if s.Metric.Name != metrics.IterationDurationName {
							assert.Equal(t, expS.Value, s.Value)
						}
						assert.Equal(t, expS.Metric.Name, s.Metric.Name)
						assert.Equal(t, expS.Tags.Map(), s.Tags.Map())
						assert.InDelta(t, 0, now.Sub(s.Time), float64(50*time.Millisecond))
					}
				}
				return
			case <-time.After(to):
				t.Errorf("Did not receive sample in the maximum allotted time (%s)", to)
				return
			}
		}
	}

	getTags := func(r *metrics.Registry, args ...string) *metrics.TagSet {
		tags := r.RootTagSet()
		for i := 0; i < len(args)-1; i += 2 {
			tags = tags.With(args[i], args[i+1])
		}
		return tags
	}
	testCounter, err := piState.Registry.NewMetric("test_counter", metrics.Counter)
	require.NoError(t, err)
	getSample := func(expValue float64, expMetric *metrics.Metric, expTags ...string) metrics.SampleContainer {
		return metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: expMetric,
				Tags:   getTags(piState.Registry, expTags...),
			},
			Time:  time.Now(),
			Value: expValue,
		}
	}
	getNetworkSamples := func(group string, addExpTags ...string) metrics.SampleContainer {
		expTags := []string{"group", group}
		expTags = append(expTags, addExpTags...)
		dialer := netext.NewDialer(
			net.Dialer{},
			netext.NewResolver(net.LookupIP, 0, types.DNSfirst, types.DNSpreferIPv4),
		)

		ctm := metrics.TagsAndMeta{Tags: getTags(piState.Registry, expTags...)}
		return dialer.IOSamples(time.Now(), ctm, piState.BuiltinMetrics)
	}

	getIterationsSamples := func(group string, addExpTags ...string) metrics.SampleContainer {
		expTags := []string{"group", group}
		expTags = append(expTags, addExpTags...)
		ctm := metrics.TagsAndMeta{Tags: getTags(piState.Registry, expTags...)}
		startTime := time.Now()
		endTime := time.Now()

		return metrics.Samples([]metrics.Sample{
			{
				TimeSeries: metrics.TimeSeries{
					Metric: piState.BuiltinMetrics.IterationDuration,
					Tags:   ctm.Tags,
				},
				Time:     endTime,
				Metadata: ctm.Metadata,
				Value:    metrics.D(endTime.Sub(startTime)),
			},
			{
				TimeSeries: metrics.TimeSeries{
					Metric: piState.BuiltinMetrics.Iterations,
					Tags:   ctm.Tags,
				},
				Time:     endTime,
				Metadata: ctm.Metadata,
				Value:    1,
			},
		})
	}

	// Initially give a long time (5s) for the execScheduler to start
	expectIn(0, 5000, getSample(1, testCounter, "group", "::setup", "place", "setupBeforeSleep"))
	expectIn(900, 1100, getSample(2, testCounter, "group", "::setup", "place", "setupAfterSleep"))
	expectIn(0, 100, getNetworkSamples("::setup"))

	expectIn(0, 100, getSample(5, testCounter, "group", "", "place", "defaultBeforeSleep", "scenario", "default"))
	expectIn(900, 1100, getSample(6, testCounter, "group", "", "place", "defaultAfterSleep", "scenario", "default"))
	expectIn(0, 100, getNetworkSamples("", "scenario", "default"))
	expectIn(0, 100, getIterationsSamples("", "scenario", "default"))

	expectIn(0, 100, getSample(5, testCounter, "group", "", "place", "defaultBeforeSleep", "scenario", "default"))
	expectIn(900, 1100, getSample(6, testCounter, "group", "", "place", "defaultAfterSleep", "scenario", "default"))
	expectIn(0, 100, getNetworkSamples("", "scenario", "default"))
	expectIn(0, 100, getIterationsSamples("", "scenario", "default"))

	expectIn(0, 1000, getSample(3, testCounter, "group", "::teardown", "place", "teardownBeforeSleep"))
	expectIn(900, 1100, getSample(4, testCounter, "group", "::teardown", "place", "teardownAfterSleep"))
	expectIn(0, 100, getNetworkSamples("::teardown"))

	for {
		select {
		case s := <-samples:
			t.Fatalf("Did not expect anything in the sample channel bug got %#v", s)
		case <-time.After(3 * time.Second):
			t.Fatalf("Local execScheduler took way to long to finish")
		case <-done:
			return // Exit normally
		}
	}
}

func TestNewSchedulerHasWork(t *testing.T) {
	t.Parallel()
	script := []byte(`
		import http from 'k6/http';

		export let options = {
			executionSegment: "3/4:1",
			executionSegmentSequence: "0,1/4,2/4,3/4,1",
			scenarios: {
				shared_iters1: {
					executor: "shared-iterations",
					vus: 3,
					iterations: 3,
				},
				shared_iters2: {
					executor: "shared-iterations",
					vus: 4,
					iterations: 4,
				},
				constant_arr_rate: {
					executor: "constant-arrival-rate",
					rate: 3,
					timeUnit: "1s",
					duration: "20s",
					preAllocatedVUs: 4,
					maxVUs: 4,
				},
		    },
		};

		export default function() {
			const response = http.get("http://test.loadimpact.com");
		};
`)

	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))
	registry := metrics.NewRegistry()
	piState := &lib.TestPreInitState{
		Logger:         logger,
		Registry:       registry,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Usage:          usage.New(),
	}
	runner, err := js.New(piState, &loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: script}, nil)
	require.NoError(t, err)

	testRunState := getTestRunState(t, piState, runner.GetOptions(), runner)
	execScheduler, err := execution.NewScheduler(testRunState, local.NewController())
	require.NoError(t, err)

	assert.Len(t, execScheduler.GetExecutors(), 2)
	assert.Len(t, execScheduler.GetExecutorConfigs(), 3)

	err = execScheduler.SetPaused(true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "doesn't support pause and resume operations after its start")
}
