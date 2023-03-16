// This is the opposite constraint of config_supported.go
//go:build !(amd64 || arm64) || !(darwin || linux || freebsd || windows)

package wazero

func newRuntimeConfig() RuntimeConfig {
	return NewRuntimeConfigInterpreter()
}
