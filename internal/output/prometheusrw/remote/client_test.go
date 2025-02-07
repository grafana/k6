package remote

import (
	"context"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	prompb "buf.build/gen/go/prometheus/prometheus/protocolbuffers/go"
	"github.com/klauspost/compress/snappy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/output/prometheusrw/stale"
	"google.golang.org/protobuf/proto"
)

func TestNewWriteClient(t *testing.T) {
	t.Parallel()
	t.Run("DefaultConfig", func(t *testing.T) {
		t.Parallel()
		wc, err := NewWriteClient("http://example.com/api/v1/write", nil)
		require.NoError(t, err)
		require.NotNil(t, wc)
		assert.Equal(t, wc.cfg, &HTTPConfig{})
	})

	t.Run("CustomConfig", func(t *testing.T) {
		t.Parallel()
		hc := &HTTPConfig{Timeout: time.Second}
		wc, err := NewWriteClient("http://example.com/api/v1/write", hc)
		require.NoError(t, err)
		require.NotNil(t, wc)
		assert.Equal(t, wc.cfg, hc)
	})

	t.Run("InvalidURL", func(t *testing.T) {
		t.Parallel()
		wc, err := NewWriteClient("fake://bad url", nil)
		require.Error(t, err)
		assert.Nil(t, wc)
	})
}

func TestClientStore(t *testing.T) {
	t.Parallel()
	h := func(rw http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Header.Get("Content-Encoding"), "snappy")
		assert.Equal(t, r.Header.Get("Content-Type"), "application/x-protobuf")
		assert.Equal(t, r.Header.Get("User-Agent"), "k6-prometheus-rw-output")
		assert.Equal(t, r.Header.Get("X-Prometheus-Remote-Write-Version"), "0.1.0")
		assert.NotEmpty(t, r.Header.Get("Content-Length"))

		b, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.NotEmpty(t, len(b))

		rw.WriteHeader(http.StatusNoContent)
	}
	ts := httptest.NewServer(http.HandlerFunc(h))
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)

	c := &WriteClient{
		hc:  ts.Client(),
		url: u,
		cfg: &HTTPConfig{},
	}
	data := &prompb.TimeSeries{
		Labels: []*prompb.Label{
			{
				Name:  "label1",
				Value: "label1-val",
			},
		},
		Samples: []*prompb.Sample{
			{
				Value:     8.5,
				Timestamp: time.Now().UnixMilli(),
			},
		},
	}
	err = c.Store(context.Background(), []*prompb.TimeSeries{data})
	assert.NoError(t, err)
}

func TestClientStoreHTTPError(t *testing.T) {
	t.Parallel()
	h := func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad", http.StatusUnauthorized)
	}
	ts := httptest.NewServer(http.HandlerFunc(h))
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)

	c := &WriteClient{
		hc:  ts.Client(),
		url: u,
		cfg: &HTTPConfig{},
	}
	assert.Error(t, c.Store(context.Background(), nil))
}

func TestClientStoreHTTPBasic(t *testing.T) {
	t.Parallel()
	h := func(_ http.ResponseWriter, r *http.Request) {
		u, pwd, ok := r.BasicAuth()
		require.True(t, ok)
		assert.Equal(t, "usertest", u)
		assert.Equal(t, "pwdtest", pwd)
	}
	ts := httptest.NewServer(http.HandlerFunc(h))
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)

	c := &WriteClient{
		hc:  ts.Client(),
		url: u,
		cfg: &HTTPConfig{
			BasicAuth: &BasicAuth{
				Username: "usertest",
				Password: "pwdtest",
			},
		},
	}
	assert.NoError(t, c.Store(context.Background(), nil))
}

func TestClientStoreHeaders(t *testing.T) {
	t.Parallel()
	h := func(_ http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Header.Get("X-Prometheus-Remote-Write-Version"), "0.1.0")
		assert.Equal(t, r.Header.Get("X-MY-CUSTOM-HEADER"), "fake")
	}
	ts := httptest.NewServer(http.HandlerFunc(h))
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)

	c := &WriteClient{
		hc:  ts.Client(),
		url: u,
		cfg: &HTTPConfig{
			Headers: http.Header(map[string][]string{
				"X-MY-CUSTOM-HEADER": {"fake"},
				// If the same key, of a mandatory protocol's header
				// is provided, it will be overwritten.
				"X-Prometheus-Remote-Write-Version": {"fake"},
			}),
		},
	}
	assert.NoError(t, c.Store(context.Background(), nil))
}

func TestNewWriteRequestBody(t *testing.T) {
	t.Parallel()
	ts := []*prompb.TimeSeries{
		{
			Labels:  []*prompb.Label{{Name: "label1", Value: "val1"}},
			Samples: []*prompb.Sample{{Value: 10.1, Timestamp: time.Unix(1, 0).Unix()}},
		},
	}
	b, err := newWriteRequestBody(ts)
	require.NoError(t, err)
	require.NotEmpty(t, string(b))
	assert.Contains(t, string(b), `label1`)
}

func TestNewWriteRequestBodyWithStaleMarker(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2022, time.December, 15, 11, 41, 18, 123, time.UTC)

	ts := []*prompb.TimeSeries{
		{
			Labels: []*prompb.Label{{Name: "label1", Value: "val1"}},
			Samples: []*prompb.Sample{{
				Value:     stale.Marker,
				Timestamp: timestamp.UnixMilli(),
			}},
		},
	}
	b, err := newWriteRequestBody(ts)
	require.NoError(t, err)
	require.NotEmpty(t, b)

	sb, err := snappy.Decode(nil, b)
	require.NoError(t, err)

	var series prompb.WriteRequest
	err = proto.Unmarshal(sb, &series)
	require.NoError(t, err)
	require.NotEmpty(t, series.Timeseries[0])
	require.NotEmpty(t, series.Timeseries[0].Samples)

	assert.True(t, math.IsNaN(series.Timeseries[0].Samples[0].Value))
	assert.Equal(t, timestamp.UnixMilli(), series.Timeseries[0].Samples[0].Timestamp)
}

func TestValidateStatusCode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status int
		expErr bool
	}{
		{status: http.StatusOK, expErr: false},        // Mimir
		{status: http.StatusNoContent, expErr: false}, // Prometheus
		{status: http.StatusBadRequest, expErr: true},
	}
	for _, tt := range tests {
		err := validateResponseStatus(tt.status)
		if tt.expErr {
			assert.Error(t, err)
			continue
		}
		assert.NoError(t, err)
	}
}
