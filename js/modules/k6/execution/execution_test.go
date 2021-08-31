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

package execution_test

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/url"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/core/local"
	"go.k6.io/k6/js"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/loader"
	"go.k6.io/k6/stats"
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

	runner, err := js.New(
		logger,
		&loader.SourceData{
			URL:  &url.URL{Path: "/script.js"},
			Data: script,
		},
		nil,
		lib.RuntimeOptions{},
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
		IdInInstance        uint64
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
			assert.Contains(t, []uint64{1, 2}, le.IdInInstance)
			if _, ok := vuStats[le.IdInInstance]; !ok {
				vuStats[le.IdInInstance] = &vuStat{0, make(map[string]uint64)}
			}
			if le.IterationInInstance > vuStats[le.IdInInstance].iteration {
				vuStats[le.IdInInstance].iteration = le.IterationInInstance
			}
			if le.IterationInScenario > vuStats[le.IdInInstance].scIter[le.Scenario] {
				vuStats[le.IdInInstance].scIter[le.Scenario] = le.IterationInScenario
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

	runner, err := js.New(
		logger,
		&loader.SourceData{
			URL:  &url.URL{Path: "/script.js"},
			Data: script,
		},
		nil,
		lib.RuntimeOptions{},
	)
	require.NoError(t, err)

	ctx, cancel, execScheduler, samples := newTestExecutionScheduler(t, runner, logger, lib.Options{})
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
	logger.SetOutput(ioutil.Discard)
	logHook := testutils.SimpleLogrusHook{HookedLevels: []logrus.Level{logrus.InfoLevel}}
	logger.AddHook(&logHook)

	runner, err := js.New(
		logger,
		&loader.SourceData{
			URL:  &url.URL{Path: "/script.js"},
			Data: script,
		},
		nil,
		lib.RuntimeOptions{},
	)
	require.NoError(t, err)

	ctx, cancel, execScheduler, samples := newTestExecutionScheduler(t, runner, logger, lib.Options{})
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

func TestExecutionInfo(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name, script, expErr string
	}{
		{name: "vu_ok", script: `
		var exec = require('k6/execution');

		exports.default = function() {
			if (exec.vu.idInInstance !== 1) throw new Error('unexpected VU ID: '+exec.vu.idInInstance);
			if (exec.vu.idInTest !== 10) throw new Error('unexpected global VU ID: '+exec.vu.idInTest);
			if (exec.vu.iterationInInstance !== 0) throw new Error('unexpected VU iteration: '+exec.vu.iterationInInstance);
			if (exec.vu.iterationInScenario !== 0) throw new Error('unexpected scenario iteration: '+exec.vu.iterationInScenario);
		}`},
		{name: "vu_err", script: `
		var exec = require('k6/execution');
		exec.vu;
		`, expErr: "getting VU information in the init context is not supported"},
		{name: "scenario_ok", script: `
		var exec = require('k6/execution');
		var sleep = require('k6').sleep;

		exports.default = function() {
			var si = exec.scenario;
			sleep(0.1);
			if (si.name !== 'default') throw new Error('unexpected scenario name: '+si.name);
			if (si.executor !== 'test-exec') throw new Error('unexpected executor: '+si.executor);
			if (si.startTime > new Date().getTime()) throw new Error('unexpected startTime: '+si.startTime);
			if (si.progress !== 0.1) throw new Error('unexpected progress: '+si.progress);
			if (si.iterationInInstance !== 3) throw new Error('unexpected scenario local iteration: '+si.iterationInInstance);
			if (si.iterationInTest !== 4) throw new Error('unexpected scenario local iteration: '+si.iterationInTest);
		}`},
		{name: "scenario_err", script: `
		var exec = require('k6/execution');
		exec.scenario;
		`, expErr: "getting scenario information in the init context is not supported"},
		{name: "test_ok", script: `
		var exec = require('k6/execution');

		exports.default = function() {
			var ti = exec.instance;
			if (ti.currentTestRunDuration !== 0) throw new Error('unexpected test duration: '+ti.currentTestRunDuration);
			if (ti.vusActive !== 1) throw new Error('unexpected vusActive: '+ti.vusActive);
			if (ti.vusInitialized !== 0) throw new Error('unexpected vusInitialized: '+ti.vusInitialized);
			if (ti.iterationsCompleted !== 0) throw new Error('unexpected iterationsCompleted: '+ti.iterationsCompleted);
			if (ti.iterationsInterrupted !== 0) throw new Error('unexpected iterationsInterrupted: '+ti.iterationsInterrupted);
		}`},
		{name: "test_err", script: `
		var exec = require('k6/execution');
		exec.instance;
		`, expErr: "getting instance information in the init context is not supported"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, err := getSimpleRunner(t, "/script.js", tc.script)
			if tc.expErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expErr)
				return
			}
			require.NoError(t, err)

			samples := make(chan stats.SampleContainer, 100)
			initVU, err := r.NewVU(1, 10, samples)
			require.NoError(t, err)

			execScheduler, err := local.NewExecutionScheduler(r, testutils.NewLogger(t))
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ctx = lib.WithExecutionState(ctx, execScheduler.GetState())
			ctx = lib.WithScenarioState(ctx, &lib.ScenarioState{
				Name:      "default",
				Executor:  "test-exec",
				StartTime: time.Now(),
				ProgressFn: func() (float64, []string) {
					return 0.1, nil
				},
			})
			vu := initVU.Activate(&lib.VUActivationParams{
				RunContext:               ctx,
				Exec:                     "default",
				GetNextIterationCounters: func() (uint64, uint64) { return 3, 4 },
			})

			execState := execScheduler.GetState()
			execState.ModCurrentlyActiveVUsCount(+1)
			err = vu.RunOnce()
			assert.NoError(t, err)
		})
	}
}
