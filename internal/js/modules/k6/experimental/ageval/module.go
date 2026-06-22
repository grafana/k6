// Package ageval is an experimental k6 module for evaluating tool-using LLM
// agents. It builds an "ageval test" from an agent's tool-call trajectory in one
// of two ways:
//
//   - AgentSimulator — simulates an agent by running a model loop against a
//     provider (Anthropic) with mocked tools, so you can exercise a prompt's
//     behavior without a live backend; or
//   - fromAgentRun — takes a real agent's recorded output + tool calls directly,
//     with no simulation, so you can evaluate your production agent's actual run.
//
// Either way it produces a RunResult that scripts assert on with
// check()/expectSequence() and an LLM-as-judge, and it emits standard k6 metrics
// (Trend/Rate/Counter) so results visualize in k6 Cloud and Grafana with no extra
// configuration.
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
			"AgentSimulator": mi.newAgentSimulator,
			"ExternalAgent":  mi.newExternalAgent,
			"fromAgentRun":   mi.fromAgentRun,
			"judge":          mi.judge,
		},
	}
}
