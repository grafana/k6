package cloudapi

import (
	"net/http"
	"testing"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/stretchr/testify/assert"
)

func TestResponseError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		respErr  ResponseError
		expected string
	}{
		{
			name: "basic error message",
			respErr: ResponseError{
				APIError: k6cloud.ErrorApiModel{
					Message: "test error",
					Code:    "error",
				},
			},
			expected: "(error) test error",
		},
		{
			name: "error with target",
			respErr: ResponseError{
				APIError: k6cloud.ErrorApiModel{
					Message: "validation failed",
					Target:  *k6cloud.NewNullableString(k6cloud.PtrString("field_name")),
				},
			},
			expected: "validation failed (target: 'field_name')",
		},
		{
			name: "error with details",
			respErr: ResponseError{
				APIError: k6cloud.ErrorApiModel{
					Message: "validation error",
					Details: []k6cloud.ErrorDetailsApiModel{
						{
							Message: "field is required",
							Target:  *k6cloud.NewNullableString(k6cloud.PtrString("first_property")),
						},
						{
							Message: "field must be positive",
							Target:  *k6cloud.NewNullableString(k6cloud.PtrString("second_property")),
						},
					},
				},
			},
			expected: "validation error\nfield is required (target: 'first_property')\nfield must be positive (target: 'second_property')",
		},
		{
			name: "error with HTTP response and API code",
			respErr: ResponseError{
				Response: &http.Response{StatusCode: http.StatusBadRequest},
				APIError: k6cloud.ErrorApiModel{
					Message: "bad request",
					Code:    "error",
				},
			},
			expected: "(400/error) bad request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.respErr.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}
