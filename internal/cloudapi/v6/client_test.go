package cloudapi

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			name: "GenericOpenAPIError error with response error with valid JSON payload",
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

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestToInt32(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       int64
		expected    int32
		expectError bool
	}{
		{
			name:        "valid positive value",
			input:       123,
			expected:    123,
			expectError: false,
		},
		{
			name:        "valid negative value",
			input:       -456,
			expected:    -456,
			expectError: false,
		},
		{
			name:        "max int32 value",
			input:       2147483647,
			expected:    2147483647,
			expectError: false,
		},
		{
			name:        "min int32 value",
			input:       -2147483648,
			expected:    -2147483648,
			expectError: false,
		},
		{
			name:        "overflow positive",
			input:       2147483648,
			expectError: true,
		},
		{
			name:        "overflow negative",
			input:       -2147483649,
			expectError: true,
		},
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

func TestRandomStrHex(t *testing.T) {
	t.Parallel()

	// Test that it generates 16 character hex strings
	result := randomStrHex()
	assert.Len(t, result, 16)

	// Test that it's valid hex
	for _, char := range result {
		assert.True(t, (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f'))
	}

	// Test that multiple calls produce different values
	result2 := randomStrHex()
	assert.NotEqual(t, result, result2)
}
