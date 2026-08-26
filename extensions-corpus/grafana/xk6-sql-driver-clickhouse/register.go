// Package clickhouse contains ClickHouse SQL driver registration for xk6-sql.
package clickhouse

import (
	"github.com/grafana/xk6-sql/sql"

	// Blank imports required for initialization of driver.
	_ "github.com/ClickHouse/clickhouse-go/v2"
)

func init() {
	sql.RegisterModule("clickhouse")
}
