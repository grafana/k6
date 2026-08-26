// Adapted more or less unchanged from: https://github.com/grafana/xk6-sql/blob/v0.4.1/sql_test.go#L99
// It will have to be refactored.

package mysql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrefixConnectionString(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		connectionString string
		want             string
	}{
		{
			name:             "HappyPath",
			connectionString: "root:password@tcp(localhost:3306)/mysql",
			want:             "root:password@tcp(localhost:3306)/mysql?tls=custom",
		},
		{
			name:             "WithExistingParams",
			connectionString: "root:password@tcp(localhost:3306)/mysql?param=value",
			want:             "root:password@tcp(localhost:3306)/mysql?param=value&tls=custom",
		},
		{
			name:             "WithExistingTLSparam",
			connectionString: "root:password@tcp(localhost:3306)/mysql?tls=custom",
			want:             "root:password@tcp(localhost:3306)/mysql?tls=custom",
		},
		{
			name:             "WithExistingTLSparam",
			connectionString: "root:password@tcp(localhost:3306)/mysql?tls=notcustom",
			want:             "root:password@tcp(localhost:3306)/mysql?tls=notcustom&tls=custom",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := prefixConnectionString(tc.connectionString, "custom")
			require.Equal(t, tc.want, got)
		})
	}
}
