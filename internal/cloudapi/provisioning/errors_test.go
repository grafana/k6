package provisioning

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/cloudapi/httperr"
)

func TestCheckResponse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
		wantStatus int
	}{
		{"2xx returns nil", 200, "{}", false, 0},
		{"4xx wraps as ResponseError", 400, `{"error":{"message":"bad","code":"BAD_REQUEST"}}`, true, 400},
		{"5xx wraps as ResponseError", 500, `{"error":{"message":"oops","code":"INTERNAL"}}`, true, 500},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resp := &http.Response{
				StatusCode: tc.statusCode,
				Body:       io.NopCloser(strings.NewReader(tc.body)),
			}

			err := CheckResponse(resp)
			if !tc.wantErr {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			var respErr *ResponseError
			require.ErrorAs(t, err, &respErr)
			assert.Equal(t, tc.wantStatus, respErr.StatusCode)
		})
	}
}

func TestCheckResponse_401And403FallBackToHttperr(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		statusCode int
		wantErr    error
	}{
		{"401 non-JSON body", http.StatusUnauthorized, httperr.ErrNotAuthenticated},
		{"403 non-JSON body", http.StatusForbidden, httperr.ErrNotAuthorized},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resp := &http.Response{
				StatusCode: tc.statusCode,
				Body:       io.NopCloser(strings.NewReader("plain text, not JSON")),
			}

			err := CheckResponse(resp)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestCheckResponse_UnparsableBodyFallsBackToGenericError(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: http.StatusTeapot,
		Body:       io.NopCloser(strings.NewReader("plain text, not JSON")),
		Request:    &http.Request{URL: &url.URL{Scheme: "https", Host: "api.k6.io", Path: "/provisioning/v1/test"}},
	}

	err := CheckResponse(resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "418")
}

func TestResponseError_Error_FormatsStatusAndBody(t *testing.T) {
	t.Parallel()

	re := &ResponseError{
		StatusCode: 422,
		APIError: k6cloud.ErrorApiModel{
			Message: "validation failed",
			Code:    "VALIDATION",
		},
	}

	msg := re.Error()
	assert.Contains(t, msg, "422")
	assert.Contains(t, msg, "VALIDATION")
	assert.Contains(t, msg, "validation failed")
}
