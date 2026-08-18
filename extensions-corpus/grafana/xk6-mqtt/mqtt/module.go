// Package mqtt contains the xk6-mqtt extension.
package mqtt

import (
	"log/slog"

	extensionapi "go.k6.io/k6-extension-api"
)

// ImportPath is the import path for the MQTT module.
const ImportPath = "k6/x/mqtt"

// New creates a new MQTT module.
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
		log:     logger.Logger().With("module", "mqtt"),
		metrics: newMqttMetrics(vu),
	}
}

type module struct {
	vu      extensionapi.VU
	log     *slog.Logger
	metrics *mqttMetrics
}

func (m *module) Exports() extensionapi.Exports {
	return extensionapi.Exports{
		Named: map[string]any{
			"Client": m.client,
		},
	}
}

var _ extensionapi.Module = (*rootModule)(nil)
