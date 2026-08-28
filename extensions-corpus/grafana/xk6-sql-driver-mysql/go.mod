module github.com/grafana/xk6-sql-driver-mysql

go 1.25.0

toolchain go1.25.14

require (
	github.com/go-sql-driver/mysql v1.10.0
	github.com/grafana/sobek v0.0.0-20260727154728-7781506a890f
	github.com/grafana/xk6-sql v1.2.1
	github.com/stretchr/testify v1.11.1
	go.k6.io/k6-extension-api v0.0.0
)

replace go.k6.io/k6-extension-api => ../../../extension-api

replace github.com/grafana/xk6-sql => ../xk6-sql

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dlclark/regexp2/v2 v2.5.2 // indirect
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible // indirect
	github.com/google/pprof v0.0.0-20260115054156-294ebfa9ad83 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/text v0.39.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
