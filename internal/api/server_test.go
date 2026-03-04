package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/internal/observability/jsexec"
)

func testHTTPHandler(rw http.ResponseWriter, _ *http.Request) {
	rw.Header().Add("Content-Type", "text/plain; charset=utf-8")
	if _, err := fmt.Fprint(rw, "ok"); err != nil {
		panic(err.Error())
	}
}

func TestLogger(t *testing.T) {
	t.Parallel()
	for _, method := range []string{"GET", "POST", "PUT", "PATCH"} {
		t.Run("method="+method, func(t *testing.T) {
			t.Parallel()
			for _, path := range []string{"/", "/test", "/test/path"} {
				t.Run("path="+path, func(t *testing.T) {
					t.Parallel()
					rw := httptest.NewRecorder()
					r := httptest.NewRequest(method, "http://example.com"+path, nil)

					l, hook := logtest.NewNullLogger()
					l.Level = logrus.DebugLevel
					withLoggingHandler(l, http.HandlerFunc(testHTTPHandler))(rw, r)

					res := rw.Result()
					assert.Equal(t, http.StatusOK, res.StatusCode)
					assert.Equal(t, "text/plain; charset=utf-8", res.Header.Get("Content-Type"))
					assert.NoError(t, res.Body.Close())

					if !assert.Len(t, hook.Entries, 1) {
						return
					}

					e := hook.LastEntry()
					assert.Equal(t, logrus.DebugLevel, e.Level)
					assert.Equal(t, fmt.Sprintf("%s %s", method, path), e.Message)
					assert.Equal(t, http.StatusOK, e.Data["status"])
				})
			}
		})
	}
}

func TestPing(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))
	mux := handlePing(logger)

	rw := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ping", nil)
	mux.ServeHTTP(rw, r)

	res := rw.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, []byte{'o', 'k'}, rw.Body.Bytes())
	assert.NoError(t, res.Body.Close())
}

func TestJSObservabilityStateHandler(t *testing.T) {
	t.Parallel()

	m := jsexec.NewManager(jsexec.Config{
		Enabled:                   true,
		FirstRunnerMemMaxBytes:    1000,
		FirstRunnerMemStepPercent: 10,
	})
	jsexec.Activate(m)
	defer jsexec.Deactivate(m)

	rw := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/js-observability", nil)
	jsObservabilityStateHandler().ServeHTTP(rw, r)

	res := rw.Result()
	defer func() {
		assert.NoError(t, res.Body.Close())
	}()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "application/json", res.Header.Get("Content-Type"))

	var payload struct {
		ProfilingEnabled bool `json:"profiling_enabled"`
		FirstRunnerMem   struct {
			MaxBytes    int64 `json:"max_bytes"`
			StepPercent int64 `json:"step_percent"`
		} `json:"first_runner_memory"`
		Artifacts map[string]bool `json:"artifacts_available"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&payload))
	assert.True(t, payload.ProfilingEnabled)
	assert.Equal(t, int64(1000), payload.FirstRunnerMem.MaxBytes)
	assert.Equal(t, int64(10), payload.FirstRunnerMem.StepPercent)
	assert.Contains(t, payload.Artifacts, "js-cpu")
}

func TestJSObservabilityToggleHandlers(t *testing.T) {
	t.Parallel()

	m := jsexec.NewManager(jsexec.Config{Enabled: false})
	jsexec.Activate(m)
	defer jsexec.Deactivate(m)

	enableRW := httptest.NewRecorder()
	enableReq := httptest.NewRequest(http.MethodPost, "/v1/js-observability/profiling/enable", nil)
	jsObservabilityToggleHandler(true).ServeHTTP(enableRW, enableReq)
	assert.Equal(t, http.StatusOK, enableRW.Result().StatusCode)
	assert.True(t, jsexec.Enabled())

	disableRW := httptest.NewRecorder()
	disableReq := httptest.NewRequest(http.MethodPost, "/v1/js-observability/profiling/disable", nil)
	jsObservabilityToggleHandler(false).ServeHTTP(disableRW, disableReq)
	assert.Equal(t, http.StatusOK, disableRW.Result().StatusCode)
	assert.False(t, jsexec.Enabled())

	getRW := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/v1/js-observability/profiling", nil)
	jsObservabilityProfilingHandler().ServeHTTP(getRW, getReq)
	assert.Equal(t, http.StatusOK, getRW.Result().StatusCode)
	assert.Equal(t, "application/json", getRW.Result().Header.Get("Content-Type"))
}
