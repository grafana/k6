// Package mqtt contains the xk6-mqtt extension.
package mqtt

import (
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/v2/js/modules"
)

// ImportPath is the import path for the MQTT module.
const ImportPath = "k6/x/mqtt"

// New creates a new MQTT module.
func New() modules.Module {
	return new(rootModule)
}

type rootModule struct{}

func (*rootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &module{
		vu: vu,
		log: vu.
			InitEnv().
			Logger.
			WithField("module", "mqtt"),
		metrics: newMqttMetrics(vu),
	}
}

type module struct {
	vu      modules.VU
	log     logrus.FieldLogger
	metrics *mqttMetrics
}

func (m *module) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]any{
			"Client": m.client,
		},
	}
}

var _ modules.Module = (*rootModule)(nil)
