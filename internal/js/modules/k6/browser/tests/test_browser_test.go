package tests

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/env"
	"go.k6.io/k6/v2/internal/lib/testutils/httpmultibin"
)

func TestTestBrowserWithFileServer(t *testing.T) {
	t.Parallel()

	tb := &testBrowser{
		t:    t,
		http: &httpmultibin.HTTPMultiBin{Mux: http.NewServeMux()},
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/page1.html", nil)
	tb.withFileServer().http.Mux.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), "<h1>Page 1</h1>")
}

func TestTestBrowserAwaitWithTimeoutShortCircuit(t *testing.T) {
	t.Parallel()
	tb := newTestBrowser(t)
	start := time.Now()
	require.NoError(t, tb.awaitWithTimeout(time.Second*10, func() error {
		runtime.Goexit() // this is what happens when a `require` fails
		return nil
	}))
	require.Less(t, time.Since(start), time.Second)
}

// testingT is a wrapper around testing.TB.
type testingT struct {
	testing.TB
	fatalfCalled bool
}

// Fatalf skips the test immediately after a test is calling it.
// This is useful when a test is expected to fail, but we don't
// want to mark it as a failure since it's expected.
func (t *testingT) Fatalf(format string, args ...any) {
	t.fatalfCalled = true
	t.SkipNow()
}

func TestTestBrowserWithLookupFunc(t *testing.T) {
	// Skip until we get answer from Chromium team in an open issue
	// https://issues.chromium.org/issues/364089353.
	t.Skip("Skipping until we get response from Chromium team")
	t.Parallel()

	tt := &testingT{TB: t}
	// this operation is expected to fail because the remote debugging port is
	// invalid, practically testing that the InitEnv.LookupEnv is used.
	_ = newTestBrowser(
		tt,
		withEnvLookup(env.ConstLookup(env.BrowserArguments, "remote-debugging-port=99999")),
	)
	require.True(t, tt.fatalfCalled)
}
