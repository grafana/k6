package cloudapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		preset   string
		wantAuth string
	}{
		{
			name:     "respects preset authorization",
			preset:   "Bearer pre-set-token",
			wantAuth: "Bearer pre-set-token",
		},
		{
			name:     "sets token auth when unset",
			preset:   "",
			wantAuth: "Token mytoken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := NewClient(nil, "mytoken", "http://example.com", "v0.1", 1*time.Second)
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com", nil)
			require.NoError(t, err)
			if tt.preset != "" {
				req.Header.Set("Authorization", tt.preset)
			}
			c.prepareHeaders(req)

			assert.Equal(t, tt.wantAuth, req.Header.Get("Authorization"))
		})
	}
}
