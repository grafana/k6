module github.com/grafana/xk6-sql-driver-sqlserver

go 1.25.7

toolchain go1.25.14

require (
	github.com/grafana/xk6-sql v1.2.1
	github.com/microsoft/go-mssqldb v1.10.0
)

require (
	github.com/dlclark/regexp2/v2 v2.5.2 // indirect
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/google/pprof v0.0.0-20260115054156-294ebfa9ad83 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grafana/sobek v0.0.0-20260727154728-7781506a890f // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	go.k6.io/k6-extension-api v0.0.0 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/text v0.39.0 // indirect
)

replace go.k6.io/k6-extension-api => ../../../extension-api

replace github.com/grafana/xk6-sql => ../xk6-sql
