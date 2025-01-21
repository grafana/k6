package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/internal/execution"
	"go.k6.io/k6/internal/execution/local"
	"go.k6.io/k6/internal/js"
	"go.k6.io/k6/internal/loader"
	"go.k6.io/k6/internal/metrics/engine"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
)

func TestSetupData(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name      string
		script    []byte
		setupRuns [][3]string
	}{
		{
			name: "setupReturns",
			script: []byte(`
			export function setup() {
				return {"v": 1};
			}

			export default function(data) {
				if (data !== undefined) {
					throw new Error("incorrect data: " + JSON.stringify(data));
				}
			};

			export function teardown(data) {
				if (data !== undefined) {
					throw new Error("incorrect teardown data: " + JSON.stringify(data));
				}
			} `),
			setupRuns: [][3]string{
				{"GET", "", "{}"},
				{"POST", "", `{"data": {"v":1}}`},
				{"GET", "", `{"data": {"v":1}}`},
				{"PUT", `{"v":2, "test":"mest"}`, `{"data": {"v":2, "test":"mest"}}`},
				{"GET", "", `{"data": {"v":2, "test":"mest"}}`},
				{"PUT", "", `{}`},
				{"GET", "", `{}`},
			},
		}, {
			name: "noSetup",
			script: []byte(`
			export default function(data) {
				if (!data || data.v != 2) {
					throw new Error("incorrect data: " + JSON.stringify(data));
				}
			};

			export function teardown(data) {
				if (!data || data.v != 2) {
					throw new Error("incorrect teardown data: " + JSON.stringify(data));
				}
			} `),
			setupRuns: [][3]string{
				{"GET", "", "{}"},
				{"POST", "", `{}`},
				{"GET", "", `{}`},
				{"PUT", `{"v":2, "test":"mest"}`, `{"data": {"v":2, "test":"mest"}}`},
				{"GET", "", `{"data": {"v":2, "test":"mest"}}`},
				{"PUT", "", `{}`},
				{"GET", "", `{}`},
				{"PUT", `{"v":2, "test":"mest"}`, `{"data": {"v":2, "test":"mest"}}`},
				{"GET", "", `{"data": {"v":2, "test":"mest"}}`},
			},
		}, {
			name: "setupNoReturn",
			script: []byte(`
			export function setup() {
				let a = {"v": 1};
			}
			export default function(data) {
				if (data === undefined || data !== "") {
					throw new Error("incorrect data: " + JSON.stringify(data));
				}
			};

			export function teardown(data) {
				if (data === undefined || data !== "") {
					throw new Error("incorrect teardown data: " + JSON.stringify(data));
				}
			} `),
			setupRuns: [][3]string{
				{"GET", "", "{}"},
				{"POST", "", `{}`},
				{"GET", "", `{}`},
				{"PUT", `{"v":2, "test":"mest"}`, `{"data": {"v":2, "test":"mest"}}`},
				{"GET", "", `{"data": {"v":2, "test":"mest"}}`},
				{"PUT", "\"\"", `{"data": ""}`},
				{"GET", "", `{"data": ""}`},
			},
		},
	}

	runTestCase := func(t *testing.T, tcid int) {
		testCase := testCases[tcid]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			piState := getTestPreInitState(t)
			runner, err := js.New(
				piState,
				&loader.SourceData{
					URL:  &url.URL{Path: "/script.js"},
					Data: testCase.script,
				},
				nil,
			)
			require.NoError(t, err)

			testState := getTestRunState(t, lib.Options{
				Paused:          null.BoolFrom(true),
				VUs:             null.IntFrom(2),
				Iterations:      null.IntFrom(3),
				NoSetup:         null.BoolFrom(true),
				SetupTimeout:    types.NullDurationFrom(5 * time.Second),
				TeardownTimeout: types.NullDurationFrom(5 * time.Second),
			}, runner)

			execScheduler, err := execution.NewScheduler(testState, local.NewController())
			require.NoError(t, err)
			metricsEngine, err := engine.NewMetricsEngine(testState.Registry, testState.Logger)
			require.NoError(t, err)

			globalCtx, globalCancel := context.WithCancel(context.Background())
			defer globalCancel()
			runCtx, runAbort := execution.NewTestRunContext(globalCtx, testState.Logger)
			defer runAbort(fmt.Errorf("unexpected abort"))

			outputManager := output.NewManager([]output.Output{metricsEngine.CreateIngester()}, testState.Logger, runAbort)
			samples := make(chan metrics.SampleContainer, 1000)
			_, stopOutputs, err := outputManager.Start(samples)
			require.NoError(t, err)
			defer stopOutputs(nil)

			cs := &ControlSurface{
				RunCtx:        runCtx,
				Samples:       samples,
				MetricsEngine: metricsEngine,
				Scheduler:     execScheduler,
				RunState:      testState,
			}

			errC := make(chan error)
			go func() { errC <- execScheduler.Run(globalCtx, runCtx, samples) }()

			handler := NewHandler(cs)

			checkSetup := func(method, body, expResult string) {
				rw := httptest.NewRecorder()
				handler.ServeHTTP(rw, httptest.NewRequest(method, "/v1/setup", bytes.NewBufferString(body)))
				res := rw.Result()
				t.Cleanup(func() {
					assert.NoError(t, res.Body.Close())
				})
				if !assert.Equal(t, http.StatusOK, res.StatusCode) {
					t.Logf("body: %s\n", rw.Body.String())
					return
				}

				var doc setUpJSONAPI
				assert.NoError(t, json.Unmarshal(rw.Body.Bytes(), &doc))
				assert.Equal(t, "setupData", doc.Data.Type)

				encoded, err := json.Marshal(doc.Data.Attributes)
				assert.NoError(t, err)
				assert.JSONEq(t, expResult, string(encoded))
			}

			for _, setupRun := range testCase.setupRuns {
				checkSetup(setupRun[0], setupRun[1], setupRun[2])
			}

			require.NoError(t, cs.Scheduler.SetPaused(false))

			select {
			case <-time.After(10 * time.Second):
				t.Fatal("Test timed out")
			case err := <-errC:
				close(samples)
				require.NoError(t, err)
			}
		})
	}

	for id := range testCases {
		id := id
		t.Run(fmt.Sprintf("testcase_%d", id), func(t *testing.T) {
			t.Parallel()
			runTestCase(t, id)
		})
	}
}
