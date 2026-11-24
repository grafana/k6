package clientcert

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, origin, want string
	}{
		{name: "lowercase_host", origin: "https://Example.COM", want: "https://example.com"},
		{name: "default_port", origin: "https://example.com:443", want: "https://example.com"},
		{name: "path_query_fragment", origin: "https://example.com:8443/path?q#f", want: "https://example.com:8443"},
		{name: "ipv6", origin: "https://[::1]:8443", want: "https://[::1]:8443"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeOrigin(tt.origin)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeOrigin_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, origin, wantErr string
	}{
		{name: "http", origin: "http://example.com", wantErr: "origin must use https"},
		{name: "wss", origin: "wss://example.com", wantErr: "origin must use https"},
		{name: "no_scheme", origin: "example.com", wantErr: "invalid origin"},
		{name: "no_host", origin: "https://", wantErr: "invalid origin"},
		{name: "unparsable", origin: "://", wantErr: "invalid origin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NormalizeOrigin(tt.origin)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
