package sql

import (
	"testing"

	"github.com/grafana/xk6-sql/sql"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/ext"
)

func Test_register(t *testing.T) {
	t.Parallel()

	require.Contains(t, ext.Get(ext.JSExtension), sql.ImportPath)
}
