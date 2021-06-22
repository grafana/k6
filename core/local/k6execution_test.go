/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
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

package local

import (
	"encoding/json"
	"io/ioutil"
	"net/url"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/loader"
)

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
	logger.SetOutput(ioutil.Discard)
	logHook := testutils.SimpleLogrusHook{HookedLevels: []logrus.Level{logrus.InfoLevel}}
	logger.AddHook(&logHook)

	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	runner, err := js.New(
		logger,
		&loader.SourceData{
			URL:  &url.URL{Path: "/script.js"},
			Data: script,
		},
		nil,
		lib.RuntimeOptions{},
		builtinMetrics,
		registry,
	)
	require.NoError(t, err)

	ctx, cancel, execScheduler, samples := newTestExecutionScheduler(t, runner, logger, lib.Options{})
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
	go func() { errCh <- execScheduler.Run(ctx, ctx, samples, builtinMetrics) }()

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
	logger.SetOutput(ioutil.Discard)
	logHook := testutils.SimpleLogrusHook{HookedLevels: []logrus.Level{logrus.InfoLevel}}
	logger.AddHook(&logHook)

	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	runner, err := js.New(
		logger,
		&loader.SourceData{
			URL:  &url.URL{Path: "/script.js"},
			Data: script,
		},
		nil,
		lib.RuntimeOptions{},
		builtinMetrics,
		registry,
	)
	require.NoError(t, err)

	ctx, cancel, execScheduler, samples := newTestExecutionScheduler(t, runner, logger, lib.Options{})
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- execScheduler.Run(ctx, ctx, samples, builtinMetrics) }()

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
	logger.SetOutput(ioutil.Discard)
	logHook := testutils.SimpleLogrusHook{HookedLevels: []logrus.Level{logrus.InfoLevel}}
	logger.AddHook(&logHook)

	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	runner, err := js.New(
		logger,
		&loader.SourceData{
			URL:  &url.URL{Path: "/script.js"},
			Data: script,
		},
		nil,
		lib.RuntimeOptions{},
		builtinMetrics,
		registry,
	)
	require.NoError(t, err)

	ctx, cancel, execScheduler, samples := newTestExecutionScheduler(t, runner, logger, lib.Options{})
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- execScheduler.Run(ctx, ctx, samples, builtinMetrics) }()

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
