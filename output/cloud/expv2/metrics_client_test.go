package expv2

import (
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
	mc, err := newMetricsClient(c, "", "test-ref-id", "")
	require.NoError(t, err)

	mset := pbcloud.MetricSet{}
	err = mc.push(&mset)
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

	c := cloudapi.NewClient(nil, "fake-token", ts.URL, "k6cloud/v0.4", 1*time.Second)
	mc, err := newMetricsClient(c, "", "test-ref-id", "")
	require.NoError(t, err)

	err = mc.push(nil)
	assert.ErrorContains(t, err, "500 Internal Server Error")
}

func TestMetricsClient_PushURLAndAuth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		pushURLSuffix string // "" = no explicit push URL
		testRunToken  string // "" ⇒ expect Token scheme with client token
		wantURLPath   string
		wantAuth      string
	}{
		{
			name:          "explicit push URL is used",
			pushURLSuffix: "/custom/metrics/push",
			testRunToken:  "",
			wantURLPath:   "/custom/metrics/push",
			wantAuth:      "Token test-token",
		},
		{
			name:          "test-run token sets Bearer auth",
			pushURLSuffix: "/any",
			testRunToken:  "scoped-token-xyz",
			wantURLPath:   "/any",
			wantAuth:      "Bearer scoped-token-xyz",
		},
		{
			name:          "no test-run token falls back to Token auth",
			pushURLSuffix: "/any",
			testRunToken:  "",
			wantURLPath:   "/any",
			wantAuth:      "Token test-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedPath, capturedAuth string
			ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.Path
				capturedAuth = r.Header.Get("Authorization")
				assert.Equal(t, "2.0", r.Header.Get("K6-Metrics-Protocol-Version"))
				assert.Equal(t, "application/x-protobuf", r.Header.Get("Content-Type"))
				assert.Equal(t, "snappy", r.Header.Get("Content-Encoding"))
			}))
			defer ts.Close()

			pushURL := ""
			if tt.pushURLSuffix != "" {
				pushURL = ts.URL + tt.pushURLSuffix
			}

			c := cloudapi.NewClient(nil, "test-token", ts.URL, "k6cloud/v0.4", 1*time.Second)
			mc, err := newMetricsClient(c, pushURL, "run1", tt.testRunToken)
			require.NoError(t, err)

			mset := pbcloud.MetricSet{}
			err = mc.push(&mset)
			require.NoError(t, err)

			assert.Equal(t, tt.wantURLPath, capturedPath)
			assert.Equal(t, tt.wantAuth, capturedAuth)
		})
	}
}
