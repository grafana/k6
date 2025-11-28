package cloudapi

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/lib/testutils"
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

		client := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", 1*time.Second)

		resp, err := client.ValidateToken("https://stack.grafana.net")

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, int32(123), resp.StackId)
		assert.Equal(t, int32(456), resp.DefaultProjectId)
	})

	t.Run("unauthorized token should fail", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fprint(t, w, `{
				"error": {
					"code": "error",
					"message": "Invalid token"
				}
			}`)
		}))
		defer server.Close()

		client := NewClient(testutils.NewLogger(t), "invalid-token", server.URL, "1.0", 1*time.Second)

		resp, err := client.ValidateToken("https://stack.grafana.net")

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "(401/error) Invalid token")
	})

	t.Run("network error should fail", func(t *testing.T) {
		t.Parallel()
		// Use an invalid URL to simulate network error
		client := NewClient(testutils.NewLogger(t), "test-token", "http://invalid-url-that-does-not-exist", "1.0", 1*time.Second)

		resp, err := client.ValidateToken("https://stack.grafana.net")

		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func fprint(t *testing.T, w io.Writer, format string) int {
	n, err := fmt.Fprint(w, format)
	require.NoError(t, err)
	return n
}
