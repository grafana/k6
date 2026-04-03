package cloudapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/lib/testutils"
)

func TestSetStackID(t *testing.T) {
	t.Parallel()

	t.Run("accepts zero", func(t *testing.T) {
		t.Parallel()
		c, err := NewClient(testutils.NewLogger(t), "token", "http://example.com", "1.0", time.Second)
		require.NoError(t, err)
		require.NoError(t, c.SetStackID(0))
	})

	t.Run("accepts valid", func(t *testing.T) {
		t.Parallel()
		c, err := NewClient(testutils.NewLogger(t), "token", "http://example.com", "1.0", time.Second)
		require.NoError(t, err)
		require.NoError(t, c.SetStackID(123))
	})

	t.Run("rejects overflow", func(t *testing.T) {
		t.Parallel()
		c, err := NewClient(testutils.NewLogger(t), "token", "http://example.com", "1.0", time.Second)
		require.NoError(t, err)
		err = c.SetStackID(1 << 33)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid stack ID")
	})
}

func TestSetProjectID(t *testing.T) {
	t.Parallel()

	t.Run("accepts zero", func(t *testing.T) {
		t.Parallel()
		c, err := NewClient(testutils.NewLogger(t), "token", "http://example.com", "1.0", time.Second)
		require.NoError(t, err)
		require.NoError(t, c.SetProjectID(0))
	})

	t.Run("accepts valid", func(t *testing.T) {
		t.Parallel()
		c, err := NewClient(testutils.NewLogger(t), "token", "http://example.com", "1.0", time.Second)
		require.NoError(t, err)
		require.NoError(t, c.SetProjectID(456))
	})
}

func TestShouldRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		resp   *http.Response
		err    error
		expect bool
	}{
		{"nil response nil error", nil, nil, false},
		{"nil response with error", nil, errors.New("dial tcp: connection refused"), true},
		{"transport error", nil, errors.New("connection refused"), true},
		{"500", &http.Response{StatusCode: http.StatusInternalServerError}, errors.New("err"), true},
		{"502", &http.Response{StatusCode: http.StatusBadGateway}, errors.New("err"), true},
		{"503", &http.Response{StatusCode: http.StatusServiceUnavailable}, errors.New("err"), true},
		{"429", &http.Response{StatusCode: http.StatusTooManyRequests}, errors.New("err"), true},
		{"400 not retryable", &http.Response{StatusCode: http.StatusBadRequest}, errors.New("err"), false},
		{"404 not retryable", &http.Response{StatusCode: http.StatusNotFound}, errors.New("err"), false},
		{"409 not retryable", &http.Response{StatusCode: http.StatusConflict}, errors.New("err"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expect, shouldRetry(tt.resp, tt.err))
		})
	}
}

func TestRetryDo(t *testing.T) {
	t.Parallel()

	t.Run("succeeds on first try", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient(testutils.NewLogger(t), "token", "http://example.com", "1.0", time.Second)
		require.NoError(t, err)

		calls := 0
		err = client.retryDo(t.Context(), func() (bool, error) {
			calls++
			return false, nil
		})
		require.NoError(t, err)
		assert.Equal(t, 1, calls)
	})

	t.Run("retries transient then succeeds", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient(testutils.NewLogger(t), "token", "http://example.com", "1.0", time.Second)
		require.NoError(t, err)
		client.retryInterval = time.Millisecond

		calls := 0
		err = client.retryDo(t.Context(), func() (bool, error) {
			calls++
			if calls < 3 {
				return true, errors.New("bad gateway")
			}
			return false, nil
		})
		require.NoError(t, err)
		assert.Equal(t, 3, calls)
	})

	t.Run("does not retry client errors", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient(testutils.NewLogger(t), "token", "http://example.com", "1.0", time.Second)
		require.NoError(t, err)

		calls := 0
		err = client.retryDo(t.Context(), func() (bool, error) {
			calls++
			return false, errors.New("bad request")
		})
		require.Error(t, err)
		assert.Equal(t, 1, calls)
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient(testutils.NewLogger(t), "token", "http://example.com", "1.0", time.Second)
		require.NoError(t, err)
		client.retryInterval = time.Hour

		ctx, cancel := context.WithCancel(t.Context())
		calls := 0
		err = client.retryDo(ctx, func() (bool, error) {
			calls++
			cancel()
			return true, errors.New("transport error")
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("exhausts retries", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient(testutils.NewLogger(t), "token", "http://example.com", "1.0", time.Second)
		require.NoError(t, err)
		client.retryInterval = time.Millisecond

		calls := 0
		err = client.retryDo(t.Context(), func() (bool, error) {
			calls++
			return true, errors.New("service unavailable")
		})
		require.Error(t, err)
		assert.Equal(t, 4, calls) // 1 initial + 3 retries
	})
}

func TestCheckResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		response            *http.Response
		err                 error
		expectResponseError bool
		expectedError       string
	}{
		{
			name:          "nil response",
			response:      nil,
			expectedError: errUnknown.Error(),
		},
		{
			name:          "successful response 200",
			response:      &http.Response{StatusCode: http.StatusOK},
			expectedError: "",
		},
		{
			name: "unauthorized 401 with invalid JSON",
			response: &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader("invalid json")),
			},
			expectedError: errNotAuthenticated.Error(),
		},
		{
			name: "forbidden 403 with invalid JSON",
			response: &http.Response{
				StatusCode: http.StatusForbidden,
				Body:       io.NopCloser(strings.NewReader("invalid json")),
			},
			expectedError: errNotAuthorized.Error(),
		},
		{
			name: "server error 500 with invalid JSON",
			response: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("invalid json")),
				Request:    &http.Request{URL: mustParseURL(t, "https://api.k6.io/test")},
			},
			expectedError: "unexpected HTTP error from https://api.k6.io/test: 500 Internal Server Error",
		},
		{
			name: "error with valid JSON payload",
			response: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body: io.NopCloser(strings.NewReader(`{
					"error": {
						"message": "validation failed",
						"code": "error"
					}
				}`)),
				Request: &http.Request{URL: mustParseURL(t, "https://api.k6.io/test")},
			},
			expectResponseError: true,
		},
		{
			name: "GenericOpenAPIError with response error with valid JSON payload",
			response: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body: io.NopCloser(strings.NewReader(`{
					"error": {
						"message": "validation failed",
						"code": "error"
					}
				}`)),
				Request: &http.Request{URL: mustParseURL(t, "https://api.k6.io/test")},
			},
			err:                 &k6cloud.GenericOpenAPIError{},
			expectResponseError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := CheckResponse(tt.response, tt.err)

			if tt.expectedError == "" && !tt.expectResponseError {
				assert.NoError(t, err)
				return
			}

			assert.Error(t, err)

			if tt.expectResponseError {
				var respErr ResponseError
				assert.True(t, errors.As(err, &respErr))
				assert.Equal(t, tt.response, respErr.Response)
			} else {
				assert.Equal(t, tt.expectedError, err.Error())
			}
		})
	}
}

func TestToInt32(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       int64
		expected    int32
		expectError bool
	}{
		{"valid positive", 123, 123, false},
		{"valid negative", -456, -456, false},
		{"max int32", 2147483647, 2147483647, false},
		{"min int32", -2147483648, -2147483648, false},
		{"overflow positive", 2147483648, 0, true},
		{"overflow negative", -2147483649, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := toInt32(tt.input)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "overflows int32")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// newTestClient creates a client pointing at the given test server.
func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "1.0", time.Second)
	require.NoError(t, err)
	require.NoError(t, client.SetStackID(123))
	require.NoError(t, client.SetProjectID(456))
	return client
}

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return u
}
