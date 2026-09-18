package httpext

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func gzipBytes(t *testing.T, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(body)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func responseDump(t *testing.T, hook *logtest.Hook) string {
	t.Helper()
	for _, e := range hook.AllEntries() {
		if strings.Contains(e.Message, "Response:") {
			return e.Message
		}
	}
	t.Fatal("no response dump logged")
	return ""
}

func TestHTTPDebugDecodesGzipResponseBody(t *testing.T) {
	t.Parallel()

	plain := []byte(`{"brotli":true,"gzip":true}`)
	compressed := gzipBytes(t, plain)

	logger, hook := logtest.NewNullLogger()
	logger.SetLevel(logrus.InfoLevel)

	tr := httpDebugTransport{
		originalTransport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				Status:        "200 OK",
				StatusCode:    http.StatusOK,
				Proto:         "HTTP/1.1",
				ProtoMajor:    1,
				ProtoMinor:    1,
				Header:        http.Header{"Content-Encoding": []string{"gzip"}},
				Body:          io.NopCloser(bytes.NewReader(compressed)),
				ContentLength: int64(len(compressed)),
				Request:       req,
			}, nil
		}),
		httpDebugOption: "full",
		logger:          logger,
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com/", nil)
	require.NoError(t, err)

	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	assert.Equal(t, compressed, got)

	dump := responseDump(t, hook)
	assert.Contains(t, dump, string(plain))
	assert.NotContains(t, dump, "\x1f\x8b")
}

func TestHTTPDebugHeadersSkipGzipResponseBody(t *testing.T) {
	t.Parallel()

	plain := []byte(`{"hello":"world"}`)
	compressed := gzipBytes(t, plain)

	logger, hook := logtest.NewNullLogger()
	logger.SetLevel(logrus.InfoLevel)

	tr := httpDebugTransport{
		originalTransport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				Status:        "200 OK",
				StatusCode:    http.StatusOK,
				Proto:         "HTTP/1.1",
				ProtoMajor:    1,
				ProtoMinor:    1,
				Header:        http.Header{"Content-Encoding": []string{"gzip"}},
				Body:          io.NopCloser(bytes.NewReader(compressed)),
				ContentLength: int64(len(compressed)),
				Request:       req,
			}, nil
		}),
		httpDebugOption: "headers",
		logger:          logger,
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com/", nil)
	require.NoError(t, err)

	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	dump := responseDump(t, hook)
	assert.Contains(t, dump, "Content-Encoding: gzip")
	assert.NotContains(t, dump, string(plain))
}
