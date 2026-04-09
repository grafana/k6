package cloudapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
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

		client, err := NewClient(testutils.NewLogger(t), "invalid-token", server.URL, "1.0", 1*time.Second)
		require.NoError(t, err)

		resp, err := client.ValidateToken(t.Context(), "https://stack.grafana.net")
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "(401/error) Invalid token")
	})

	t.Run("network error should fail", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient(
			testutils.NewLogger(t), "test-token", "http://invalid-url-that-does-not-exist", "1.0", 1*time.Second,
		)
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

	t.Run("canceled context should fail", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(testutils.NewLogger(t), "test-token", "http://example.com", "1.0", 1*time.Second)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		resp, err := client.ValidateToken(ctx, "https://stack.grafana.net")
		assert.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, resp)
	})
}

func fprint(t testing.TB, w io.Writer, format string) int {
	t.Helper()

	n, err := fmt.Fprint(w, format)
	require.NoError(t, err)
	return n
}

func TestFetchTest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/cloud/v6/test_runs/123", r.URL.Path)
		assert.Equal(t, "123", r.Header.Get("X-Stack-Id"))

		statusDetails := *k6cloud.NewStatusApiModel(StatusCompleted, time.Unix(0, 0).UTC())
		result := ResultPassed
		estimatedDuration := int32(10)
		resp := k6cloud.NewTestRunApiModel(
			123,
			456,
			124,
			*k6cloud.NewNullableString(nil),
			time.Unix(0, 0).UTC(),
			*k6cloud.NewNullableTime(nil),
			"",
			*k6cloud.NewNullableTime(nil),
			*k6cloud.NewNullableTestRunApiModelCost(nil),
			StatusCompleted,
			statusDetails,
			[]k6cloud.StatusApiModel{statusDetails},
			[]k6cloud.DistributionZoneApiModel{},
			*k6cloud.NewNullableString(&result),
			map[string]any{},
			map[string]any{},
			map[string]string{},
			map[string]string{},
			*k6cloud.NewNullableInt32(nil),
			*k6cloud.NewNullableInt32(nil),
			*k6cloud.NewNullableInt32(&estimatedDuration),
			10,
		)
		data, err := json.Marshal(resp)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(data)
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", time.Second)
	require.NoError(t, err)
	client.SetStackID(123)

	progress, err := client.FetchTest(t.Context(), 123)
	require.NoError(t, err)
	require.NotNil(t, progress)
	assert.Equal(t, StatusCompleted, progress.Status)
	assert.Equal(t, ResultPassed, progress.Result)
	assert.True(t, progress.IsFinished())
}

func TestStopTest(t *testing.T) {
	t.Parallel()

	var stopped bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/cloud/v6/test_runs/123/abort", r.URL.Path)
		assert.Equal(t, "123", r.Header.Get("X-Stack-Id"))
		stopped = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", time.Second)
	require.NoError(t, err)
	client.SetStackID(123)

	require.NoError(t, client.StopTest(t.Context(), 123))
	assert.True(t, stopped)
}
