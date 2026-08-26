package mysql

import (
	_ "embed"
	"os"
	"testing"

	"github.com/grafana/xk6-sql/sqltest"
)

//go:embed testdata/script.js
var script string

func TestIntegration(t *testing.T) { //nolint:paralleltest
	if testing.Short() {
		t.Skip()
	}

	conn := os.Getenv("MYSQL_CONNECTION_STRING")
	if conn == "" {
		t.Skip("MYSQL_CONNECTION_STRING is not set")
	}

	sqltest.RunScript(t, "mysql", conn, script)
}
