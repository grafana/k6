module github.com/tango-tango/xk6-msgpack

go 1.25.0

require (
	github.com/vmihailenco/msgpack/v5 v5.4.1
	go.k6.io/k6-extension-api v0.0.0
)

require (
	github.com/dlclark/regexp2/v2 v2.5.2 // indirect
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible // indirect
	github.com/google/pprof v0.0.0-20230207041349-798e818bf904 // indirect
	github.com/grafana/sobek v0.0.0-20260727154728-7781506a890f // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	golang.org/x/text v0.39.0 // indirect
)

replace go.k6.io/k6-extension-api => ../../../extension-api
