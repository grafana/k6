// Package ageval is an experimental k6 module for evaluating tool-using LLM
// agents. It runs an agent loop against a model provider (Anthropic), records
// the tool-call trajectory, lets scripts assert on it with check()/expectSequence
// and an LLM-as-judge, and emits standard k6 metrics (Trend/Rate/Counter) so the
// results visualize in k6 Cloud and Grafana with no extra configuration.
package ageval

import (
	"go.k6.io/k6/v2/js/modules"
)

type (
	// RootModule is the global module instance that creates a ModuleInstance
	// per VU.
	RootModule struct{}

	// ModuleInstance is the per-VU instance of the ageval module.
	ModuleInstance struct {
		vu      modules.VU
		metrics *agevalMetrics
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface and returns a new
// instance of the module for the given VU. Metrics are registered here, in the
// init context, where the registry is available.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &ModuleInstance{
		vu:      vu,
		metrics: registerMetrics(vu.InitEnv().Registry),
	}
}

// Exports implements the modules.Instance interface.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]any{
			"Agent": mi.newAgent,
			"judge": mi.judge,
		},
	}
}
