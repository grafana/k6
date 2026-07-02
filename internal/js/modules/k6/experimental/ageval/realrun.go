package ageval

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

// newAgentTestCase is the `new AgentTestCase({...})` constructor: it builds an AgentTestCase
// from a recorded agent trajectory — no simulation, no agent model call.
// Use it to evaluate a run you already have
// (a logged production run, a captured dataset, a framework shim's output, or a
// raw payload parsed via `format`). To run the agent as part of the k6 test
// instead, use a producer (AgentSimulator / CliAgent) — they return an
// AgentTestCase too, so everything downstream (check/expectSequence/judge) is identical.
//
//	new AgentTestCase({
//	  output, toolCalls: [{ name, input, output }],    // the recorded trajectory
//	  input?,                                           // the task/prompt the agent was given (used by judge)
//	  expectedTools?: [{ name, input? }],               // graded by expectSequence() called with no argument
//	  model?, usage?: { inputTokens, outputTokens },    // optional, enables token/cost metrics
//	  durationMs?, steps?, name?, stepReportTool?, tags?,
//	  format?, raw?,                                    // alternative: parse a raw payload via an adapter
//	})
//
// It emits the same k6 metrics as a simulated run for whatever data is provided
// (always agent_tool_calls; agent_duration/steps/tokens/cost when supplied), so
// recorded and simulated runs share the same dashboards and thresholds.
func (mi *ModuleInstance) newAgentTestCase(call sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()
	state := mi.vu.State()
	if state == nil {
		common.Throw(rt, errInitContext)
	}
	arg := call.Argument(0)
	if common.IsNullish(arg) {
		common.Throw(rt, errors.New("new AgentTestCase() requires an object with `output` and/or `toolCalls`"))
	}
	o := arg.ToObject(rt)

	modelName := getString(o, "model", "")
	tags := mi.realRunTags(state, getString(o, "name", defaultAgentName), modelName, o.Get("tags"))
	tr := mi.trajectoryFromConfig(rt, o)

	result := mi.newRealAgentTestCase(rt, realRunData{
		tags:           tags,
		stepReportTool: getString(o, "stepReportTool", defaultStepReportTool),
		input:          getString(o, "input", ""),
		output:         tr.output,
		model:          modelName,
		toolCalls:      tr.toolCalls,
		inTok:          tr.inTok,
		outTok:         tr.outTok,
		durationMs:     getFloat(o, "durationMs", 0),
		steps:          getInt(o, "steps", len(tr.toolCalls)),
	})
	result.ExpectedTools = parseToolCalls(rt, o.Get("expectedTools"))
	return rt.ToValue(result).ToObject(rt)
}

// trajectoryFromConfig builds the trajectory from either a `format` + `raw`
// payload (run through the adapter registry) or the explicit
// `output`/`toolCalls`/`usage` fields.
func (mi *ModuleInstance) trajectoryFromConfig(rt *sobek.Runtime, o *sobek.Object) trajectory {
	if format := getString(o, "format", ""); format != "" {
		adapter, ok := lookupAdapter(format)
		if !ok {
			common.Throw(rt, fmt.Errorf("unknown format %q (supported: %s)",
				format, strings.Join(adapterNames(), ", ")))
		}
		return adapter(getString(o, "raw", ""))
	}
	return trajectoryFromJS(rt, o)
}

// realRunData is the provider-agnostic trajectory used to build an AgentTestCase for
// both new AgentTestCase and CliAgent.
type realRunData struct {
	tags           *metrics.TagSet
	stepReportTool string
	input          string
	output         string
	model          string
	toolCalls      []ToolCall
	inTok          int64
	outTok         int64
	durationMs     float64
	steps          int
}

// newRealAgentTestCase builds an AgentTestCase from a real (non-simulated) trajectory and
// emits its metrics.
func (mi *ModuleInstance) newRealAgentTestCase(rt *sobek.Runtime, d realRunData) *AgentTestCase {
	if d.toolCalls == nil {
		d.toolCalls = []ToolCall{}
	}
	result := &AgentTestCase{
		vu:             mi.vu,
		rt:             rt,
		metrics:        mi.metrics,
		tags:           d.tags,
		stepReportTool: d.stepReportTool,
		Input:          d.input,
		Output:         d.output,
		ToolCalls:      d.toolCalls,
		Usage:          RunUsage{InputTokens: d.inTok, OutputTokens: d.outTok},
		Steps:          d.steps,
		Duration:       d.durationMs,
	}
	mi.emitRealRunMetrics(result, d.tags, d.model, d.inTok, d.outTok)
	return result
}

// realRunTags builds the base tag set (agent + model + user tags) for a real run.
func (mi *ModuleInstance) realRunTags(
	state *lib.State, name, model string, userTags sobek.Value,
) *metrics.TagSet {
	tags := state.Tags.GetCurrentValues().Tags.With("agent", name)
	if model != "" {
		tags = tags.With("model", model)
	}
	if userTags != nil && !common.IsNullish(userTags) {
		obj := userTags.ToObject(mi.vu.Runtime())
		for _, k := range obj.Keys() {
			tags = tags.With(k, obj.Get(k).String())
		}
	}
	return tags
}

// emitRealRunMetrics emits the metrics derivable from a provided trajectory.
func (mi *ModuleInstance) emitRealRunMetrics(
	result *AgentTestCase, tags *metrics.TagSet, modelName string, inTok, outTok int64,
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
