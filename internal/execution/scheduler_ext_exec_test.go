package execution_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/js"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/internal/loader"
	"go.k6.io/k6/internal/usage"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

// TODO: rewrite and/or move these as integration tests to reduce boilerplate
// and improve reliability?
func TestExecutionInfoVUSharing(t *testing.T) {
	t.Parallel()
	script := []byte(`
		import exec from 'k6/execution';
		import { sleep } from 'k6';

		// The cvus scenario should reuse the two VUs created for the carr scenario.
		export let options = {
			scenarios: {
				carr: {
					executor: 'constant-arrival-rate',
					exec: 'carr',
					rate: 9,
					timeUnit: '0.95s',
					duration: '1s',
					preAllocatedVUs: 2,
					maxVUs: 10,
					gracefulStop: '100ms',
				},
			    cvus: {
					executor: 'constant-vus',
					exec: 'cvus',
					vus: 2,
					duration: '1s',
					startTime: '2s',
					gracefulStop: '0s',
			    },
		    },
		};

		export function cvus() {
			const info = Object.assign({scenario: 'cvus'}, exec.vu);
			console.log(JSON.stringify(info));
			sleep(0.2);
		};

		export function carr() {
			const info = Object.assign({scenario: 'carr'}, exec.vu);
			console.log(JSON.stringify(info));
		};
`)

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	logHook := testutils.NewLogHook(logrus.InfoLevel)
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
			URL:  &url.URL{Path: "/script.js"},
			Data: script,
		},
		nil,
	)
	require.NoError(t, err)

	ctx, cancel, execScheduler, samples := newTestScheduler(t, runner, logger, lib.Options{})
	defer cancel()

	type vuStat struct {
		iteration uint64
		scIter    map[string]uint64
	}
	vuStats := map[uint64]*vuStat{}

	type logEntry struct {
		IDInInstance        uint64
		Scenario            string
		IterationInInstance uint64
		IterationInScenario uint64
	}

	errCh := make(chan error, 1)
	go func() { errCh <- execScheduler.Run(ctx, ctx, samples) }()

	select {
	case err := <-errCh:
		require.NoError(t, err)
		entries := logHook.Drain()
		assert.InDelta(t, 20, len(entries), 2)
		le := &logEntry{}
		for _, entry := range entries {
			err = json.Unmarshal([]byte(entry.Message), le)
			require.NoError(t, err)
			assert.Contains(t, []uint64{1, 2}, le.IDInInstance)
			if _, ok := vuStats[le.IDInInstance]; !ok {
				vuStats[le.IDInInstance] = &vuStat{0, make(map[string]uint64)}
			}
			if le.IterationInInstance > vuStats[le.IDInInstance].iteration {
				vuStats[le.IDInInstance].iteration = le.IterationInInstance
			}
			if le.IterationInScenario > vuStats[le.IDInInstance].scIter[le.Scenario] {
				vuStats[le.IDInInstance].scIter[le.Scenario] = le.IterationInScenario
			}
		}
		require.Len(t, vuStats, 2)
		// Both VUs should complete 10 iterations each globally, but 5
		// iterations each per scenario (iterations are 0-based)
		for _, v := range vuStats {
			assert.Equal(t, uint64(9), v.iteration)
			assert.Equal(t, uint64(4), v.scIter["cvus"])
			assert.Equal(t, uint64(4), v.scIter["carr"])
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out")
	}
}

func TestExecutionInfoScenarioIter(t *testing.T) {
	t.Parallel()
	script := []byte(`
		import exec from 'k6/execution';

		// The pvu scenario should reuse the two VUs created for the carr scenario.
		export let options = {
			scenarios: {
				carr: {
					executor: 'constant-arrival-rate',
					exec: 'carr',
					rate: 9,
					timeUnit: '0.95s',
					duration: '1s',
					preAllocatedVUs: 2,
					maxVUs: 10,
					gracefulStop: '100ms',
				},
				pvu: {
					executor: 'per-vu-iterations',
					exec: 'pvu',
					vus: 2,
					iterations: 5,
					startTime: '2s',
					gracefulStop: '100ms',
				},
			},
		};

		export function pvu() {
			const info = Object.assign({VUID: __VU}, exec.scenario);
			console.log(JSON.stringify(info));
		}

		export function carr() {
			const info = Object.assign({VUID: __VU}, exec.scenario);
			console.log(JSON.stringify(info));
		};
`)

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	logHook := testutils.NewLogHook(logrus.InfoLevel)
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
			URL:  &url.URL{Path: "/script.js"},
			Data: script,
		},
		nil,
	)
	require.NoError(t, err)

	ctx, cancel, execScheduler, samples := newTestScheduler(t, runner, logger, lib.Options{})
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- execScheduler.Run(ctx, ctx, samples) }()

	scStats := map[string]uint64{}

	type logEntry struct {
		Name                      string
		IterationInInstance, VUID uint64
	}

	select {
	case err := <-errCh:
		require.NoError(t, err)
		entries := logHook.Drain()
		require.Len(t, entries, 20)
		le := &logEntry{}
		for _, entry := range entries {
			err = json.Unmarshal([]byte(entry.Message), le)
			require.NoError(t, err)
			assert.Contains(t, []uint64{1, 2}, le.VUID)
			if le.IterationInInstance > scStats[le.Name] {
				scStats[le.Name] = le.IterationInInstance
			}
		}
		require.Len(t, scStats, 2)
		// The global per scenario iteration count should be 9 (iterations
		// start at 0), despite VUs being shared or more than 1 being used.
		for _, v := range scStats {
			assert.Equal(t, uint64(9), v)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out")
	}
}

