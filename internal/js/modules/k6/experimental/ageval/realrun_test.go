package ageval

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentTestCaseBuildsResultAndMetrics(t *testing.T) {
	t.Parallel()
	ts := newTestSetup(t)

	v, err := ts.rt.VU.Runtime().RunString(`
		const r = new AgentTestCase({
			model: "claude-sonnet-4-5",
			output: "Invoice INV-123 is paid.",
			toolCalls: [
				{ name: "get_customer", input: { email: "a@b.com" }, output: "{...}" },
				{ name: "get_invoice", input: { invoice_id: "INV-123" }, output: "{...}" },
			],
			usage: { inputTokens: 1000, outputTokens: 200 },
			durationMs: 4200,
			tags: { case: "real" },
		});
		[
			r.calledTool("get_customer"),
			r.calledTool("get_invoice"),
			r.toolCalls.length,
			r.output,
			r.usage.inputTokens,
		];
	`)
	require.NoError(t, err)

	rt := ts.rt.VU.Runtime()
	arr := v.ToObject(rt)
	assert.True(t, arr.Get("0").ToBoolean())
	assert.True(t, arr.Get("1").ToBoolean())
	assert.Equal(t, int64(2), arr.Get("2").ToInteger())
	assert.Equal(t, "Invoice INV-123 is paid.", arr.Get("3").String())
	assert.Equal(t, int64(1000), arr.Get("4").ToInteger())

	samples := drainSamples(ts.samples)
	assert.Equal(t, []float64{4200}, samples["agent_duration"])
	assert.Equal(t, []float64{2}, samples["agent_steps"]) // defaulted to toolCalls length
	require.Len(t, samples["agent_tool_calls"], 2)
	require.Len(t, samples["agent_tokens"], 2)
	// cost = 1000/1e6*3 + 200/1e6*15
	require.Len(t, samples["agent_cost_usd"], 1)
	assert.InDelta(t, 1000.0/1e6*3+200.0/1e6*15, samples["agent_cost_usd"][0], 1e-12)
}

func TestAgentTestCaseMinimalNoOptionalMetrics(t *testing.T) {
	t.Parallel()
	ts := newTestSetup(t)

	_, err := ts.rt.VU.Runtime().RunString(`
		globalThis.r = new AgentTestCase({
			output: "done",
			toolCalls: [{ name: "search", input: {}, output: "ok" }],
		});
	`)
	require.NoError(t, err)

	samples := drainSamples(ts.samples)
	// Only tool_calls is derivable without usage/duration.
	require.Len(t, samples["agent_tool_calls"], 1)
	assert.Empty(t, samples["agent_duration"])
	assert.Empty(t, samples["agent_tokens"])
	assert.Empty(t, samples["agent_cost_usd"])
}

func TestAgentTestCaseExpectedToolsFallback(t *testing.T) {
	t.Parallel()
	ts := newTestSetup(t)

	v, err := ts.rt.VU.Runtime().RunString(`
		const expectedTools = [
			{ name: "get_customer" },
			{ name: "get_invoice", input: { invoice_id: "INV-123" } },
		];
		const good = new AgentTestCase({
			output: "ok",
			toolCalls: [
				{ name: "get_customer", input: { email: "a@b.com" }, output: "{}" },
				{ name: "get_invoice", input: { invoice_id: "INV-123" }, output: "paid" },
			],
			expectedTools,
		});
		const bad = new AgentTestCase({
			output: "ok",
			toolCalls: [{ name: "get_invoice", input: { invoice_id: "INV-999" }, output: "?" }],
			expectedTools,
		});
		// expectSequence() with NO argument grades against the stored expectedTools.
		[good.expectSequence(), bad.expectSequence()];
	`)
	require.NoError(t, err)

	rt := ts.rt.VU.Runtime()
	arr := v.ToObject(rt)
	assert.True(t, arr.Get("0").ToBoolean(), "good run matches expectedTools")
	assert.False(t, arr.Get("1").ToBoolean(), "bad run fails expectedTools (wrong invoice id)")

	samples := drainSamples(ts.samples)
	assert.Equal(t, []float64{1, 0}, samples["agent_tool_correctness"])
}

func TestAgentTestCaseIsJudgeable(t *testing.T) {
	t.Parallel()
	ts := newTestSetup(t)
	judgeSrv := cannedServer(t, judgeResponse)

	v, err := ts.rt.VU.Runtime().RunString(`
		const r = new AgentTestCase({
			output: "Invoice INV-123 is paid.",
			toolCalls: [{ name: "get_invoice", input: { invoice_id: "INV-123" }, output: "paid" }],
			tags: { case: "real_judge" },
		});
		// Explicit expectSequence([...]) form with exact mode (still supported API).
		const seqOK = r.expectSequence([{ name: "get_invoice" }], { mode: "exact" });
		const verdict = judge(r, {
			name: "invoice_paid", model: "claude-sonnet-4-5", apiKey: "jt", baseURL: ` + "`" + judgeSrv.URL + "`" + `,
			rubric: "The answer states the invoice is paid.", threshold: 0.7,
		});
		[seqOK, verdict.score, verdict.passed];
	`)
	require.NoError(t, err)
	rt := ts.rt.VU.Runtime()
	arr := v.ToObject(rt)
	assert.True(t, arr.Get("0").ToBoolean())
	assert.InDelta(t, 0.9, arr.Get("1").ToFloat(), 1e-9)
	assert.True(t, arr.Get("2").ToBoolean())

	samples := drainSamples(ts.samples)
	assert.Equal(t, []float64{1}, samples["agent_tool_correctness"])
	assert.Equal(t, []float64{0.9}, samples["agent_quality_score"])
}
