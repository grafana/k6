package cloudapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/lib/testutils"
)

func TestValidateToken(t *testing.T) {
	t.Parallel()

	t.Run("successful token validation", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the authorization header
			authHeader := r.Header.Get("Authorization")
			assert.Equal(t, "Bearer test-token", authHeader)

			// Verify the stack URL
			stackURL := r.Header.Get("X-Stack-Url")
			assert.Equal(t, stackURL, "https://stack.grafana.net")

			w.Header().Add("Content-Type", "application/json")
			fprint(t, w, `{
				"stack_id": 123,
				"default_project_id": 456
			}`)
		}))
		defer server.Close()

		client, err := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", 1*time.Second)
		require.NoError(t, err)

		resp, err := client.ValidateToken(t.Context(), "https://stack.grafana.net")
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, int32(123), resp.StackId)
		assert.Equal(t, int32(456), resp.DefaultProjectId)
	})

	t.Run("unauthorized token should fail", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeError(t, w, http.StatusUnauthorized, "error", "Invalid token")
		}))
		defer server.Close()

		client, err := NewClient(testutils.NewLogger(t), "invalid-token", server.URL, "1.0", 1*time.Second)
		require.NoError(t, err)

		resp, err := client.ValidateToken(t.Context(), "https://stack.grafana.net")
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "(401/error) Invalid token")
	})

	t.Run("network error should fail", func(t *testing.T) {
		t.Parallel()
		// Use an invalid URL to simulate network error
		client, err := NewClient(testutils.NewLogger(t), "test-token", "http://invalid-url-that-does-not-exist", "1.0", 1*time.Second)
		require.NoError(t, err)

		resp, err := client.ValidateToken(t.Context(), "https://stack.grafana.net")
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("missing stack URL should fail", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient(testutils.NewLogger(t), "test-token", "http://example.com", "1.0", 1*time.Second)
		require.NoError(t, err)

		resp, err := client.ValidateToken(t.Context(), "")
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, "stack URL is required to validate token", err.Error())
	})

	t.Run("invalid stack URL should fail", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient(testutils.NewLogger(t), "test-token", "http://example.com", "1.0", 1*time.Second)
		require.NoError(t, err)

		resp, err := client.ValidateToken(t.Context(), "://invalid-url")
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "invalid stack URL")
	})
}

func TestClientRetry(t *testing.T) {
	t.Parallel()

	client, err := NewClient(testutils.NewLogger(t), "test-token", "http://example.com", "1.0", 1*time.Second)
	require.NoError(t, err)
	// the client already tests retries. we just verify we set it up correctly.
	assert.Equal(t, MaxRetries, client.apiClient.GetConfig().MaxRetries)
	assert.Equal(t, RetryInterval, client.apiClient.GetConfig().RetryInterval)
}

func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "1.0", 5*time.Second)
	require.NoError(t, err)
	client.SetStackID(1)

	return client
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, v any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	assert.NoError(t, json.NewEncoder(w).Encode(v))
}

func writeError(t *testing.T, w http.ResponseWriter, status int, code, msg string) {
	t.Helper()

	writeJSON(t, w, status, map[string]any{
		"error": map[string]string{"code": code, "message": msg},
	})
}

func fprint(t *testing.T, w io.Writer, format string) int {
	n, err := fmt.Fprint(w, format)
	assert.NoError(t, err)
	return n
}
