package httpext

import (
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

func TestHTTPDebugDoesNotInventGzipAcceptEncoding(t *testing.T) {
	t.Parallel()

	logger, hook := logtest.NewNullLogger()
	logger.SetLevel(logrus.InfoLevel)

	tr := httpDebugTransport{
		originalTransport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
				Request:    req,
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

	var dump string
	for _, e := range hook.AllEntries() {
		if strings.Contains(e.Message, "Request:") {
			dump = e.Message
			break
		}
	}
	require.NotEmpty(t, dump)
	assert.NotContains(t, dump, "gzip")
	assert.NotContains(t, dump, "Accept-Encoding")
}
