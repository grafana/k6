package provisioning

import (
	"io"
	"net/http"
	"strings"
	"testing"

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
		wantIs     error
	}{
		{"2xx returns nil", 200, "{}", false, 0, nil},
		{"4xx wraps as ResponseError", 400, `{"error":"bad"}`, true, 400, nil},
		{"5xx wraps as ResponseError", 500, `{"error":"oops"}`, true, 500, nil},
		{"401 retains body and classification", 401, `{"error":"unauthenticated"}`, true, 401, httperr.ErrNotAuthenticated},
		{"403 retains body and classification", 403, `{"error":{"code":4}}`, true, 403, httperr.ErrNotAuthorized},
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
			assert.Equal(t, tc.body, respErr.Body)
			if tc.wantIs != nil {
				assert.ErrorIs(t, err, tc.wantIs)
			}
		})
	}
}

func TestResponseError_Error_FormatsStatusAndBody(t *testing.T) {
	t.Parallel()

	re := &ResponseError{
		StatusCode: 422,
		Body:       "validation failed",
	}

	msg := re.Error()
	assert.Contains(t, msg, "422")
	assert.Contains(t, msg, "validation failed")
}
