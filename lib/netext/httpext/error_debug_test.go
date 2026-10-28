package httpext

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

func TestMakeRequestHTTPErrorDebug(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			_, _ = w.Write([]byte("fine"))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	t.Cleanup(srv.Close)
	errorMode := null.StringFrom("error")

	makeRequestWithResponse := func(t *testing.T, httpDebug null.String, url string) ([]*logrus.Entry, *Response) {
		t.Helper()
		logger, hook := logtest.NewNullLogger()
		registry := metrics.NewRegistry()
		state := &lib.State{
			Options: lib.Options{
				SystemTags: &metrics.DefaultSystemTagSet,
				HTTPDebug:  httpDebug,
			},
			Transport:      srv.Client().Transport,
			Samples:        make(chan metrics.SampleContainer, 10),
			Logger:         logger,
			BufferPool:     lib.NewBufferPool(),
			BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
			Tags:           lib.NewVUStateTags(registry.RootTagSet()),
		}

		body := bytes.NewBufferString(`{"user":"bob"}`)
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, nil)
		require.NoError(t, err)
		req.ContentLength = int64(body.Len())
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body.Bytes())), nil
		}
		req.Body, _ = req.GetBody()

		preq := &ParsedHTTPRequest{
			Req:              req,
			URL:              &URL{u: req.URL, URL: req.URL.String()},
			Body:             body,
			Timeout:          10 * time.Second,
			ResponseType:     ResponseTypeText,
			ResponseCallback: func(status int) bool { return status >= 200 && status < 400 },
			TagsAndMeta:      state.Tags.GetCurrentValues(),
		}
		preq.TagsAndMeta.SetTag("name", "Login")
		preq.TagsAndMeta.SetTag("team", "auth")
		resp, err := MakeRequest(t.Context(), state, preq)
		require.NoError(t, err)
		return hook.AllEntries(), resp
	}
	makeRequest := func(t *testing.T, httpDebug null.String, url string) []*logrus.Entry {
		t.Helper()
		entries, _ := makeRequestWithResponse(t, httpDebug, url)
		return entries
	}

	t.Run("request tags for checks", func(t *testing.T) {
		t.Parallel()
		_, resp := makeRequestWithResponse(t, errorMode, srv.URL+"/ok")
		tags, dump, ok := DebugDump(resp)
		require.True(t, ok)
		assert.Equal(t, "Login", tags["name"])
		assert.Equal(t, "auth", tags["team"])
		assert.Len(t, tags["request_id"], 36, "a UUID")
		assert.Contains(t, dump, "POST http")

		_, resp = makeRequestWithResponse(t, null.StringFrom("full"), srv.URL+"/ok")
		tags, _, _ = DebugDump(resp)
		assert.Nil(t, tags, "tags are only kept with --http-debug=error")
	})

	t.Run("other http-debug modes are unchanged", func(t *testing.T) {
		t.Parallel()
		for _, mode := range []string{"headers", "full"} {
			entries := makeRequest(t, null.StringFrom(mode), srv.URL+"/fail")
			require.Len(t, entries, 2, mode) // the request and the response, no "Request Failed"
			assert.True(t, strings.HasPrefix(entries[0].Message, "Request:\n"), mode)
			assert.True(t, strings.HasPrefix(entries[1].Message, "Response:\n"), mode)
			assert.Equal(t, entries[0].Data["request_id"], entries[1].Data["request_id"], mode)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, makeRequest(t, null.String{}, srv.URL+"/fail"))
	})

	t.Run("successful request", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, makeRequest(t, errorMode, srv.URL+"/ok"))
	})

	t.Run("failed request", func(t *testing.T) {
		t.Parallel()
		entries := makeRequest(t, errorMode, srv.URL+"/fail")
		require.Len(t, entries, 1)
		msg := entries[0].Message
		assert.True(t, strings.HasPrefix(msg, "Request Failed"))
		assert.Regexp(t, ` request_id=[0-9a-f-]{36} `, msg)
		// The metadata goes before the details, with no blank lines between the sections.
		assert.Less(t, strings.Index(msg, " request_id="), strings.Index(msg, "\nRequest:\n"))
		assert.Contains(t, msg, "\nResponse:\n")
		assert.NotContains(t, msg, "\n\n\n")
		assert.NotContains(t, msg, "\n\nResponse:")
		assert.Contains(t, entries[0].Message, "POST /fail HTTP/1.1")
		assert.Contains(t, entries[0].Message, "500 Internal Server Error")
		assert.Contains(t, entries[0].Message, `{"user":"bob"}`)
		assert.Contains(t, entries[0].Message, "boom")
	})

	t.Run("network error", func(t *testing.T) {
		t.Parallel()
		// A port that nothing listens on, so the connection is refused.
		ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr := ln.Addr().String()
		require.NoError(t, ln.Close())

		entries, resp := makeRequestWithResponse(t, errorMode, "http://"+addr+"/fail")
		require.Len(t, entries, 1)
		msg := entries[0].Message
		assert.True(t, strings.HasPrefix(msg, "Request Failed"))
		assert.Contains(t, msg, fmt.Sprintf(" error_code=%d ", tcpDialRefusedErrorCode))
		assert.Contains(t, msg, "connection refused")
		assert.Contains(t, msg, "POST /fail HTTP/1.1")
		assert.NotContains(t, msg, "\nResponse:\n", "there is no response")

		fields, dump, ok := DebugDump(resp)
		require.True(t, ok)
		assert.Equal(t, strconv.Itoa(int(tcpDialRefusedErrorCode)), fields["error_code"])
		assert.Contains(t, dump, "\nResponse:\nerror: dial: connection refused")
	})
}

func TestAddMissing(t *testing.T) {
	t.Parallel()
	dst := map[string]any{"vu": 1}
	AddMissing(dst, map[string]string{"vu": "999", "name": "Login"}, map[string]string{"name": "Other", "team": "auth"})
	assert.Equal(t, map[string]any{"vu": 1, "name": "Login", "team": "auth"}, dst)
}

func TestIsFailedStatus(t *testing.T) {
	t.Parallel()
	only200 := func(s int) bool { return s == 200 }
	tests := []struct {
		name     string
		callback func(int) bool
		status   int
		want     bool
	}{
		{name: "no callback", callback: nil, status: http.StatusInternalServerError, want: false},
		{name: "unexpected status", callback: only200, status: http.StatusInternalServerError, want: true},
		{name: "expected status", callback: only200, status: http.StatusOK, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isFailedStatus(tt.callback, tt.status))
		})
	}
}

func TestDebugMessage(t *testing.T) {
	t.Parallel()
	meta := map[string]any{"vu": 1, "name": "Log in", "group": "", "status": 500}
	msg := DebugMessage("Title", meta, "details")
	assert.Equal(t, "Title\nContext: name=\"Log in\" status=500 vu=1\n\ndetails\n\n", msg)
}
