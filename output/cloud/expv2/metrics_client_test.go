package expv2

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/internal/output/cloud/expv2/pbcloud"
)

func TestMetricsClientPush(t *testing.T) {
	t.Parallel()

	reqs := 0
	h := func(_ http.ResponseWriter, r *http.Request) {
		reqs++

		assert.Equal(t, "/v2/metrics/test-ref-id", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "Token fake-token", r.Header.Get("Authorization"))
		assert.Contains(t, r.Header.Get("User-Agent"), "k6cloud/v0.4")
		assert.Equal(t, "application/x-protobuf", r.Header.Get("Content-Type"))
		assert.Equal(t, "snappy", r.Header.Get("Content-Encoding"))
		assert.Equal(t, "2.0", r.Header.Get("K6-Metrics-Protocol-Version"))
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.NotEmpty(t, b)
	}

	ts := httptest.NewServer(http.HandlerFunc(h))
	defer ts.Close()

	c := cloudapi.NewClient(nil, "fake-token", ts.URL, "k6cloud/v0.4", 1*time.Second)
	mc, err := newMetricsClient(c, "test-ref-id")
	require.NoError(t, err)

	mset := pbcloud.MetricSet{}
	err = mc.push(&mset)
	require.NoError(t, err)
	assert.Equal(t, 1, reqs)
}

func TestMetricsClientPushUnexpectedStatus(t *testing.T) {
	t.Parallel()

	// Use a mock that returns immediately without HTTP - no server, no retries.
	mock := &mockMetricsHTTPClient{
		doErr: errors.New("500 Internal Server Error"),
	}
	mc, err := newMetricsClient(mock, "test-ref-id")
	require.NoError(t, err)

	err = mc.push(nil)
	assert.ErrorContains(t, err, "500 Internal Server Error")
}

// mockMetricsHTTPClient implements metricsHTTPClient for tests.
// It returns immediately without making HTTP requests.
type mockMetricsHTTPClient struct {
	doErr error
}

func (m *mockMetricsHTTPClient) Do(_ *http.Request, _ any) error {
	return m.doErr
}

func (m *mockMetricsHTTPClient) BaseURL() string {
	return "http://test/v1"
}
