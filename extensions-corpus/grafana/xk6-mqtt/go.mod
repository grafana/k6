module github.com/grafana/xk6-mqtt

go 1.25.0

toolchain go1.25.12

require (
	github.com/eclipse/paho.mqtt.golang v1.5.1
	github.com/grafana/sobek v0.0.0-20260727154728-7781506a890f
	github.com/mstoykov/k6-taskqueue-lib v0.1.3
	go.k6.io/k6-extension-api v0.0.0
)

replace go.k6.io/k6-extension-api => ../../../extension-api

// test dependencies
require (
	github.com/mochi-mqtt/server/v2 v2.7.9
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dlclark/regexp2/v2 v2.5.2 // indirect
	github.com/evanw/esbuild v0.28.0 // indirect
	github.com/fatih/color v1.19.0 // indirect
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible // indirect
	github.com/google/pprof v0.0.0-20260115054156-294ebfa9ad83 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/mailru/easyjson v0.9.2 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/rs/xid v1.4.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.44.0 // indirect
	go.opentelemetry.io/otel/sdk v1.44.0 // indirect
	go.opentelemetry.io/otel/trace v1.44.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/text v0.39.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	google.golang.org/grpc v1.82.1 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
