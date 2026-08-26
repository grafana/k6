package sql_test

import (
	_ "embed"
	"testing"

	"github.com/grafana/xk6-sql/sql"
	"github.com/grafana/xk6-sql/sqltest"
	_ "github.com/proullon/ramsql/driver"
)

//go:embed testdata/script.js
var script string

func TestMain(m *testing.M) {
	sql.RegisterModule("ramsql")

	m.Run()
}

// TestIntegration performs an integration test creating a ramsql database.
func TestIntegration(t *testing.T) {
	t.Parallel()

	sqltest.RunScript(t, "ramsql", "testdb", script)
}

func TestOptions(t *testing.T) {
	t.Parallel()

	sqltest.RunScript(t, "ramsql", "testdb", `const db = sql.open(driver, connection, { conn_max_idle_time: "5s" });`)
}
