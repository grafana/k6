package ageval

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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

// judge runs an LLM-as-judge (GEval-style) over an AgentTestCase: it scores the
// agent's behavior against a natural-language rubric on a 0..1 scale, emits the
// agent_quality_score (Trend) and agent_judge_pass (Rate) metrics, and returns
// `{ score, reason, passed }`.
//
// `name` is REQUIRED: it tags agent_quality_score / agent_judge_pass (and the judge
// cost/token metrics) with eval=<name> so each eval is identifiable in dashboards and
// downstream tools — which matters because the rubric/reason are not metrics. The
// judge also logs a parseable line (ageval_eval {json}) carrying name/score/passed/
// reason, so the reason is recoverable from k6 logs even under --local-execution.
// The judge's own spend is emitted as agent_judge_tokens / agent_judge_cost_usd,
// tagged with the judge model (separate from the agent's agent_tokens/agent_cost_usd).
//
// opts: { name, provider, model, apiKey, rubric, threshold?, input?, actualOutput?, baseURL? }
func (mi *ModuleInstance) judge(resultVal sobek.Value, opts sobek.Value) sobek.Value {
	rt := mi.vu.Runtime()
	if mi.vu.State() == nil {
		common.Throw(rt, errInitContext)
	}
	if opts == nil || common.IsNullish(opts) {
		common.Throw(rt, errors.New("judge() requires an options object with a `rubric`"))
	}
	o := opts.ToObject(rt)

	name := getString(o, "name", "")
	if name == "" {
		common.Throw(rt, errors.New("judge() requires a non-empty `name` (the eval label; emitted as the `eval` tag)"))
	}
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

	rr := mi.exportAgentTestCase(resultVal)
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

	// eval = which eval; threshold = its judge cutoff (low-cardinality, so tools can
	// read the real per-eval threshold from the metrics — distinct from the k6
	// options.thresholds config).
	tags := mi.judgeTags(rr).With("eval", name).With("threshold", strconv.FormatFloat(threshold, 'g', -1, 64))
	pushSample(mi.vu, mi.metrics.qualityScore, tags, score)
	passVal := 0.0
	if passed {
		passVal = 1
	}
	pushSample(mi.vu, mi.metrics.judgePass, tags, passVal)
	mi.emitJudgeCost(tags, prov, modelName, rep.usage)
	mi.logEval(evalLog{
		name:      name,
		score:     score,
		threshold: threshold,
		passed:    passed,
		reason:    reason,
		rubric:    rubric,
		input:     input,
		actual:    actual,
	})

	return rt.ToValue(judgeResult{Score: score, Reason: reason, Passed: passed}).ToObject(rt)
}

// logEvalPrefix marks judge result log lines so tools can find and parse them (the
// JSON after the prefix carries the eval result). Logged because the rubric/reason
// can't be metric tags; under --local-execution logs stay on the CLI, and under
// cloud execution they reach the cloud Logs tab.
const logEvalPrefix = "ageval_eval"

type evalLog struct {
	name      string
	score     float64
	threshold float64
	passed    bool
	reason    string
	rubric    string
	input     string
	actual    string
}

// logEval logs one parseable `ageval_eval {json}` line per judge. A FAILING eval is
// logged at Error level (prominent on the CLI, filterable as level=error in the
// cloud) and carries extra debugging detail — the rubric, the task input and the
// agent's actual behavior — so the failure is self-explanatory. A passing eval logs
// a lean line at Info level.
func (mi *ModuleInstance) logEval(e evalLog) {
	state := mi.vu.State()
	if state == nil || state.Logger == nil {
		return
	}
	payload := struct {
		Name      string  `json:"name"`
		Score     float64 `json:"score"`
		Threshold float64 `json:"threshold"`
		Passed    bool    `json:"passed"`
		Reason    string  `json:"reason"`
		Rubric    string  `json:"rubric,omitempty"`
		Input     string  `json:"input,omitempty"`
		Actual    string  `json:"actual,omitempty"`
	}{Name: e.name, Score: e.score, Threshold: e.threshold, Passed: e.passed, Reason: e.reason}
	if !e.passed {
		// Pack as much context as possible on failure (truncated to keep the line sane).
		payload.Rubric = truncate(e.rubric, 1000)
		payload.Input = truncate(e.input, 1000)
		payload.Actual = truncate(e.actual, 2000)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return
	}
	msg := logEvalPrefix + " " + string(encoded)
	if e.passed {
		state.Logger.Info(msg)
	} else {
		state.Logger.Error(msg)
	}
}

// emitJudgeCost records the judge model's own token usage and estimated USD spend
// (distinct from the agent's). It tags the samples with the judge model so judge
// spend is attributable separately from agent spend.
func (mi *ModuleInstance) emitJudgeCost(tags *metrics.TagSet, prov provider, modelName string, u usage) {
	if u.inputTokens == 0 && u.outputTokens == 0 {
		return
	}
	jt := tags.With("model", modelName)
	pushSample(mi.vu, mi.metrics.judgeTokens, jt.With("direction", "input"), float64(u.inputTokens))
	pushSample(mi.vu, mi.metrics.judgeTokens, jt.With("direction", "output"), float64(u.outputTokens))
	if info, ok := prov.model(modelName); ok {
		cost := float64(u.inputTokens)/1e6*info.inUSDPerMTok + float64(u.outputTokens)/1e6*info.outUSDPerMTok
		pushSample(mi.vu, mi.metrics.judgeCost, jt, cost)
		// Also emit micro-USD (integer-scale) so sub-cent costs survive cloud rounding.
		pushSample(mi.vu, mi.metrics.judgeCostMicroUsd, jt, cost*1e6)
	}
}

// exportAgentTestCase recovers the *AgentTestCase wrapped in a JS value, or nil.
func (mi *ModuleInstance) exportAgentTestCase(v sobek.Value) *AgentTestCase {
	if v == nil || common.IsNullish(v) {
		return nil
	}
	if rr, ok := v.Export().(*AgentTestCase); ok {
		return rr
	}
	return nil
}

func (mi *ModuleInstance) judgeTags(rr *AgentTestCase) *metrics.TagSet {
	if rr != nil && rr.tags != nil {
		return rr.tags
	}
	return mi.vu.State().Tags.GetCurrentValues().Tags
}

func serializeTrajectory(rr *AgentTestCase) string {
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
