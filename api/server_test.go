package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/api/common"
	"go.k6.io/k6/core"
	"go.k6.io/k6/core/local"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/testutils/minirunner"
	"go.k6.io/k6/metrics"
)

func testHTTPHandler(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Add("Content-Type", "text/plain; charset=utf-8")
	if _, err := fmt.Fprint(rw, "ok"); err != nil {
		panic(err.Error())
	}
}

func TestLogger(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PUT", "PATCH"} {
		t.Run("method="+method, func(t *testing.T) {
			for _, path := range []string{"/", "/test", "/test/path"} {
				t.Run("path="+path, func(t *testing.T) {
					rw := httptest.NewRecorder()
					r := httptest.NewRequest(method, "http://example.com"+path, nil)

					l, hook := logtest.NewNullLogger()
					l.Level = logrus.DebugLevel
					newLogger(l, http.HandlerFunc(testHTTPHandler))(rw, r)

					res := rw.Result()
					assert.Equal(t, http.StatusOK, res.StatusCode)
					assert.Equal(t, "text/plain; charset=utf-8", res.Header.Get("Content-Type"))

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

func TestWithEngine(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))
	registry := metrics.NewRegistry()
	testState := &lib.TestRunState{
		TestPreInitState: &lib.TestPreInitState{
			Logger:         logger,
			Registry:       registry,
			BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		},
		Options: lib.Options{},
		Runner:  &minirunner.MiniRunner{},
	}

	execScheduler, err := local.NewExecutionScheduler(testState)
	require.NoError(t, err)
	engine, err := core.NewEngine(testState, execScheduler, nil)
	require.NoError(t, err)

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://example.com/", nil)
	withEngine(engine, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		assert.Equal(t, engine, common.GetEngine(r.Context()))
	}))(rw, r)
}

func TestPing(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))
	mux := newHandler(logger)

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ping", nil)
	mux.ServeHTTP(rw, r)

	res := rw.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, []byte{'o', 'k'}, rw.Body.Bytes())
}
