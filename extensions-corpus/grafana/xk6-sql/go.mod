module github.com/grafana/xk6-sql

go 1.25.0

toolchain go1.25.14

require (
	github.com/grafana/sobek v0.0.0-20260727154728-7781506a890f
	github.com/proullon/ramsql v0.1.4
	github.com/stretchr/testify v1.11.1
	go.k6.io/k6-extension-api v0.0.0
)

replace go.k6.io/k6-extension-api => ../../../extension-api

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dlclark/regexp2/v2 v2.5.2 // indirect
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible // indirect
	github.com/google/pprof v0.0.0-20260115054156-294ebfa9ad83 // indirect
	github.com/jackc/pgx/v5 v5.9.2 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	golang.org/x/exp v0.0.0-20230713183714-613f0c0eb8a1 // indirect
	golang.org/x/text v0.39.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
