package ageval

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentRunRecordsTrajectoryAndMetrics(t *testing.T) {
	t.Parallel()
	ts := newTestSetup(t)
	srv := cannedServer(t, toolUseResponse, endTurnResponse)

	_, err := ts.rt.VU.Runtime().RunString(fmt.Sprintf(`
		globalThis.calledWith = null;
		const agent = new AgentSimulator({
			provider: "anthropic",
			model: "claude-sonnet-4-5",
			apiKey: "test",
			baseURL: %q,
			systemPrompt: "be helpful",
			tools: [{
				name: "echo",
				description: "echoes",
				inputSchema: { type: "object", properties: { msg: { type: "string" } } },
				mock: (input) => { globalThis.calledWith = input.msg; return "echoed: " + input.msg; },
			}],
		});
		globalThis.r = agent.run({ input: "say hi", tags: { case: "smoke" } });
	`, srv.URL))
	require.NoError(t, err)

	rt := ts.rt.VU.Runtime()
	assert.Equal(t, "hi", rt.Get("calledWith").String())

	r := rt.Get("r").ToObject(rt)
	assert.Equal(t, "all done, invoice paid", r.Get("output").String())
	assert.Equal(t, int64(2), r.Get("steps").ToInteger())

	// Trajectory recorded.
	require.NoError(t, rt.Set("res", r))
	v, err := rt.RunString(`res.toolCalls.length === 1 && res.calledTool("echo") && res.toolCalls[0].output === "echoed: hi"`)
	require.NoError(t, err)
	assert.True(t, v.ToBoolean())

	// Token usage summed across both model calls.
	assert.Equal(t, int64(22), r.Get("usage").ToObject(rt).Get("inputTokens").ToInteger())
	assert.Equal(t, int64(11), r.Get("usage").ToObject(rt).Get("outputTokens").ToInteger())

	samples := drainSamples(ts.samples)
	assert.Len(t, samples["agent_duration"], 1)
	assert.Equal(t, []float64{2}, samples["agent_steps"])
	assert.Equal(t, []float64{1}, samples["agent_tool_calls"])
	require.Len(t, samples["agent_tokens"], 2) // input + output
	require.Len(t, samples["agent_cost_usd"], 1)
	// cost = 22/1e6*3 + 11/1e6*15
	assert.InDelta(t, 22.0/1e6*3+11.0/1e6*15, samples["agent_cost_usd"][0], 1e-12)
}

func TestAgentExpectSequenceEmitsCorrectness(t *testing.T) {
	t.Parallel()
	ts := newTestSetup(t)
	srv := cannedServer(t, toolUseResponse, endTurnResponse)

	v, err := ts.rt.VU.Runtime().RunString(fmt.Sprintf(`
		const agent = new AgentSimulator({
			model: "claude-sonnet-4-5", apiKey: "t", baseURL: %q,
			tools: [{ name: "echo", description: "d", mock: () => "ok" }],
		});
		const r = agent.run({ input: "go" });
		const good = r.expectSequence([{ name: "echo", args: { msg: "hi" } }], { mode: "in-order" });
		const bad  = r.expectSequence([{ name: "missing" }], { mode: "in-order" });
		[good, bad];
	`, srv.URL))
	require.NoError(t, err)
	arr := v.ToObject(ts.rt.VU.Runtime())
	assert.True(t, arr.Get("0").ToBoolean())
	assert.False(t, arr.Get("1").ToBoolean())

	samples := drainSamples(ts.samples)
	assert.ElementsMatch(t, []float64{1, 0}, samples["agent_tool_correctness"])
}

func TestAgentRejectsUnsupportedModel(t *testing.T) {
	t.Parallel()
	ts := newTestSetup(t)
	_, err := ts.rt.VU.Runtime().RunString(`new AgentSimulator({ model: "gpt-4", apiKey: "t" });`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported model")
}

func TestAgentSkillMergesToolsAndInstructions(t *testing.T) {
	t.Parallel()
	ts := newTestSetup(t)
	srv := cannedServer(t, endTurnResponse)

	_, err := ts.rt.VU.Runtime().RunString(fmt.Sprintf(`
		const agent = new AgentSimulator({
			model: "claude-sonnet-4-5", apiKey: "t", baseURL: %q,
			systemPrompt: "base",
			skills: [{ name: "s", instructions: "do the thing", tools: [{ name: "skilltool", description: "d", mock: () => "x" }] }],
		});
		globalThis.r2 = agent.run({ input: "go" });
	`, srv.URL))
	require.NoError(t, err)
	// The run completes (end_turn immediately); the skill tool is registered without error.
	assert.NotNil(t, ts.rt.VU.Runtime().Get("r2"))
}
