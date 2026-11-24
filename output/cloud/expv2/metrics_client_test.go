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
	url, err := deriveMetricsURL(c.BaseURL(), "test-ref-id")
	require.NoError(t, err)
	mc, err := newMetricsClientWithURL(c, url)
	require.NoError(t, err)

	mset := pbcloud.MetricSet{}
	err = mc.push(t.Context(), &mset)
	require.NoError(t, err)
	assert.Equal(t, 1, reqs)
}

func TestMetricsClientPushUnexpectedStatus(t *testing.T) {
	t.Parallel()

	// Use a mock that returns immediately without HTTP - no server, no retries.
	mock := &mockMetricsHTTPClient{
		doErr: errors.New("500 Internal Server Error"),
	}
	mc, err := newMetricsClientWithURL(mock, "http://test/v2/metrics/test-ref-id")
	require.NoError(t, err)

	err = mc.push(t.Context(), nil)
	assert.ErrorContains(t, err, "500 Internal Server Error")
}

// mockMetricsHTTPClient implements the metricsHTTPClient interface for tests.
// It returns immediately without making HTTP requests.
type mockMetricsHTTPClient struct {
	doErr error
}

func (m *mockMetricsHTTPClient) Do(_ *http.Request, _ any) error {
	return m.doErr
}

func TestNewMetricsClientWithURL(t *testing.T) {
	t.Parallel()

	stub := &mockMetricsHTTPClient{}
	mc, err := newMetricsClientWithURL(stub, "https://ingest.example/metrics/abc")
	require.NoError(t, err)
	assert.Equal(t, "https://ingest.example/metrics/abc", mc.url)
	assert.Equal(t, stub, mc.httpClient)
}

func TestNewMetricsClientWithURL_EmptyURLReturnsError(t *testing.T) {
	t.Parallel()

	stub := &mockMetricsHTTPClient{}
	_, err := newMetricsClientWithURL(stub, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metrics push URL is required")
}

func TestDeriveMetricsURL(t *testing.T) {
	t.Parallel()

	url, err := deriveMetricsURL("https://ingest.example/v1", "12345")
	require.NoError(t, err)
	assert.Equal(t, "https://ingest.example/v2/metrics/12345", url)
}

func TestDeriveMetricsURL_MissingV1Suffix(t *testing.T) {
	t.Parallel()

	_, err := deriveMetricsURL("https://ingest.example", "12345")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/v1 suffix")
}

func TestDeriveMetricsURL_EmptyTestRunID(t *testing.T) {
	t.Parallel()

	_, err := deriveMetricsURL("https://ingest.example/v1", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TestRunID")
}
