// Package icmp contains the xk6-icmp extension.
package icmp

import extensionapi "go.k6.io/k6-extension-api"

// ImportPath is the import path for the ICMP module.
const ImportPath = "k6/x/icmp"

// New creates a new ICMP module.
func New() extensionapi.Module { return new(rootModule) }

type rootModule struct{}

func (*rootModule) NewModuleInstance(vu extensionapi.VU) extensionapi.Instance {
	logger, ok := vu.(extensionapi.Logger)
	if !ok {
		panic("extension API logger capability is unavailable")
	}
	lookupEnv := func(string) (string, bool) { return "", false }
	if environment, ok := vu.(extensionapi.Environment); ok {
		lookupEnv = environment.LookupEnv
	}
	return &module{
		vu:        vu,
		log:       newLogger(logger.Logger().With("module", "icmp")),
		lookupEnv: lookupEnv,
		metrics:   newICMPMetrics(vu),
	}
}

type module struct {
	vu        extensionapi.VU
	log       extensionLogger
	metrics   *icmpMetrics
	lookupEnv func(string) (string, bool)
}

func (m *module) Exports() extensionapi.Exports {
	return extensionapi.Exports{Named: map[string]any{
		"ping":      m.ping,
		"pingAsync": m.pingAsync,
	}}
}

var _ extensionapi.Module = (*rootModule)(nil)
var _ extensionapi.Instance = (*module)(nil)
