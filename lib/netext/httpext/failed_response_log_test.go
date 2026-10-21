package httpext

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

func TestMakeRequestLogFailedResponses(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "value")
		switch r.URL.Path {
		case "/ok":
			_, _ = w.Write([]byte("all good"))
		case "/notfound":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found body"))
		case "/large":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(strings.Repeat("a", maxLoggedFailedResponseBodySize+10)))
		}
	}))
	t.Cleanup(srv.Close)

	// A closed listener, so that connecting to it results in a transport error.
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	closedURL := "http://" + ln.Addr().String() + "/"
	require.NoError(t, ln.Close())

	defaultCallback := func(status int) bool { return status >= 200 && status < 400 }

	testCases := []struct {
		name             string
		url              string
		enabled          bool
		responseCallback func(int) bool
		throw            bool
		expectLog        bool
		expectReason     string
		expectStatus     int
		expectBody       string
		expectError      bool
	}{
		{name: "disabled", url: srv.URL + "/notfound", enabled: false, expectLog: false},
		{name: "success", url: srv.URL + "/ok", enabled: true, responseCallback: defaultCallback},
		{
			name: "status 404 without callback", url: srv.URL + "/notfound", enabled: true,
			expectLog: true, expectReason: FailedResponseReasonUnexpected, expectStatus: 404,
			expectBody: "not found body",
		},
		{
			name: "status 404 with default callback", url: srv.URL + "/notfound", enabled: true,
			responseCallback: defaultCallback,
			expectLog:        true, expectReason: FailedResponseReasonUnexpected, expectStatus: 404,
			expectBody: "not found body",
		},
		{
			name: "status 404 expected by callback", url: srv.URL + "/notfound", enabled: true,
			responseCallback: func(int) bool { return true }, expectLog: false,
		},
		{
			name: "status 200 unexpected by callback", url: srv.URL + "/ok", enabled: true,
			responseCallback: func(int) bool { return false },
			expectLog:        true, expectReason: FailedResponseReasonUnexpected, expectStatus: 200,
			expectBody: "all good",
		},
		{
			name: "large body is truncated", url: srv.URL + "/large", enabled: true,
			expectLog: true, expectReason: FailedResponseReasonUnexpected, expectStatus: 500,
			expectBody: strings.Repeat("a", maxLoggedFailedResponseBodySize) + "... [truncated, 65546 bytes total]",
		},
		{
			name: "transport error", url: closedURL, enabled: true, responseCallback: defaultCallback,
			expectLog: true, expectReason: FailedResponseReasonRequestError, expectError: true,
		},
		{
			name: "transport error with throw", url: closedURL, enabled: true, throw: true,
			expectLog: true, expectReason: FailedResponseReasonRequestError, expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			hook := testutils.NewLogHook(logrus.WarnLevel)
			logger := logrus.New()
			logger.AddHook(hook)
			logger.Out = &bytes.Buffer{}

			registry := metrics.NewRegistry()
			state := &lib.State{
				Options: lib.Options{
					SystemTags:         &metrics.DefaultSystemTagSet,
					LogFailedResponses: null.BoolFrom(tc.enabled),
				},
				Transport:      http.DefaultTransport,
				Samples:        make(chan metrics.SampleContainer, 10),
				Logger:         logger,
				BufferPool:     lib.NewBufferPool(),
				BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
				Tags:           lib.NewVUStateTags(registry.RootTagSet()),
				VUID:           3,
				Iteration:      7,
			}

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, tc.url, nil)
			require.NoError(t, err)
			preq := &ParsedHTTPRequest{
				Req:              req,
				URL:              &URL{u: req.URL, URL: tc.url},
				Timeout:          10 * time.Second,
				ResponseType:     ResponseTypeText,
				ResponseCallback: tc.responseCallback,
				Throw:            tc.throw,
				TagsAndMeta:      state.Tags.GetCurrentValues(),
			}
			_, err = MakeRequest(context.Background(), state, preq)
			if tc.throw {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			var entries []logrus.Entry
			for _, e := range hook.Drain() {
				if e.Data["source"] == "failed-response" {
					entries = append(entries, e)
				}
			}
			if !tc.expectLog {
				require.Empty(t, entries)
				return
			}
			require.Len(t, entries, 1)
			e := entries[0]
			assert.Equal(t, logrus.WarnLevel, e.Level)
			assert.Equal(t, "Failed HTTP response", e.Message)
			assert.Equal(t, tc.expectReason, e.Data["reason"])
			assert.Equal(t, http.MethodGet, e.Data["method"])
			assert.Equal(t, uint64(3), e.Data["vu"])
			assert.Equal(t, int64(7), e.Data["iter"])
			assert.Equal(t, tc.expectStatus, e.Data["status"])
			if tc.expectError {
				assert.NotEmpty(t, e.Data["error"])
				return
			}
			assert.Equal(t, tc.expectBody, e.Data["body"])
			headers, ok := e.Data["headers"].(map[string]string)
			require.True(t, ok)
			assert.Equal(t, "value", headers["X-Test"])
		})
	}
}

func TestLogFailedResponseBody(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", failedResponseBody(nil))
	assert.Equal(t, "text", failedResponseBody("text"))
	assert.Equal(t, "bytes", failedResponseBody([]byte("bytes")))
}
