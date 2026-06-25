package ageval

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJudgeNameTagAndCost(t *testing.T) {
	t.Parallel()
	ts := newTestSetup(t)
	srv := cannedServer(t, judgeResponse) // usage: 3 input / 4 output tokens

	_, err := ts.rt.VU.Runtime().RunString(fmt.Sprintf(`
		const r = new AgentTestCase({ input: "q", output: "Invoice INV-123 is paid." });
		judge(r, {
			name: "answer_quality",
			model: "claude-sonnet-4-5", apiKey: "jt", baseURL: %q,
			rubric: "The answer states the invoice is paid.",
		});
	`, srv.URL))
	require.NoError(t, err)

	type tagged struct {
		value float64
		tags  map[string]string
	}
	got := map[string][]tagged{}
	for done := false; !done; {
		select {
		case sc := <-ts.samples:
			for _, s := range sc.GetSamples() {
				got[s.Metric.Name] = append(got[s.Metric.Name], tagged{s.Value, s.GetTags().Map()})
			}
		default:
			done = true
		}
	}

	// (1) the score/pass metrics carry the metric=<name> tag.
	require.Len(t, got["agent_quality_score"], 1)
	assert.Equal(t, "answer_quality", got["agent_quality_score"][0].tags["eval"])
	require.Len(t, got["agent_judge_pass"], 1)
	assert.Equal(t, "answer_quality", got["agent_judge_pass"][0].tags["eval"])

	// (2) the judge's own spend is emitted, tagged with the judge model.
	require.Len(t, got["agent_judge_tokens"], 2) // input + output
	require.Len(t, got["agent_judge_cost_usd"], 1)
	assert.Equal(t, "claude-sonnet-4-5", got["agent_judge_cost_usd"][0].tags["model"])
	// cost = 3/1e6*3 + 4/1e6*15 (sonnet-4-5 pricing $3/$15 per Mtok)
	assert.InDelta(t, 3.0/1e6*3+4.0/1e6*15, got["agent_judge_cost_usd"][0].value, 1e-12)
}

func TestJudgeRequiresName(t *testing.T) {
	t.Parallel()
	ts := newTestSetup(t)
	_, err := ts.rt.VU.Runtime().RunString(`
		const r = new AgentTestCase({ input: "q", output: "x" });
		judge(r, { model: "claude-sonnet-4-5", apiKey: "k", rubric: "ok" });
	`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

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
			name: "echoes",
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

func TestJudgeIncludesRunInputInPrompt(t *testing.T) {
	t.Parallel()
	ts := newTestSetup(t)

	var captured string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured = string(body)
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(judgeResponse))
	}))
	t.Cleanup(srv.Close)

	_, err := ts.rt.VU.Runtime().RunString(fmt.Sprintf(`
		const r = new AgentTestCase({
			input: "Count the go files in the repo",
			output: "There are 18 go files.",
			toolCalls: [{ name: "Glob", input: { pattern: "*.go" }, output: "18 files" }],
		});
		// Note: no input passed to judge() -- it must fall back to the run's input.
		judge(r, { name: "count", model: "claude-sonnet-4-5", apiKey: "jt", baseURL: %q, rubric: "Reports a count." });
	`, srv.URL))
	require.NoError(t, err)
	assert.Contains(t, captured, "Count the go files in the repo", "judge prompt should include the run's task/input")
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
