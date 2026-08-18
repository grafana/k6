// Package tcp contains the xk6-tcp k6 extension.
package tcp

import (
	"log/slog"

	extensionapi "go.k6.io/k6-extension-api"
)

// ImportPath is the import path for the TCP module.
const ImportPath = "k6/x/tcp"

// New creates a new TCP module.
func New() extensionapi.Module {
	return new(rootModule)
}

type rootModule struct{}

func (*rootModule) NewModuleInstance(vu extensionapi.VU) extensionapi.Instance {
	logger, ok := vu.(extensionapi.Logger)
	if !ok {
		panic("extension API logger capability is unavailable")
	}
	return &module{
		vu:      vu,
		log:     logger.Logger().With("module", "tcp"),
		metrics: newTCPMetrics(vu),
	}
}

type module struct {
	vu      extensionapi.VU
	log     *slog.Logger
	metrics *tcpMetrics
}

func (m *module) Exports() extensionapi.Exports {
	return extensionapi.Exports{
		Named: map[string]any{
			"Socket": m.socket,
		},
	}
}

var _ extensionapi.Module = (*rootModule)(nil)
