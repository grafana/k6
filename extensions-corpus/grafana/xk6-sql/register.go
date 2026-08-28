// Package sql is the primary go package of the xk6-sql extension.
// Contains the registration of the xk6-sql extension as a k6 extension.
package sql

import (
	"github.com/grafana/xk6-sql/sql"
	extensionapi "go.k6.io/k6-extension-api"
)

func init() {
	extensionapi.Register(sql.ImportPath, sql.New())
}
