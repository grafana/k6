module github.com/grafana/xk6-loki

go 1.26.4

require (
	github.com/brianvoe/gofakeit/v6 v6.9.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/snappy v1.0.0
	github.com/grafana/loki/pkg/push v0.0.0-20260611205623-ac76b402773c
	github.com/grafana/loki/v3 v3.0.0-20260611205623-ac76b402773c
	github.com/grafana/sobek v0.0.0-20260727154728-7781506a890f
	github.com/mailru/easyjson v0.7.7
	github.com/prometheus/common v0.67.5
	go.k6.io/k6-extension-api v0.0.0
)

replace go.k6.io/k6-extension-api => ../../../extension-api

require (
	github.com/dlclark/regexp2/v2 v2.5.2 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.6.1 // indirect
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible // indirect
	github.com/google/pprof v0.0.0-20260302011040-a15ffb7f9dcc // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260406210006-6f92a3bedf2d // indirect
	google.golang.org/grpc v1.80.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

// Use fork of gocql that has gokit logs and Prometheus metrics.
replace github.com/gocql/gocql => github.com/grafana/gocql v0.0.0-20200605141915-ba5dc39ece85

replace github.com/Azure/go-ntlmssp => github.com/Azure/go-ntlmssp v0.1.1

exclude k8s.io/client-go v8.0.0+incompatible
