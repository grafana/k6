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

	"go.k6.io/k6/core"
	"go.k6.io/k6/core/local"
	"go.k6.io/k6/js"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/loader"
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
				piState, &loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: testCase.script}, nil,
			)
			require.NoError(t, err)
			require.NoError(t, runner.SetOptions(lib.Options{
				Paused:          null.BoolFrom(true),
				VUs:             null.IntFrom(2),
				Iterations:      null.IntFrom(3),
				NoSetup:         null.BoolFrom(true),
				SetupTimeout:    types.NullDurationFrom(5 * time.Second),
				TeardownTimeout: types.NullDurationFrom(5 * time.Second),
			}))
			testState := &lib.TestRunState{
				TestPreInitState: piState,
				Options:          runner.GetOptions(),
				Runner:           runner,
			}

			execScheduler, err := local.NewExecutionScheduler(testState)
			require.NoError(t, err)
			engine, err := core.NewEngine(testState, execScheduler, nil)
			require.NoError(t, err)

			require.NoError(t, engine.OutputManager.StartOutputs())
			defer engine.OutputManager.StopOutputs()

			globalCtx, globalCancel := context.WithCancel(context.Background())
			runCtx, runCancel := context.WithCancel(globalCtx)
			run, wait, err := engine.Init(globalCtx, runCtx)
			require.NoError(t, err)

			defer wait()
			defer globalCancel()

			errC := make(chan error)
			go func() { errC <- run() }()

			handler := NewHandler()

			checkSetup := func(method, body, expResult string) {
				rw := httptest.NewRecorder()
				handler.ServeHTTP(rw, newRequestWithEngine(engine, method, "/v1/setup", bytes.NewBufferString(body)))
				res := rw.Result()
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

			require.NoError(t, engine.ExecutionScheduler.SetPaused(false))

			select {
			case <-time.After(10 * time.Second):
				runCancel()
				t.Fatal("Test timed out")
			case err := <-errC:
				runCancel()
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
