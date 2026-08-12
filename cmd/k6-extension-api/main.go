// k6-extension-api builds a local k6 binary with extensions that use the new
// standalone extension API.
package main

import (
	_ "github.com/grafana/xk6-disruptor"
	_ "github.com/grafana/xk6-faker"
	_ "github.com/grafana/xk6-kubernetes"
	_ "github.com/grafana/xk6-sql"
	_ "github.com/grafana/xk6-sql-driver-azuresql"
	_ "github.com/grafana/xk6-sql-driver-clickhouse"
	_ "github.com/grafana/xk6-sql-driver-mysql"
	_ "github.com/grafana/xk6-sql-driver-postgres"
	_ "github.com/grafana/xk6-sql-driver-sqlserver"
	_ "github.com/grafana/xk6-ssh"
	_ "github.com/grafana/xk6-tls"
	_ "github.com/tango-tango/xk6-msgpack"
	"go.k6.io/k6/v2/cmd"
)

func main() {
	cmd.Execute()
}
