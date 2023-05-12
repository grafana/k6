package expv2

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/output/cloud/expv2/pbcloud"
)

type httpDoerFunc func(*http.Request) (*http.Response, error)

func (fn httpDoerFunc) Do(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func TestMetricsClientPush(t *testing.T) {
	t.Parallel()

	done := make(chan struct{}, 1)
	reqs := 0
	h := func(rw http.ResponseWriter, r *http.Request) {
		defer close(done)
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

	mc, err := NewMetricsClient(testutils.NewLogger(t), ts.URL, "fake-token")
	require.NoError(t, err)
	mc.httpClient = ts.Client()

	mset := pbcloud.MetricSet{}
	err = mc.Push(context.TODO(), "test-ref-id", &mset)
	<-done
	require.NoError(t, err)
	assert.Equal(t, 1, reqs)
}

func TestMetricsClientPushUnexpectedStatus(t *testing.T) {
	t.Parallel()

	h := func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusInternalServerError)
	}
	ts := httptest.NewServer(http.HandlerFunc(h))
	defer ts.Close()

	mc, err := NewMetricsClient(nil, ts.URL, "fake-token")
	require.NoError(t, err)
	mc.httpClient = ts.Client()

	err = mc.Push(context.TODO(), "test-ref-id", nil)
	assert.ErrorContains(t, err, "500 Internal Server Error")
}

func TestMetricsClientPushError(t *testing.T) {
	t.Parallel()

	httpClientMock := func(_ *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("fake generated error")
	}

	mc := MetricsClient{
		httpClient: httpDoerFunc(httpClientMock),
		pushBufferPool: sync.Pool{
			New: func() interface{} {
				return &bytes.Buffer{}
			},
		},
	}

	err := mc.Push(context.TODO(), "test-ref-id", nil)
	assert.ErrorContains(t, err, "fake generated error")
}
