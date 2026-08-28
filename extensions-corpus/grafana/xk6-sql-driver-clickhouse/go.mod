module github.com/grafana/xk6-sql-driver-clickhouse

go 1.25.0

toolchain go1.25.14

require (
	github.com/ClickHouse/clickhouse-go/v2 v2.47.0
	github.com/grafana/xk6-sql v1.2.1
)

require (
	github.com/ClickHouse/ch-go v0.73.0 // indirect
	github.com/andybalholm/brotli v1.2.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dlclark/regexp2/v2 v2.5.2 // indirect
	github.com/go-faster/city v1.0.1 // indirect
	github.com/go-faster/errors v0.7.1 // indirect
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible // indirect
	github.com/google/pprof v0.0.0-20260115054156-294ebfa9ad83 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grafana/sobek v0.0.0-20260727154728-7781506a890f // indirect
	github.com/klauspost/compress v1.18.7 // indirect
	github.com/paulmach/orb v0.13.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.27 // indirect
	github.com/segmentio/asm v1.2.1 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	go.k6.io/k6-extension-api v0.0.0 // indirect
	go.opentelemetry.io/otel v1.44.0 // indirect
	go.opentelemetry.io/otel/trace v1.44.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.39.0 // indirect
)

replace go.k6.io/k6-extension-api => ../../../extension-api

replace github.com/grafana/xk6-sql => ../xk6-sql
