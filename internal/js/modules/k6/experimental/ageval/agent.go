package ageval

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/metrics"
)

// errInitContext is thrown when run()/judge() are called outside a VU iteration.
var errInitContext = errors.New("agent.run() must be called in the VU (default) function, not in the init context")

const (
	defaultMaxSteps       = 30
	defaultMaxTokens      = 2048
	defaultStepReportTool = "report_step"
	defaultAgentName      = "agent"
)

// AgentSimulator is the JS-facing agent. It is created via `new AgentSimulator({...})` and exposes
// a single `run(opts)` method.
type AgentSimulator struct {
	mi *ModuleInstance
	rt *sobek.Runtime

	name           string
	prov           provider
	info           modelInfo
	model          string
	apiKey         string
	baseURL        string
	system         string
	maxSteps       int
	maxTokens      int
	terminalTool   string
	stepReportTool string
	registry       *toolRegistry
}

// newAgentSimulator is the `new AgentSimulator({...})` constructor.
func (mi *ModuleInstance) newAgentSimulator(call sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()
	if len(call.Arguments) < 1 || common.IsNullish(call.Argument(0)) {
		common.Throw(rt, errors.New("AgentSimulator constructor requires a config object"))
	}
	cfg := call.Argument(0).ToObject(rt)

	providerName := getString(cfg, "provider", "anthropic")
	prov, ok := lookupProvider(providerName)
	if !ok {
		common.Throw(rt, fmt.Errorf("unknown provider %q (supported: anthropic)", providerName))
	}

	modelName := getString(cfg, "model", "")
	info, ok := prov.model(modelName)
	if !ok {
		supported := prov.modelNames()
		sort.Strings(supported)
		common.Throw(rt, fmt.Errorf("unsupported model %q for provider %q (supported: %s)",
			modelName, providerName, strings.Join(supported, ", ")))
	}

	a := &AgentSimulator{
		mi:             mi,
		rt:             rt,
		name:           getString(cfg, "name", defaultAgentName),
		prov:           prov,
		info:           info,
		model:          modelName,
		apiKey:         getString(cfg, "apiKey", ""),
		baseURL:        getString(cfg, "baseURL", ""),
		system:         getString(cfg, "systemPrompt", ""),
		maxSteps:       getInt(cfg, "maxSteps", defaultMaxSteps),
		maxTokens:      getInt(cfg, "maxTokens", defaultMaxTokens),
		terminalTool:   getString(cfg, "terminalTool", ""),
		stepReportTool: getString(cfg, "stepReportTool", defaultStepReportTool),
		registry:       newToolRegistry(),
	}
	if a.apiKey == "" {
		common.Throw(rt, errors.New("AgentSimulator config requires a non-empty apiKey"))
	}

	mi.parseTools(cfg.Get("tools"), a.registry)
	a.applySkills(cfg.Get("skills"))

	return rt.ToValue(a).ToObject(rt)
}

// applySkills merges each skill's instructions into the system prompt and
// registers its tools. A skill is `{ name, instructions, tools }` — the
// lightweight Claude-API tool-use notion of a skill (instructions + tools).
func (a *AgentSimulator) applySkills(v sobek.Value) {
	if v == nil || common.IsNullish(v) {
		return
	}
	arr := v.ToObject(a.rt)
	n := int(arr.Get("length").ToInteger())
	for i := range n {
		item := arr.Get(strconv.Itoa(i))
		if item == nil || sobek.IsUndefined(item) {
			continue
		}
		obj := item.ToObject(a.rt)
		if instr := getString(obj, "instructions", ""); instr != "" {
			if a.system != "" {
				a.system += "\n\n"
			}
			a.system += instr
		}
		a.mi.parseTools(obj.Get("tools"), a.registry)
	}
}

// runMocks holds per-run tool result overrides (string or JS callable).
type runMocks map[string]sobek.Value

// Run executes the agent loop synchronously and returns an AgentTestCase. Blocking is
// intentional and idiomatic (like http.request): we run on the VU goroutine and
// hold the runtime, so JS mock handlers can be called directly.
//
//	run({ input, mocks?, expectedTools?: [{ name, input? }], tags? })
//
// expectedTools is attached to the returned AgentTestCase so expectSequence() can
// grade against it with no argument.
func (a *AgentSimulator) Run(opts sobek.Value) sobek.Value {
	state := a.mi.vu.State()
	if state == nil {
		common.Throw(a.rt, errInitContext)
	}
	if opts == nil || common.IsNullish(opts) {
		common.Throw(a.rt, errors.New("agent.run() requires an options object with an `input`"))
	}
	o := opts.ToObject(a.rt)
	input := getString(o, "input", "")
	if input == "" {
		common.Throw(a.rt, errors.New("agent.run() requires a non-empty `input`"))
	}
	mocks := a.parseRunMocks(o.Get("mocks"))
	tags := a.baseTags(o.Get("tags"))

	result := &AgentTestCase{
		vu:             a.mi.vu,
		rt:             a.rt,
		metrics:        a.mi.metrics,
		tags:           tags,
		stepReportTool: a.stepReportTool,
		Input:          input,
		ToolCalls:      []ToolCall{},
		ExpectedTools:  parseToolCalls(a.rt, o.Get("expectedTools")),
	}

	start := time.Now()
	totalIn, totalOut := a.converse(input, mocks, tags, result)
	result.Usage = RunUsage{InputTokens: totalIn, OutputTokens: totalOut}
	result.Duration = float64(time.Since(start)) / float64(time.Millisecond)

	m := a.mi.metrics
	pushSample(a.mi.vu, m.duration, tags, result.Duration)
	pushSample(a.mi.vu, m.steps, tags, float64(result.Steps))
	pushSample(a.mi.vu, m.tokens, tags.With("direction", "input"), float64(totalIn))
	pushSample(a.mi.vu, m.tokens, tags.With("direction", "output"), float64(totalOut))
	cost := float64(totalIn)/1e6*a.info.inUSDPerMTok + float64(totalOut)/1e6*a.info.outUSDPerMTok
	pushSample(a.mi.vu, m.cost, tags, cost)

	return a.rt.ToValue(result).ToObject(a.rt)
}

