package sqltest_test

import (
	_ "embed"
	"testing"

	"github.com/grafana/xk6-sql/sql"
	"github.com/grafana/xk6-sql/sqltest"
	_ "github.com/proullon/ramsql/driver"
)

//go:embed testdata/script.js
var script string

func TestRunScript(t *testing.T) {
	t.Parallel()

	sql.RegisterModule("ramsql")

	sqltest.RunScript(t, "ramsql", "testdb", script)
}
