package sql

import (
	"testing"

	"github.com/grafana/xk6-sql/sql"
	"github.com/stretchr/testify/require"
	extensionapi "go.k6.io/k6-extension-api"
)

func Test_register(t *testing.T) {
	t.Parallel()

	require.Contains(t, extensionapi.Registered(), sql.ImportPath)
}
