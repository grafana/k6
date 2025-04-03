package remote

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPushgatewayClient(t *testing.T) {
	t.Parallel()

	t.Run("CustomConfig", func(t *testing.T) {
		t.Parallel()
		hc := &HTTPConfig{Timeout: time.Second}
		wc, err := NewPushgatewayClient("http://example.com/api/v1/write", "job", hc)
		require.NoError(t, err)
		require.NotNil(t, wc)
		assert.Equal(t, wc.cfg, hc)
	})

	t.Run("InvalidURL", func(t *testing.T) {
		t.Parallel()
		wc, err := NewPushgatewayClient("fake://bad url", "job", nil)
		require.Error(t, err)
		assert.Nil(t, wc)
	})
}

func TestPushgatewayPush(t *testing.T) {
	t.Parallel()
	h := func(rw http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Header.Get("Content-Type"), "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=delimited")
		assert.Equal(t, r.Header.Get("User-Agent"), "k6-prometheus-rw-output")
		assert.Equal(t, r.Header.Get("X-MY-CUSTOM-HEADER"), "fake")
		assert.Equal(t, r.URL.Path, "/metrics/job/myjob")
		username, password, ok := r.BasicAuth()
		assert.Equal(t, username, "foo")
		assert.Equal(t, password, "bar")
		assert.True(t, ok)
		assert.NotEmpty(t, r.Header.Get("Content-Length"))

		b, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.NotEmpty(t, len(b))

		rw.WriteHeader(http.StatusOK)
	}
	ts := httptest.NewServer(http.HandlerFunc(h))
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)

	c := &PushgatewayClient{
		hc:  ts.Client(),
		url: u,
		job: "myjob",
		cfg: &HTTPConfig{
			BasicAuth: &BasicAuth{
				Username: "foo",
				Password: "bar",
			},
			Headers: map[string][]string{
				"X-MY-CUSTOM-HEADER": {"fake"},
			},
		},
	}

	reg1 := prometheus.NewRegistry()
	err = reg1.Register(prometheus.NewGauge(prometheus.GaugeOpts{Name: "test", Help: "test"}))
	assert.NoError(t, err)

	err = c.Push(context.Background(), []*prometheus.Registry{reg1})
	assert.NoError(t, err)
}
