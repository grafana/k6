// Package postgres contains PostgreSQL SQL driver registration for xk6-sql.
package postgres

import (
	"github.com/grafana/xk6-sql/sql"

	// Blank import required for initialization of driver.
	_ "github.com/lib/pq"
)

func init() {
	sql.RegisterModule("postgres")
}
