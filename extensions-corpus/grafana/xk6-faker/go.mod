module github.com/grafana/xk6-faker

go 1.25.0

toolchain go1.25.12

require (
	github.com/brianvoe/gofakeit/v6 v6.28.0
	github.com/grafana/sobek v0.0.0-20260727154728-7781506a890f
	github.com/iancoleman/strcase v0.3.0
	github.com/stretchr/testify v1.11.1
	go.k6.io/k6-extension-api v0.0.0
	lukechampine.com/frand v1.4.2
)

replace go.k6.io/k6-extension-api => ../../../extension-api

require (
	github.com/aead/chacha20 v0.0.0-20180709150244-8b13a72661da // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dlclark/regexp2/v2 v2.5.2 // indirect
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible // indirect
	github.com/google/pprof v0.0.0-20260115054156-294ebfa9ad83 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.39.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