// Ensure that scenario iterations returned from k6/execution are
// stable during the execution of an iteration.
func TestSharedIterationsStable(t *testing.T) {
	t.Parallel()
	script := []byte(`
		import { sleep } from 'k6';
		import exec from 'k6/execution';

		export let options = {
			scenarios: {
				test: {
					executor: 'shared-iterations',
					vus: 50,
					iterations: 50,
				},
			},
		};
		export default function () {
			sleep(1);
			console.log(JSON.stringify(Object.assign({VUID: __VU}, exec.scenario)));
		}
`)

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	logHook := testutils.NewLogHook(logrus.InfoLevel)
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
			URL:  &url.URL{Path: "/script.js"},
			Data: script,
		},
		nil,
	)
	require.NoError(t, err)

	ctx, cancel, execScheduler, samples := newTestScheduler(t, runner, logger, lib.Options{})
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- execScheduler.Run(ctx, ctx, samples) }()

	expIters := [50]int64{}
	for i := 0; i < 50; i++ {
		expIters[i] = int64(i)
	}
	gotLocalIters, gotGlobalIters := []int64{}, []int64{}

	type logEntry struct{ IterationInInstance, IterationInTest int64 }

	select {
	case err := <-errCh:
		require.NoError(t, err)
		entries := logHook.Drain()
		require.Len(t, entries, 50)
		le := &logEntry{}
		for _, entry := range entries {
			err = json.Unmarshal([]byte(entry.Message), le)
			require.NoError(t, err)
			require.Equal(t, le.IterationInInstance, le.IterationInTest)
			gotLocalIters = append(gotLocalIters, le.IterationInInstance)
			gotGlobalIters = append(gotGlobalIters, le.IterationInTest)
		}

		assert.ElementsMatch(t, expIters, gotLocalIters)
		assert.ElementsMatch(t, expIters, gotGlobalIters)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}
}

func TestExecutionInfoAll(t *testing.T) {
	t.Parallel()

	scriptTemplate := `
	import { sleep } from 'k6';
	import exec from "k6/execution";

	export let options = {
		scenarios: {
			executor: {
				executor: "%[1]s",
				%[2]s
			}
		}
	}

	export default function () {
		sleep(0.2);
		console.log(JSON.stringify(exec));
	}`

	executorConfigs := map[string]string{
		"constant-arrival-rate": `
			rate: 1,
			timeUnit: "1s",
			duration: "1s",
			preAllocatedVUs: 1,
			maxVUs: 2,
			gracefulStop: "0s",`,
		"constant-vus": `
			vus: 1,
			duration: "1s",
			gracefulStop: "0s",`,
		"externally-controlled": `
			vus: 1,
			duration: "1s",`,
		"per-vu-iterations": `
			vus: 1,
			iterations: 1,
			gracefulStop: "0s",`,
		"shared-iterations": `
			vus: 1,
			iterations: 1,
			gracefulStop: "0s",`,
		"ramping-arrival-rate": `
			startRate: 1,
			timeUnit: "0.5s",
			preAllocatedVUs: 1,
			maxVUs: 2,
			stages: [ { target: 1, duration: "1s" } ],
			gracefulStop: "0s",`,
		"ramping-vus": `
			startVUs: 1,
			stages: [ { target: 1, duration: "1s" } ],
			gracefulStop: "0s",`,
	}

	testCases := []struct{ name, script string }{}

	for ename, econf := range executorConfigs {
		testCases = append(testCases, struct{ name, script string }{
			ename, fmt.Sprintf(scriptTemplate, ename, econf),
		})
	}

	// We're only checking a small subset of all properties, to ensure
	// there were no errors with accessing any of the top-level ones.
	// Most of the others are time-based, and would be difficult/flaky to check.
	type logEntry struct {
		Scenario struct{ Executor string }
		Instance struct{ VUsActive int }
		VU       struct{ IDInTest int }
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			logger := logrus.New()
			logger.SetOutput(io.Discard)
			logHook := testutils.NewLogHook(logrus.InfoLevel)
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
					URL:  &url.URL{Path: "/script.js"},
					Data: []byte(tc.script),
				}, nil)
			require.NoError(t, err)

			ctx, cancel, execScheduler, samples := newTestScheduler(t, runner, logger, lib.Options{})
			defer cancel()

			errCh := make(chan error, 1)
			go func() { errCh <- execScheduler.Run(ctx, ctx, samples) }()

			select {
			case err := <-errCh:
				require.NoError(t, err)
				entries := logHook.Drain()
				require.GreaterOrEqual(t, len(entries), 1)

				le := &logEntry{}
				err = json.Unmarshal([]byte(entries[0].Message), le)
				require.NoError(t, err)

				assert.Equal(t, tc.name, le.Scenario.Executor)
				assert.Equal(t, 1, le.Instance.VUsActive)
				assert.Equal(t, 1, le.VU.IDInTest)
			case <-time.After(5 * time.Second):
				t.Fatal("timed out")
			}
		})
	}
}
