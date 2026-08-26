// Package icmp contains the xk6-icmp extension.
package icmp

import (
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/v2/js/modules"
)

// ImportPath is the import path for the ICMP module.
const ImportPath = "k6/x/icmp"

// New creates a new ICMP module.
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
			WithField("module", "icmp"),
		lookupEnv: vu.InitEnv().LookupEnv,
		metrics:   newIcmpMetrics(vu),
	}
}

type module struct {
	vu        modules.VU
	log       logrus.FieldLogger
	metrics   *icmpMetrics
	lookupEnv func(string) (string, bool)
}

func (m *module) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]any{
			"ping":      m.ping,
			"pingAsync": m.pingAsync,
		},
	}
}

var _ modules.Module = (*rootModule)(nil)