// converse runs the model/tool loop, filling result and returning total token
// usage. It stops on end_turn, the configured terminalTool, maxSteps, or a turn
// with no tool calls.
func (a *AgentSimulator) converse(
	input string, mocks runMocks, tags *metrics.TagSet, result *AgentTestCase,
) (int64, int64) {
	messages := []message{{role: roleUser, blocks: []block{{kind: blockText, text: input}}}}
	var totalIn, totalOut int64

	for step := range a.maxSteps {
		rep, err := a.prov.createMessage(a.mi.vu.Context(), conversation{
			model:     a.model,
			apiKey:    a.apiKey,
			baseURL:   a.baseURL,
			system:    a.system,
			maxTokens: a.maxTokens,
			messages:  messages,
			tools:     a.registry.schemas(),
		})
		if err != nil {
			common.Throw(a.rt, fmt.Errorf("agent model call failed: %w", err))
		}
		result.Steps = step + 1
		totalIn += rep.usage.inputTokens
		totalOut += rep.usage.outputTokens

		asst := message{role: roleAssistant}
		toolResults, hitTerminal := a.handleBlocks(rep.blocks, mocks, tags, result, &asst)
		messages = append(messages, asst)

		if rep.stopReason == "end_turn" || hitTerminal || len(toolResults) == 0 {
			break
		}
		messages = append(messages, message{role: roleUser, blocks: toolResults})
	}
	return totalIn, totalOut
}

// handleBlocks processes the model's reply blocks: records text/tool calls,
// dispatches tools, emits the tool-call metric, and returns the tool-result
// blocks plus whether the terminal tool was hit.
func (a *AgentSimulator) handleBlocks(
	blocks []block, mocks runMocks, tags *metrics.TagSet, result *AgentTestCase, asst *message,
) ([]block, bool) {
	var toolResults []block
	hitTerminal := false
	for _, b := range blocks {
		switch b.kind {
		case blockText:
			result.Output = b.text
			asst.blocks = append(asst.blocks, b)
		case blockToolUse:
			asst.blocks = append(asst.blocks, b)
			out := a.dispatch(b.name, b.input, mocks)
			result.ToolCalls = append(result.ToolCalls, ToolCall{Name: b.name, Input: b.input, Output: out})
			pushSample(a.mi.vu, a.mi.metrics.toolCalls, tags.With("tool", b.name), 1)
			toolResults = append(toolResults, block{kind: blockToolResult, toolUseID: b.id, content: out})
			if a.terminalTool != "" && b.name == a.terminalTool {
				hitTerminal = true
			}
		}
	}
	return toolResults, hitTerminal
}

// dispatch resolves a tool call to a result string. Order: per-run mock override
// → the tool's own mock → a generic ack → an error for unknown tools.
func (a *AgentSimulator) dispatch(name string, input map[string]any, mocks runMocks) string {
	if v, ok := mocks[name]; ok && v != nil && !sobek.IsUndefined(v) {
		return a.resolveMock(v, input)
	}
	if t, ok := a.registry.get(name); ok {
		if t.mock != nil && !sobek.IsUndefined(t.mock) && !sobek.IsNull(t.mock) {
			return a.resolveMock(t.mock, input)
		}
		return "Tool executed"
	}
	return fmt.Sprintf("Error: tool %q is not available", name)
}

// resolveMock turns a mock value (callable or static) into a tool-result string.
func (a *AgentSimulator) resolveMock(v sobek.Value, input map[string]any) string {
	if fn, ok := sobek.AssertFunction(v); ok {
		res, err := fn(sobek.Undefined(), a.rt.ToValue(input))
		if err != nil {
			common.Throw(a.rt, fmt.Errorf("tool mock handler threw: %w", err))
		}
		return toResultString(res)
	}
	return toResultString(v)
}

func (a *AgentSimulator) parseRunMocks(v sobek.Value) runMocks {
	out := runMocks{}
	if v == nil || common.IsNullish(v) {
		return out
	}
	obj := v.ToObject(a.rt)
	for _, k := range obj.Keys() {
		out[k] = obj.Get(k)
	}
	return out
}

func (a *AgentSimulator) baseTags(userTags sobek.Value) *metrics.TagSet {
	state := a.mi.vu.State()
	tags := state.Tags.GetCurrentValues().Tags.With("agent", a.name).With("model", a.model)
	if userTags != nil && !common.IsNullish(userTags) {
		obj := userTags.ToObject(a.rt)
		for _, k := range obj.Keys() {
			tags = tags.With(k, obj.Get(k).String())
		}
	}
	return tags
}

func toResultString(v sobek.Value) string {
	if v == nil || sobek.IsUndefined(v) || sobek.IsNull(v) {
		return ""
	}
	exported := v.Export()
	if s, ok := exported.(string); ok {
		return s
	}
	if b, err := json.Marshal(exported); err == nil {
		return string(b)
	}
	return v.String()
}

func getString(obj *sobek.Object, key, def string) string {
	v := obj.Get(key)
	if v == nil || sobek.IsUndefined(v) || sobek.IsNull(v) {
		return def
	}
	return v.String()
}

func getInt(obj *sobek.Object, key string, def int) int {
	v := obj.Get(key)
	if v == nil || sobek.IsUndefined(v) || sobek.IsNull(v) {
		return def
	}
	return int(v.ToInteger())
}
