// Package azuresql contains azuresql SQL driver registration for xk6-sql.
package azuresql

import (
	"github.com/grafana/xk6-sql/sql"

	"github.com/microsoft/go-mssqldb/azuread"
)

func init() {
	sql.RegisterModule(azuread.DriverName)
}
