package cloudapi

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/v2/internal/cloudapi/httperr"
)

func TestContains(t *testing.T) {
	t.Parallel()

	s := []string{"a", "b", "c"}

	assert.False(t, contains(s, "e"))
	assert.True(t, contains(s, "b"))
}

func TestErrorResponse_Error(t *testing.T) {
	t.Parallel()

	msg1 := "some message"
	msg2 := "some other message"

	errResp := ResponseError{
		Message: msg1,
		Errors:  []string{msg2},
		FieldErrors: map[string][]string{
			"field1": {"error1", "error2"},
		},
		Code: 123,
	}

	expected := "(E123) " + msg1 + "\n " + msg2 + "\n field1: error1, error2"
	assert.Equal(t, expected, errResp.Error())
}

func TestCheckResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		response      *http.Response
		expectedError error
	}{
		{
			name:          "nil response",
			response:      nil,
			expectedError: errUnknown,
		},
		{
			name:     "successful response 200",
			response: &http.Response{StatusCode: http.StatusOK},
		},
		{
			name: "unauthorized 401 with invalid JSON",
			response: &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader("invalid json")),
			},
			expectedError: httperr.ErrNotAuthenticated,
		},
		{
			name: "forbidden 403 with invalid JSON",
			response: &http.Response{
				StatusCode: http.StatusForbidden,
				Body:       io.NopCloser(strings.NewReader("invalid json")),
			},
			expectedError: httperr.ErrNotAuthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := CheckResponse(tt.response)

			if tt.expectedError == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.expectedError)
		})
	}
}
