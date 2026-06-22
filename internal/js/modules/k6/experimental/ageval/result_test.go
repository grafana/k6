package ageval

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const stepIndexKey = "step_index"

func calls(names ...string) []ToolCall {
	out := make([]ToolCall, 0, len(names))
	for _, n := range names {
		out = append(out, ToolCall{Name: n, Input: map[string]any{}})
	}
	return out
}

func TestMatchSequenceInOrder(t *testing.T) {
	t.Parallel()
	actual := calls("html", "fillinput", "screenshot", "clickelement", defaultStepReportTool)
	exp := []expectedCall{{name: "fillinput"}, {name: "clickelement"}}

	assert.True(t, matchSequence(actual, exp, "in-order", true), "subsequence with others allowed")
	assert.False(t, matchSequence(actual, exp, "in-order", false), "foreign calls present -> fail when not allowed")
	assert.True(t, matchSequence(calls("fillinput", "clickelement"), exp, "in-order", false))
}

func TestMatchSequenceExact(t *testing.T) {
	t.Parallel()
	exp := []expectedCall{{name: "fillinput"}, {name: "clickelement"}}
	assert.True(t, matchSequence(calls("fillinput", "clickelement"), exp, "exact", false))
	assert.False(t, matchSequence(calls("fillinput", "clickelement", defaultStepReportTool), exp, "exact", false))
	assert.False(t, matchSequence(calls("clickelement", "fillinput"), exp, "exact", false))
}

func TestMatchSequenceArgs(t *testing.T) {
	t.Parallel()
	actual := []ToolCall{{Name: "fillinput", Input: map[string]any{"selector": "#s", "text": "kubernetes"}}}
	assert.True(t, matchSequence(actual, []expectedCall{{name: "fillinput", args: map[string]any{"text": "kubernetes"}}}, "exact", false))
	assert.False(t, matchSequence(actual, []expectedCall{{name: "fillinput", args: map[string]any{"text": "other"}}}, "exact", false))
}

func TestArgsMatchNumberNormalization(t *testing.T) {
	t.Parallel()
	got := map[string]any{stepIndexKey: float64(1), "ok": true}
	assert.True(t, argsMatch(got, map[string]any{stepIndexKey: 1}))
	assert.True(t, argsMatch(got, map[string]any{stepIndexKey: int64(1)}))
	assert.False(t, argsMatch(got, map[string]any{stepIndexKey: 2}))
	assert.True(t, argsMatch(got, nil))
}

func TestRunResultHelpers(t *testing.T) {
	t.Parallel()
	r := &RunResult{
		stepReportTool: defaultStepReportTool,
		ToolCalls: []ToolCall{
			{Name: "html", Input: map[string]any{}},
			{Name: defaultStepReportTool, Input: map[string]any{stepIndexKey: float64(1), "success": true}},
			{Name: defaultStepReportTool, Input: map[string]any{stepIndexKey: float64(2), "success": false}},
		},
	}
	assert.True(t, r.CalledTool("html"))
	assert.False(t, r.CalledTool("nope"))
	assert.Equal(t, []string{"html", defaultStepReportTool, defaultStepReportTool}, r.ToolSequence())
	assert.Len(t, r.StepReports(), 2)
	assert.Len(t, r.FailedSteps(), 1)
	assert.Len(t, r.CallsOf(defaultStepReportTool), 2)
}
