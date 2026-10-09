package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/lib/testutils/minirunner"
	"go.k6.io/k6/v2/lib"
)

func TestGetStatus(t *testing.T) {
	t.Parallel()

	testState := getTestRunState(t, lib.Options{}, &minirunner.MiniRunner{})
	cs := getControlSurface(t, testState)

	rw := httptest.NewRecorder()
	NewHandler(cs).ServeHTTP(rw, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/status", nil))
	res := rw.Result()
	t.Cleanup(func() {
		assert.NoError(t, res.Body.Close())
	})
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
		assert.Nil(t, status.ExecutionResult)
		assert.Contains(t, rw.Body.String(), `"execution_result":null`)
	})
}

func TestGetStatusRunning(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		started     bool
		ended       bool
		status      lib.ExecutionStatus
		wantRunning bool
	}{
		{name: "not started", status: lib.ExecutionStatusInitDone, wantRunning: false},
		{name: "started", started: true, status: lib.ExecutionStatusStarted, wantRunning: true},
		{name: "setup", started: true, status: lib.ExecutionStatusSetup, wantRunning: true},
		{name: "running", started: true, status: lib.ExecutionStatusRunning, wantRunning: true},
		{name: "teardown", started: true, status: lib.ExecutionStatusTeardown, wantRunning: true},
		{name: "ended", started: true, ended: true, status: lib.ExecutionStatusEnded, wantRunning: false},
		// Aborted and failed runs never get an end time, see
		// https://github.com/grafana/k6/issues/6073
		{name: "interrupted", started: true, status: lib.ExecutionStatusInterrupted, wantRunning: false},
		{name: "marked as failed", started: true, status: lib.ExecutionStatusMarkedAsFailed, wantRunning: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cs := getControlSurface(t, getTestRunState(t, lib.Options{}, &minirunner.MiniRunner{}))
			state := cs.Scheduler.GetState()
			if tc.started {
				state.MarkStarted()
			}
			if tc.ended {
				state.MarkEnded()
			}
			state.SetExecutionStatus(tc.status)

			rw := httptest.NewRecorder()
			NewHandler(cs).ServeHTTP(
				rw, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/status", nil),
			)

			var statusEnvelop StatusJSONAPI
			require.NoError(t, json.Unmarshal(rw.Body.Bytes(), &statusEnvelop))
			assert.Equal(t, tc.wantRunning, statusEnvelop.Status().Running)
		})
	}
}

func TestGetStatusExecutionResult(t *testing.T) {
	t.Parallel()

	testState := getTestRunState(t, lib.Options{}, &minirunner.MiniRunner{})
	cs := getControlSurface(t, testState)
	cs.Scheduler.GetState().SetExecutionResult(108)

	rw := httptest.NewRecorder()
	NewHandler(cs).ServeHTTP(
		rw, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/status", nil),
	)

	var statusEnvelop StatusJSONAPI
	require.NoError(t, json.Unmarshal(rw.Body.Bytes(), &statusEnvelop))
	assert.Equal(t, &ExecutionResult{ExitCode: 108}, statusEnvelop.Status().ExecutionResult)
}
