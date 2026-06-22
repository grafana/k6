package ageval

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/metrics"
)

const defaultJudgeThreshold = 0.7

// judgeResult is returned to JS from judge().
type judgeResult struct {
	Score  float64 `js:"score"`
	Reason string  `js:"reason"`
	Passed bool    `js:"passed"`
}

// judge runs an LLM-as-judge (GEval-style) over a RunResult: it scores the
// agent's behavior against a natural-language rubric on a 0..1 scale, emits the
// agent_quality_score (Trend) and agent_judge_pass (Rate) metrics, and returns
// `{ score, reason, passed }`.
//
// opts: { provider, model, apiKey, rubric, threshold?, input?, actualOutput?, baseURL? }
func (mi *ModuleInstance) judge(resultVal sobek.Value, opts sobek.Value) sobek.Value {
	rt := mi.vu.Runtime()
	if mi.vu.State() == nil {
		common.Throw(rt, errInitContext)
	}
	if opts == nil || common.IsNullish(opts) {
		common.Throw(rt, errors.New("judge() requires an options object with a `rubric`"))
	}
	o := opts.ToObject(rt)

	rubric := getString(o, "rubric", "")
	if rubric == "" {
		common.Throw(rt, errors.New("judge() requires a non-empty `rubric`"))
	}
	providerName := getString(o, "provider", "anthropic")
	prov, ok := lookupProvider(providerName)
	if !ok {
		common.Throw(rt, fmt.Errorf("unknown provider %q (supported: anthropic)", providerName))
	}
	modelName := getString(o, "model", "")
	if _, ok := prov.model(modelName); !ok {
		common.Throw(rt, fmt.Errorf("unsupported judge model %q for provider %q", modelName, providerName))
	}
	apiKey := getString(o, "apiKey", "")
	if apiKey == "" {
		common.Throw(rt, errors.New("judge() requires a non-empty `apiKey`"))
	}
	threshold := getFloat(o, "threshold", defaultJudgeThreshold)

	rr := mi.exportRunResult(resultVal)
	// The task/prompt the agent was given. Default to the run's own input so the
	// judge always sees what the agent was asked to do, not just what it did.
	input := getString(o, "input", "")
	if input == "" && rr != nil {
		input = rr.Input
	}
	actual := getString(o, "actualOutput", "")
	if actual == "" && rr != nil {
		actual = serializeTrajectory(rr)
	}

	prompt := buildJudgePrompt(rubric, input, actual)
	rep, err := prov.createMessage(mi.vu.Context(), conversation{
		model:    modelName,
		apiKey:   apiKey,
		baseURL:  getString(o, "baseURL", ""),
		messages: []message{{role: roleUser, blocks: []block{{kind: blockText, text: prompt}}}},
	})
	if err != nil {
		common.Throw(rt, fmt.Errorf("judge model call failed: %w", err))
	}

	score, reason := parseJudgeReply(rep)
	passed := score >= threshold

	tags := mi.judgeTags(rr)
	pushSample(mi.vu, mi.metrics.qualityScore, tags, score)
	passVal := 0.0
	if passed {
		passVal = 1
	}
	pushSample(mi.vu, mi.metrics.judgePass, tags, passVal)

	return rt.ToValue(judgeResult{Score: score, Reason: reason, Passed: passed}).ToObject(rt)
}

// exportRunResult recovers the *RunResult wrapped in a JS value, or nil.
func (mi *ModuleInstance) exportRunResult(v sobek.Value) *RunResult {
	if v == nil || common.IsNullish(v) {
		return nil
	}
	if rr, ok := v.Export().(*RunResult); ok {
		return rr
	}
	return nil
}

func (mi *ModuleInstance) judgeTags(rr *RunResult) *metrics.TagSet {
	if rr != nil && rr.tags != nil {
		return rr.tags
	}
	return mi.vu.State().Tags.GetCurrentValues().Tags
}

func serializeTrajectory(rr *RunResult) string {
	var sb strings.Builder
	for i, c := range rr.ToolCalls {
		args, err := json.Marshal(c.Input)
		if err != nil {
			args = []byte("{}")
		}
		fmt.Fprintf(&sb, "  %d. %s(%s) → %s\n", i+1, c.Name, string(args), c.Output)
	}
	if rr.Output != "" {
		fmt.Fprintf(&sb, "Final message: %s\n", rr.Output)
	}
	return sb.String()
}

func buildJudgePrompt(rubric, input, actual string) string {
	var sb strings.Builder
	sb.WriteString("You are an expert evaluator scoring an AI agent's behavior against a rubric.\n")
	sb.WriteString("Score how well the agent meets the rubric on a scale from 0.0 (fails) to 1.0 (fully meets).\n")
	sb.WriteString("Respond with ONLY a JSON object and nothing else: ")
	sb.WriteString(`{"score": <number 0..1>, "reason": "<one sentence>"}` + "\n\n")
	sb.WriteString("Rubric:\n")
	sb.WriteString(rubric)
	sb.WriteString("\n\n")
	if input != "" {
		sb.WriteString("Task / context:\n")
		sb.WriteString(input)
		sb.WriteString("\n\n")
	}
	sb.WriteString("Agent actual behavior:\n")
	sb.WriteString(actual)
	sb.WriteString("\n")
	return sb.String()
}

// parseJudgeReply extracts {score, reason} from the judge model's text reply,
// tolerating surrounding prose by scanning for the JSON object.
func parseJudgeReply(rep reply) (float64, string) {
	var text strings.Builder
	for _, b := range rep.blocks {
		if b.kind == blockText {
			text.WriteString(b.text)
		}
	}
	raw := text.String()
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end < start {
		return 0, "judge did not return parseable JSON: " + truncate(raw, 200)
	}
	var parsed struct {
		Score  float64 `json:"score"`
		Reason string  `json:"reason"`
	}
	if err := json.Unmarshal([]byte(raw[start:end+1]), &parsed); err != nil {
		return 0, "judge JSON parse error: " + err.Error()
	}
	if parsed.Score < 0 {
		parsed.Score = 0
	} else if parsed.Score > 1 {
		parsed.Score = 1
	}
	return parsed.Score, parsed.Reason
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func getFloat(obj *sobek.Object, key string, def float64) float64 {
	v := obj.Get(key)
	if v == nil || sobek.IsUndefined(v) || sobek.IsNull(v) {
		return def
	}
	return v.ToFloat()
}
