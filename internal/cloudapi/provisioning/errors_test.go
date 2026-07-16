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
		wantErr    error // nil means "no error"; sentinel means require.ErrorIs
		wantStatus int   // checked via *ResponseError when wantErr is nil and status is non-2xx
	}{
		{name: "2xx returns nil", statusCode: 200, body: "{}"},
		{
			name: "4xx parses into ResponseError", statusCode: 400,
			body: `{"error":{"message":"bad","code":"BAD_REQUEST"}}`, wantStatus: 400,
		},
		{
			name: "5xx parses into ResponseError", statusCode: 500,
			body: `{"error":{"message":"oops","code":"INTERNAL"}}`, wantStatus: 500,
		},
		{
			name: "401 with unparsable body falls back to httperr", statusCode: http.StatusUnauthorized,
			body: "plain text, not JSON", wantErr: httperr.ErrNotAuthenticated,
		},
		{
			name: "403 with unparsable body falls back to httperr", statusCode: http.StatusForbidden,
			body: "plain text, not JSON", wantErr: httperr.ErrNotAuthorized,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resp := &http.Response{
				StatusCode: tc.statusCode,
				Body:       io.NopCloser(strings.NewReader(tc.body)),
			}

			err := CheckResponse(resp)
			switch {
			case tc.statusCode >= 200 && tc.statusCode <= 299:
				assert.NoError(t, err)
			case tc.wantErr != nil:
				require.ErrorIs(t, err, tc.wantErr)
			default:
				var respErr *ResponseError
				require.ErrorAs(t, err, &respErr)
				assert.Equal(t, tc.wantStatus, respErr.StatusCode)
			}
		})
	}
}

func TestCheckResponse_UnparsableBodyWithoutHttperrMatchFallsBackToGenericError(t *testing.T) {
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

func TestResponseError_Error(t *testing.T) {
	t.Parallel()

	re := &ResponseError{
		StatusCode: 422,
		APIError:   k6cloud.ErrorApiModel{Message: "validation failed", Code: "VALIDATION"},
	}

	msg := re.Error()
	assert.Contains(t, msg, "422")
	assert.Contains(t, msg, "VALIDATION")
	assert.Contains(t, msg, "validation failed")
}
