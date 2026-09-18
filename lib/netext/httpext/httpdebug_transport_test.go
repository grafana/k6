package httpext

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceRequestDumpProto(t *testing.T) {
	t.Parallel()

	dump := []byte("GET /path HTTP/1.1\r\nHost: example.com\r\n\r\n")

	got := replaceRequestDumpProto(dump, "HTTP/2.0")
	assert.Equal(t, []byte("GET /path HTTP/2.0\r\nHost: example.com\r\n\r\n"), got)

	assert.Equal(t, dump, replaceRequestDumpProto(dump, ""))
	assert.Equal(t, dump, replaceRequestDumpProto(dump, "HTTP/1.1"))
	assert.Equal(t, []byte("no-eol"), replaceRequestDumpProto([]byte("no-eol"), "HTTP/2.0"))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestHTTPDebugTransportLogsNegotiatedProto(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true, DisableColors: true})

	transport := httpDebugTransport{
		originalTransport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
				Proto:      "HTTP/2.0",
				ProtoMajor: 2,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    req,
			}, nil
		}),
		logger: logger,
	}

	req, err := http.NewRequest(http.MethodGet, "http://example.com/path", nil)
	require.NoError(t, err)

	_, err = transport.RoundTrip(req)
	require.NoError(t, err)

	logged := buf.String()
	assert.Contains(t, logged, "GET /path HTTP/2.0")
	assert.NotContains(t, logged, "GET /path HTTP/1.1")
}
