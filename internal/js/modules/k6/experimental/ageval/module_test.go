package ageval

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
)

func TestModuleExportsAndMetrics(t *testing.T) {
	t.Parallel()
	rt := modulestest.NewRuntime(t)
	registry := rt.VU.InitEnvField.Registry

	mi, ok := New().NewModuleInstance(rt.VU).(*ModuleInstance)
	require.True(t, ok)

	exports := mi.Exports().Named
	assert.Contains(t, exports, "AgentTestCase")
	assert.Contains(t, exports, "AgentSimulator")
	assert.Contains(t, exports, "ExternalAgent")
	assert.Contains(t, exports, "judge")

	for _, name := range []string{
		"agent_duration", "agent_steps", "agent_tool_calls", "agent_tokens",
		"agent_cost_usd", "agent_tool_correctness", "agent_quality_score", "agent_judge_pass",
	} {
		assert.NotNil(t, registry.Get(name), "metric %q should be registered", name)
	}
}
