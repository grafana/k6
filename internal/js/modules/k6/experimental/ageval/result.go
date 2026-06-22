package ageval

import (
	"reflect"
	"strconv"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/metrics"
)

// ToolCall is one tool invocation recorded during a run, exposed to JS.
type ToolCall struct {
	Name   string         `js:"name"`
	Input  map[string]any `js:"input"`
	Output string         `js:"output"`
}

// RunUsage is the token usage of a run, exposed to JS.
type RunUsage struct {
	InputTokens  int64 `js:"inputTokens"`
	OutputTokens int64 `js:"outputTokens"`
}

// RunResult is the outcome of Agent.run(), exposed to JS. Exported fields become
// camelCase properties; exported methods become camelCase methods.
type RunResult struct {
	vu             modules.VU
	rt             *sobek.Runtime
	metrics        *agevalMetrics
	tags           *metrics.TagSet
	stepReportTool string

	Input     string     `js:"input"`
	Output    string     `js:"output"`
	ToolCalls []ToolCall `js:"toolCalls"`
	Usage     RunUsage   `js:"usage"`
	Steps     int        `js:"steps"`
	Duration  float64    `js:"duration"`
}

// CalledTool reports whether a tool with the given name was called at least once.
func (r *RunResult) CalledTool(name string) bool {
	for _, c := range r.ToolCalls {
		if c.Name == name {
			return true
		}
	}
	return false
}

// CallsOf returns every recorded call to the named tool, in order.
func (r *RunResult) CallsOf(name string) []ToolCall {
	out := []ToolCall{}
	for _, c := range r.ToolCalls {
		if c.Name == name {
			out = append(out, c)
		}
	}
	return out
}

// ToolSequence returns the ordered list of tool names called during the run.
func (r *RunResult) ToolSequence() []string {
	out := make([]string, 0, len(r.ToolCalls))
	for _, c := range r.ToolCalls {
		out = append(out, c.Name)
	}
	return out
}

// StepReports returns the calls the agent made to its step-reporting tool.
func (r *RunResult) StepReports() []ToolCall {
	return r.CallsOf(r.stepReportTool)
}

// FailedSteps returns the step reports whose `success` field is false.
func (r *RunResult) FailedSteps() []ToolCall {
	out := []ToolCall{}
	for _, c := range r.StepReports() {
		if success, ok := c.Input["success"].(bool); ok && !success {
			out = append(out, c)
		}
	}
	return out
}

type expectedCall struct {
	name string
	args map[string]any
}

// ExpectSequence checks the recorded tool calls against an expected sequence and
// emits the agent_tool_correctness metric (1 on match, 0 otherwise). It returns
// the boolean result so it can also be used inside check().
//
// expected is an array of `{ name, args? }`. opts is `{ mode, allowOtherCalls }`:
//   - mode "in-order" (default): expected is an in-order subsequence of the
//     actual calls. With allowOtherCalls=false, no tool outside the expected set
//     may be called.
//   - mode "exact": the actual sequence must equal expected one-to-one, in order.
func (r *RunResult) ExpectSequence(expected sobek.Value, opts sobek.Value) bool {
	exp := r.parseExpected(expected)
	mode, allowOther := "in-order", true
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		o := opts.ToObject(r.rt)
		if v := o.Get("mode"); v != nil && !sobek.IsUndefined(v) {
			mode = v.String()
		}
		if v := o.Get("allowOtherCalls"); v != nil && !sobek.IsUndefined(v) {
			allowOther = v.ToBoolean()
		}
	}

	ok := matchSequence(r.ToolCalls, exp, mode, allowOther)

	value := 0.0
	if ok {
		value = 1
	}
	pushSample(r.vu, r.metrics.toolCorrectness, r.tags, value)
	return ok
}

func (r *RunResult) parseExpected(expected sobek.Value) []expectedCall {
	out := []expectedCall{}
	if expected == nil || sobek.IsUndefined(expected) || sobek.IsNull(expected) {
		return out
	}
	arr := expected.ToObject(r.rt)
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
		obj := item.ToObject(r.rt)
		ec := expectedCall{}
		if v := obj.Get("name"); v != nil {
			ec.name = v.String()
		}
		if v := obj.Get("args"); v != nil && !sobek.IsUndefined(v) && !sobek.IsNull(v) {
			if m, ok := v.Export().(map[string]any); ok {
				ec.args = m
			}
		}
		out = append(out, ec)
	}
	return out
}

func matchSequence(actual []ToolCall, expected []expectedCall, mode string, allowOther bool) bool {
	if mode == "exact" {
		if len(actual) != len(expected) {
			return false
		}
		for i, exp := range expected {
			if actual[i].Name != exp.name || !argsMatch(actual[i].Input, exp.args) {
				return false
			}
		}
		return true
	}

	// in-order subsequence match.
	ai := 0
	for _, exp := range expected {
		found := false
		for ai < len(actual) {
			c := actual[ai]
			ai++
			if c.Name == exp.name && argsMatch(c.Input, exp.args) {
				found = true
				break
			}
			if !allowOther && !inExpectedSet(c.Name, expected) {
				return false
			}
		}
		if !found {
			return false
		}
	}
	if !allowOther {
		for ; ai < len(actual); ai++ {
			if !inExpectedSet(actual[ai].Name, expected) {
				return false
			}
		}
	}
	return true
}

func inExpectedSet(name string, expected []expectedCall) bool {
	for _, e := range expected {
		if e.name == name {
			return true
		}
	}
	return false
}

// argsMatch reports whether every key in want is present in got with an equal
// value (subset match). A nil want matches anything.
func argsMatch(got, want map[string]any) bool {
	for k, wv := range want {
		gv, ok := got[k]
		if !ok || !reflect.DeepEqual(normalizeNumber(gv), normalizeNumber(wv)) {
			return false
		}
	}
	return true
}

// normalizeNumber coerces integer-valued floats to float64 so JS numbers compare
// equal regardless of how they were produced.
func normalizeNumber(v any) any {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return v
	}
}
