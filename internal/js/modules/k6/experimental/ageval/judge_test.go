package ageval

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJudgeScoresAndEmitsMetrics(t *testing.T) {
	t.Parallel()
	ts := newTestSetup(t)
	agentSrv := cannedServer(t, toolUseResponse, endTurnResponse)
	judgeSrv := cannedServer(t, judgeResponse)

	v, err := ts.rt.VU.Runtime().RunString(fmt.Sprintf(`
		const agent = new AgentSimulator({
			model: "claude-sonnet-4-5", apiKey: "t", baseURL: %q,
			tools: [{ name: "echo", description: "d", mock: () => "ok" }],
		});
		const r = agent.run({ input: "go", tags: { case: "judge" } });
		judge(r, {
			model: "claude-sonnet-4-5",
			apiKey: "jt",
			baseURL: %q,
			rubric: "The agent should call echo and finish.",
			threshold: 0.7,
		});
	`, agentSrv.URL, judgeSrv.URL))
	require.NoError(t, err)

	rt := ts.rt.VU.Runtime()
	res := v.ToObject(rt)
	assert.InDelta(t, 0.9, res.Get("score").ToFloat(), 1e-9)
	assert.True(t, res.Get("passed").ToBoolean())
	assert.Equal(t, "good", res.Get("reason").String())

	samples := drainSamples(ts.samples)
	assert.Equal(t, []float64{0.9}, samples["agent_quality_score"])
	assert.Equal(t, []float64{1}, samples["agent_judge_pass"])
}

func TestParseJudgeReply(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		text      string
		wantScore float64
		wantOK    bool
	}{
		{"clean json", `{"score": 0.8, "reason": "ok"}`, 0.8, true},
		{"surrounding prose", "Here is my verdict: {\"score\": 0.5, \"reason\": \"meh\"} done", 0.5, true},
		{"clamps high", `{"score": 1.5, "reason": "x"}`, 1.0, true},
		{"clamps low", `{"score": -2, "reason": "x"}`, 0.0, true},
		{"no json", `I cannot score this`, 0.0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			score, reason := parseJudgeReply(reply{blocks: []block{{kind: blockText, text: tc.text}}})
			assert.InDelta(t, tc.wantScore, score, 1e-9)
			if !tc.wantOK {
				assert.NotEmpty(t, reason)
			}
		})
	}
}
