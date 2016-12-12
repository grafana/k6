package api

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	logtest "github.com/Sirupsen/logrus/hooks/test"
	"github.com/loadimpact/k6/api/common"
	"github.com/loadimpact/k6/lib"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/negroni"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testHTTPHandler(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Add("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(rw, "ok")
}

func TestLogger(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PUT", "PATCH"} {
		t.Run("method="+method, func(t *testing.T) {
			for _, path := range []string{"/", "/test", "/test/path"} {
				t.Run("path="+path, func(t *testing.T) {
					rw := httptest.NewRecorder()
					r := httptest.NewRequest(method, "http://example.com"+path, nil)

					l, hook := logtest.NewNullLogger()
					l.Level = log.DebugLevel
					NewLogger(l)(negroni.NewResponseWriter(rw), r, testHTTPHandler)

					res := rw.Result()
					assert.Equal(t, http.StatusOK, res.StatusCode)
					assert.Equal(t, "text/plain; charset=utf-8", res.Header.Get("Content-Type"))

					if !assert.Len(t, hook.Entries, 1) {
						return
					}

					e := hook.LastEntry()
					assert.Equal(t, log.DebugLevel, e.Level)
					assert.Equal(t, fmt.Sprintf("%s %s", method, path), e.Message)
					assert.Equal(t, http.StatusOK, e.Data["status"])
				})
			}
		})
	}
}

func TestWithEngine(t *testing.T) {
	engine, err := lib.NewEngine(nil)
	if !assert.NoError(t, err) {
		return
	}

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://example.com/", nil)
	WithEngine(engine)(rw, r, func(rw http.ResponseWriter, r *http.Request) {
		assert.Equal(t, engine, common.GetEngine(r.Context()))
	})
}

func TestPing(t *testing.T) {
	mux := NewHandler()

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ping", nil)
	mux.ServeHTTP(rw, r)

	res := rw.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, []byte{'o', 'k'}, rw.Body.Bytes())
}
