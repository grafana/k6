package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"

	"go.k6.io/k6/internal/lib/testutils"
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
		method := method
		t.Run("method="+method, func(t *testing.T) {
			t.Parallel()
			for _, path := range []string{"/", "/test", "/test/path"} {
				path := path
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
