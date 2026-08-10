module github.com/grafana/xk6-ssh

go 1.25.0

toolchain go1.25.14

require (
	github.com/spf13/afero v1.15.0
	go.k6.io/k6-extension-api v0.0.0
	golang.org/x/crypto v0.54.0
)

require (
	github.com/dlclark/regexp2/v2 v2.5.2 // indirect
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible // indirect
	github.com/google/pprof v0.0.0-20230207041349-798e818bf904 // indirect
	github.com/grafana/sobek v0.0.0-20260727154728-7781506a890f // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
)

replace go.k6.io/k6-extension-api => ../../../extension-api
