package httperr

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		expected   error
	}{
		{
			name:       "401 returns ErrNotAuthenticated",
			statusCode: http.StatusUnauthorized,
			expected:   ErrNotAuthenticated,
		},
		{
			name:       "403 returns ErrNotAuthorized",
			statusCode: http.StatusForbidden,
			expected:   ErrNotAuthorized,
		},
		{
			name:       "200 returns nil",
			statusCode: http.StatusOK,
			expected:   nil,
		},
		{
			name:       "500 returns nil",
			statusCode: http.StatusInternalServerError,
			expected:   nil,
		},
		{
			name:       "400 returns nil",
			statusCode: http.StatusBadRequest,
			expected:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ClassifyStatus(tt.statusCode)
			if tt.expected == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.expected)
		})
	}
}
