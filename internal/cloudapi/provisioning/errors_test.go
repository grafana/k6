package provisioning

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{"4xx wraps as ResponseError", 400, `{"error":"bad"}`, true, 400},
		{"5xx wraps as ResponseError", 500, `{"error":"oops"}`, true, 500},
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
