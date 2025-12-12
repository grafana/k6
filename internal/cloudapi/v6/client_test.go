package cloudapi

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
