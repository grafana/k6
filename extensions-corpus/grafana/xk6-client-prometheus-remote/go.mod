module github.com/grafana/xk6-client-prometheus-remote

go 1.25.0

toolchain go1.25.12

require (
	github.com/golang/snappy v1.0.0
	github.com/grafana/sobek v0.0.0-20260727154728-7781506a890f
	github.com/pkg/errors v0.9.1
	github.com/prometheus/prometheus v0.313.0
	github.com/stretchr/testify v1.11.1
	github.com/xhit/go-str2duration/v2 v2.1.0
	go.k6.io/k6-extension-api v0.0.0
	google.golang.org/protobuf v1.36.11
)

replace go.k6.io/k6-extension-api => ../../../extension-api

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dlclark/regexp2/v2 v2.5.2 // indirect
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/pprof v0.0.0-20260604005048-7023385849c0 // indirect
	github.com/grafana/regexp v0.0.0-20250905093917-f7b3be9d1853 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.69.0 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	golang.org/x/text v0.39.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
