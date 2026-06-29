// Package ageval is an experimental k6 module for evaluating tool-using LLM
// agents. Its single data type is the AgentTestCase
// LLMTestCase — holding an agent run's input, output, tool calls and usage. You
// obtain an AgentTestCase in one of three ways:
//
//   - new AgentTestCase({...}) — wrap a recorded trajectory you already have (a logged
//     production run, a captured dataset, a framework's output, or a raw payload
//     parsed via a `format` adapter); no agent is run;
//   - AgentSimulator.run() — an optional producer that simulates an agent by
//     running a model loop against a provider (Anthropic) with mocked tools; or
//   - CliAgent.run() — an optional producer that runs a real agent CLI as a
//     subprocess (so the agent runs as part of the k6 test, even under load); or
//   - SigilAgent.run() — like CliAgent, but the trajectory is collected from the
//     Sigil (Grafana AI observability) telemetry the instrumented agent streams
//     to an ageval-hosted gRPC endpoint, instead of being parsed from stdout.
//
// All yield an AgentTestCase that scripts assert on with check()/expectSequence()
// and an LLM-as-judge, and it emits standard k6 metrics (Trend/Rate/Counter) so
// results visualize in k6 Cloud and Grafana with no extra configuration.
package ageval

import (
	"sync"

	"go.k6.io/k6/v2/js/modules"
)

type (
	// RootModule is the global module instance that creates a ModuleInstance
	// per VU. It also holds the process-wide Sigil ingest server, lazily started
	// and shared across all VUs (see sigil.go).
	RootModule struct {
		mu     sync.Mutex
		server *sigilServer
	}

	// ModuleInstance is the per-VU instance of the ageval module.
	ModuleInstance struct {
		vu      modules.VU
		metrics *agevalMetrics
		root    *RootModule
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
func (rm *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &ModuleInstance{
		vu:      vu,
		metrics: registerMetrics(vu.InitEnv().Registry),
		root:    rm,
	}
}

// Exports implements the modules.Instance interface.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]any{
			"AgentTestCase":  mi.newAgentTestCase,
			"AgentSimulator": mi.newAgentSimulator,
			"CliAgent":       mi.newCliAgent,
			"SigilAgent":     mi.newSigilAgent,
			"sigilSummary":   mi.sigilSummary,
			"judge":          mi.judge,
		},
	}
}
