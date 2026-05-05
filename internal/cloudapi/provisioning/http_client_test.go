package provisioning

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/lib/testutils"
)

func TestHTTPClient(t *testing.T) {
	t.Parallel()

	var (
		gotAuth string
		gotUA   string
		hits    atomic.Int32
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"foo": "bar"})
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "scoped-token", "v1.2.3", testutils.NewLogger(t))

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, nil)
	require.NoError(t, err)

	var target struct {
		Foo string `json:"foo"`
	}
	err = c.Do(req, &target)
	require.NoError(t, err)

	assert.Equal(t, "bar", target.Foo)
	assert.Equal(t, "Bearer scoped-token", gotAuth)
	assert.Equal(t, "k6cloud/v1.2.3", gotUA)
	assert.Equal(t, int32(1), hits.Load())
}

func TestHTTPClient_NilVDoesNotDecode(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "tok", "v0.1.0", testutils.NewLogger(t))

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, nil)
	require.NoError(t, err)

	err = c.Do(req, nil)
	assert.NoError(t, err)
}

func TestHTTPClient_RetriesOn5xx(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "tok", "v0.1.0", testutils.NewLogger(t))

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, nil)
	require.NoError(t, err)

	err = c.Do(req, nil)
	assert.NoError(t, err)
	assert.Equal(t, int32(2), hits.Load())
}

func TestHTTPClient_NoRetryOn4xx(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad input"}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "tok", "v0.1.0", testutils.NewLogger(t))

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, nil)
	require.NoError(t, err)

	err = c.Do(req, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Contains(t, err.Error(), "bad input")
	assert.Equal(t, int32(1), hits.Load())
}

func TestHTTPClient_RequestBodyReplayedOnRetry(t *testing.T) {
	t.Parallel()

	var (
		hits  atomic.Int32
		body1 []byte
		body2 []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if hits.Add(1) == 1 {
			body1 = b
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		body2 = b
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	payload := []byte(`{"metrics":[1,2,3]}`)
	c := NewHTTPClient(srv.Client(), "tok", "v0.1.0", testutils.NewLogger(t))

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, bytes.NewReader(payload))
	require.NoError(t, err)

	err = c.Do(req, nil)
	require.NoError(t, err)
	assert.Equal(t, int32(2), hits.Load())
	assert.Equal(t, body1, body2, "body replay should produce identical bytes on retry")
}
