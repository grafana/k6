package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/core"
	"go.k6.io/k6/core/local"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils/minirunner"
)

func TestGetStatus(t *testing.T) {
	t.Parallel()

	testState := getTestRunState(t, lib.Options{}, &minirunner.MiniRunner{})
	execScheduler, err := local.NewExecutionScheduler(testState)
	require.NoError(t, err)
	engine, err := core.NewEngine(testState, execScheduler, nil)
	require.NoError(t, err)

	rw := httptest.NewRecorder()
	NewHandler().ServeHTTP(rw, newRequestWithEngine(engine, "GET", "/v1/status", nil))
	res := rw.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)

	t.Run("document", func(t *testing.T) {
		t.Parallel()

		var doc StatusJSONAPI
		assert.NoError(t, json.Unmarshal(rw.Body.Bytes(), &doc))
		assert.Equal(t, "status", doc.Data.Type)
	})

	t.Run("status", func(t *testing.T) {
		t.Parallel()

		var statusEnvelop StatusJSONAPI

		err := json.Unmarshal(rw.Body.Bytes(), &statusEnvelop)
		assert.NoError(t, err)

		status := statusEnvelop.Status()

		assert.True(t, status.Paused.Valid)
		assert.True(t, status.VUs.Valid)
		assert.True(t, status.VUsMax.Valid)
		assert.False(t, status.Stopped)
		assert.False(t, status.Tainted)
	})
}

func TestPatchStatus(t *testing.T) {
	t.Parallel()

	testData := map[string]struct {
		ExpectedStatusCode int
		ExpectedStatus     Status
		Payload            []byte
	}{
		"nothing": {
			ExpectedStatusCode: 200,
			ExpectedStatus:     Status{},
			Payload:            []byte(`{"data":{"type":"status","id":"default","attributes":{"status":0,"paused":null,"vus":null,"vus-max":null,"stopped":false,"running":false,"tainted":false}}}`),
		},
		"paused": {
			ExpectedStatusCode: 200,
			ExpectedStatus:     Status{Paused: null.BoolFrom(true)},
			Payload:            []byte(`{"data":{"type":"status","id":"default","attributes":{"status":0,"paused":true,"vus":null,"vus-max":null,"stopped":false,"running":false,"tainted":false}}}`),
		},
		"max vus": {
			ExpectedStatusCode: 200,
			ExpectedStatus:     Status{VUsMax: null.IntFrom(20)},
			Payload:            []byte(`{"data":{"type":"status","id":"default","attributes":{"status":0,"paused":null,"vus":null,"vus-max":20,"stopped":false,"running":false,"tainted":false}}}`),
		},
		"max vus below initial": {
			ExpectedStatusCode: 400,
			ExpectedStatus:     Status{VUsMax: null.IntFrom(5)},
			Payload:            []byte(`{"data":{"type":"status","id":"default","attributes":{"status":0,"paused":null,"vus":null,"vus-max":5,"stopped":false,"running":false,"tainted":false}}}`),
		},
		"too many vus": {
			ExpectedStatusCode: 400,
			ExpectedStatus:     Status{VUs: null.IntFrom(10), VUsMax: null.IntFrom(0)},
			Payload:            []byte(`{"data":{"type":"status","id":"default","attributes":{"status":0,"paused":null,"vus":10,"vus-max":0,"stopped":false,"running":false,"tainted":false}}}`),
		},
		"vus": {
			ExpectedStatusCode: 200,
			ExpectedStatus:     Status{VUs: null.IntFrom(10), VUsMax: null.IntFrom(10)},
			Payload:            []byte(`{"data":{"type":"status","id":"default","attributes":{"status":0,"paused":null,"vus":10,"vus-max":10,"stopped":false,"running":false,"tainted":false}}}`),
		},
	}

	for name, testCase := range testData {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			scenarios := lib.ScenarioConfigs{}
			err := json.Unmarshal([]byte(`
			{"external": {"executor": "externally-controlled",
			"vus": 0, "maxVUs": 10, "duration": "0"}}`), &scenarios)
			require.NoError(t, err)

			testState := getTestRunState(t, lib.Options{Scenarios: scenarios}, &minirunner.MiniRunner{})
			execScheduler, err := local.NewExecutionScheduler(testState)
			require.NoError(t, err)
			engine, err := core.NewEngine(testState, execScheduler, nil)
			require.NoError(t, err)

			require.NoError(t, engine.OutputManager.StartOutputs())
			defer engine.OutputManager.StopOutputs()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			run, wait, err := engine.Init(ctx, ctx)
			require.NoError(t, err)

			defer func() {
				cancel()
				wait()
			}()

			go func() {
				assert.NoError(t, run())
			}()
			// wait for the executor to initialize to avoid a potential data race below
			time.Sleep(200 * time.Millisecond)

			rw := httptest.NewRecorder()
			NewHandler().ServeHTTP(rw, newRequestWithEngine(engine, "PATCH", "/v1/status", bytes.NewReader(testCase.Payload)))
			res := rw.Result()

			require.Equal(t, testCase.ExpectedStatusCode, res.StatusCode)

			if testCase.ExpectedStatusCode != 200 {
				return
			}

			status := NewStatus(engine)
			if testCase.ExpectedStatus.Paused.Valid {
				assert.Equal(t, testCase.ExpectedStatus.Paused, status.Paused)
			}
			if testCase.ExpectedStatus.VUs.Valid {
				assert.Equal(t, testCase.ExpectedStatus.VUs, status.VUs)
			}
			if testCase.ExpectedStatus.VUsMax.Valid {
				assert.Equal(t, testCase.ExpectedStatus.VUsMax, status.VUsMax)
			}
		})
	}
}
