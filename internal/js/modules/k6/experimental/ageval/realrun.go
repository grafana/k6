package ageval

import (
	"errors"
	"strconv"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/metrics"
)

// fromAgentRun builds a RunResult from a real agent's recorded trajectory, with
// no simulation and no model call for the agent. Use it to evaluate your
// production agent's actual run: capture its final output and the tools it
// called, pass them here, then assert with check()/expectSequence() and judge().
//
//	input: {
//	  output, toolCalls: [{ name, input, output }],   // the recorded trajectory
//	  input?,                                          // the task/prompt the agent was given (used by judge)
//	  model?, usage?: { inputTokens, outputTokens },  // optional, enables token/cost metrics
//	  durationMs?, steps?, name?, stepReportTool?, tags?,
//	}
//
// It emits the same k6 metrics as a simulated run for whatever data is provided
// (always agent_tool_calls; agent_duration/steps/tokens/cost when supplied), so
// real and simulated runs share the same dashboards and thresholds.
func (mi *ModuleInstance) fromAgentRun(input sobek.Value) sobek.Value {
	rt := mi.vu.Runtime()
	state := mi.vu.State()
	if state == nil {
		common.Throw(rt, errInitContext)
	}
	if input == nil || common.IsNullish(input) {
		common.Throw(rt, errors.New("fromAgentRun() requires an object with `output` and/or `toolCalls`"))
	}
	o := input.ToObject(rt)

	name := getString(o, "name", defaultAgentName)
	modelName := getString(o, "model", "")

	tags := state.Tags.GetCurrentValues().Tags.With("agent", name)
	if modelName != "" {
		tags = tags.With("model", modelName)
	}
	if ut := o.Get("tags"); ut != nil && !common.IsNullish(ut) {
		uto := ut.ToObject(rt)
		for _, k := range uto.Keys() {
			tags = tags.With(k, uto.Get(k).String())
		}
	}

	result := &RunResult{
		vu:             mi.vu,
		rt:             rt,
		metrics:        mi.metrics,
		tags:           tags,
		stepReportTool: getString(o, "stepReportTool", defaultStepReportTool),
		Input:          getString(o, "input", ""),
		Output:         getString(o, "output", ""),
		ToolCalls:      parseToolCalls(rt, o.Get("toolCalls")),
	}
	result.Steps = getInt(o, "steps", len(result.ToolCalls))
	result.Duration = getFloat(o, "durationMs", 0)

	var inTok, outTok int64
	if uv := o.Get("usage"); uv != nil && !common.IsNullish(uv) {
		uo := uv.ToObject(rt)
		inTok = int64(getInt(uo, "inputTokens", 0))
		outTok = int64(getInt(uo, "outputTokens", 0))
		result.Usage = RunUsage{InputTokens: inTok, OutputTokens: outTok}
	}

	mi.emitRealRunMetrics(result, tags, modelName, inTok, outTok)
	return rt.ToValue(result).ToObject(rt)
}

// emitRealRunMetrics emits the metrics derivable from a provided trajectory.
func (mi *ModuleInstance) emitRealRunMetrics(
	result *RunResult, tags *metrics.TagSet, modelName string, inTok, outTok int64,
) {
	for _, c := range result.ToolCalls {
		pushSample(mi.vu, mi.metrics.toolCalls, tags.With("tool", c.Name), 1)
	}
	if result.Duration > 0 {
		pushSample(mi.vu, mi.metrics.duration, tags, result.Duration)
	}
	if result.Steps > 0 {
		pushSample(mi.vu, mi.metrics.steps, tags, float64(result.Steps))
	}
	if inTok > 0 || outTok > 0 {
		pushSample(mi.vu, mi.metrics.tokens, tags.With("direction", "input"), float64(inTok))
		pushSample(mi.vu, mi.metrics.tokens, tags.With("direction", "output"), float64(outTok))
		if info, ok := modelPricing(modelName); ok {
			cost := float64(inTok)/1e6*info.inUSDPerMTok + float64(outTok)/1e6*info.outUSDPerMTok
			pushSample(mi.vu, mi.metrics.cost, tags, cost)
		}
	}
}

// parseToolCalls reads a JS array of `{ name, input, output }` into []ToolCall.
func parseToolCalls(rt *sobek.Runtime, v sobek.Value) []ToolCall {
	out := []ToolCall{}
	if v == nil || common.IsNullish(v) {
		return out
	}
	arr := v.ToObject(rt)
	lengthVal := arr.Get("length")
	if lengthVal == nil {
		return out
	}
	n := int(lengthVal.ToInteger())
	for i := range n {
		item := arr.Get(strconv.Itoa(i))
		if item == nil || sobek.IsUndefined(item) {
			continue
		}
		obj := item.ToObject(rt)
		tc := ToolCall{
			Name:   getString(obj, "name", ""),
			Output: getString(obj, "output", ""),
			Input:  map[string]any{},
		}
		if iv := obj.Get("input"); iv != nil && !common.IsNullish(iv) {
			if m, ok := iv.Export().(map[string]any); ok {
				tc.Input = m
			}
		}
		out = append(out, tc)
	}
	return out
}

// modelPricing looks up a model's pricing across all registered providers.
func modelPricing(modelName string) (modelInfo, bool) {
	if modelName == "" {
		return modelInfo{}, false
	}
	for _, p := range providers {
		if info, ok := p.model(modelName); ok {
			return info, true
		}
	}
	return modelInfo{}, false
}
