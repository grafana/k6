package cloudapi

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/lib/testutils"
)

func TestNewClientConfiguresRetries(t *testing.T) {
	t.Parallel()

	client, err := NewClient(testutils.NewLogger(t), "test-token", "https://api.k6.io", "1.0", time.Second)
	require.NoError(t, err)

	cfg := client.apiClient.GetConfig()
	assert.Equal(t, MaxRetries, cfg.MaxRetries)
	assert.Equal(t, RetryInterval, cfg.RetryInterval)
}

func TestCheckResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		response            *http.Response
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := CheckResponse(tt.response)

			if tt.expectedError == "" && !tt.expectResponseError {
				assert.NoError(t, err)
				return
			}

			assert.Error(t, err)

			if tt.expectResponseError {
				var rerr ResponseError
				assert.True(t, errors.As(err, &rerr))
				assert.Equal(t, tt.response, rerr.Response)
			} else {
				assert.Equal(t, tt.expectedError, err.Error())
			}
		})
	}
}

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
