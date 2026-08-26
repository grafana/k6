// Package sqlserver contains Microsoft SQL Server driver registration for xk6-sql.
package sqlserver

import (
	"github.com/grafana/xk6-sql/sql"

	// Blank import required for initialization of driver.
	_ "github.com/microsoft/go-mssqldb"
)

func init() {
	sql.RegisterModule("sqlserver")
}
