// Package sql is the primary go package of the xk6-sql extension.
// Contains the registration of the xk6-sql extension as a k6 extension.
package sql

import (
	"github.com/grafana/xk6-sql/sql"
	"go.k6.io/k6/v2/js/modules"
)

func init() {
	modules.Register(sql.ImportPath, sql.New())
}
