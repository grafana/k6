// Package tcp contains the xk6-tcp k6 extension.
package tcp

import (
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/v2/js/modules"
)

// ImportPath is the import path for the TCP module.
const ImportPath = "k6/x/tcp"

// New creates a new TCP module.
func New() modules.Module {
	return new(rootModule)
}

type rootModule struct{}

func (*rootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &module{
		vu:      vu,
		log:     vu.InitEnv().Logger.WithField("module", "tcp"),
		metrics: newTCPMetrics(vu),
	}
}

type module struct {
	vu      modules.VU
	log     logrus.FieldLogger
	metrics *tcpMetrics
}

func (m *module) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]any{
			"Socket": m.socket,
		},
	}
}

var _ modules.Module = (*rootModule)(nil)
