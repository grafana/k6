package ageval

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdapterClaudeCode(t *testing.T) {
	t.Parallel()
	tr := adapterClaudeCode(cannedTranscript)
	assert.Equal(t, "The script is valid.", tr.output)
	require.Len(t, tr.toolCalls, 1)
	assert.Equal(t, "validate_script", tr.toolCalls[0].Name) // mcp__k6__validate_script normalized
	assert.Equal(t, "valid", tr.toolCalls[0].Output)
	assert.Equal(t, "export default function(){}", tr.toolCalls[0].Input["script"])
	assert.Equal(t, int64(150), tr.inTok) // 100 input + 50 cache_read
	assert.Equal(t, int64(20), tr.outTok)
}

func TestNormalizeMCPToolName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "validate_script", normalizeMCPToolName("mcp__k6__validate_script"))
	assert.Equal(t, "Read", normalizeMCPToolName("Read")) // non-MCP names unchanged
}

// codexFixture is a `codex exec --json` JSONL stream: a shell command, an MCP tool
// call, an agent message, and turn usage (matching codex-cli 0.140 output).
const codexFixture = `{"type":"thread.started","thread_id":"t-1"}
{"type":"turn.started"}
{"type":"item.started","item":{"id":"item_0","type":"command_execution","command":"ls *.go","status":"in_progress"}}
{"type":"item.completed","item":{"id":"item_0","type":"command_execution","command":"ls *.go","aggregated_output":"a.go\nb.go\n","exit_code":0,"status":"completed"}}
{"type":"item.completed","item":{"id":"item_1","type":"mcp_tool_call","tool":"validate_script","arguments":{"script":"x"},"result":"valid"}}
{"type":"item.completed","item":{"id":"item_2","type":"reasoning","text":"thinking..."}}
{"type":"item.completed","item":{"id":"item_3","type":"agent_message","text":"There are 2 .go files."}}
{"type":"turn.completed","usage":{"input_tokens":42698,"cached_input_tokens":28928,"output_tokens":510,"reasoning_output_tokens":408}}
`

func TestAdapterCodex(t *testing.T) {
	t.Parallel()
	tr := adapterCodex(codexFixture)
	assert.Equal(t, "There are 2 .go files.", tr.output)
	require.Len(t, tr.toolCalls, 2) // shell + mcp_tool_call; reasoning ignored
	assert.Equal(t, "shell", tr.toolCalls[0].Name)
	assert.Equal(t, "ls *.go", tr.toolCalls[0].Input["command"])
	assert.Equal(t, "a.go\nb.go\n", tr.toolCalls[0].Output)
	assert.Equal(t, "validate_script", tr.toolCalls[1].Name)
	assert.Equal(t, "x", tr.toolCalls[1].Input["script"])
	assert.Equal(t, "valid", tr.toolCalls[1].Output)
	assert.Equal(t, int64(42698), tr.inTok) // input_tokens already includes cached
	assert.Equal(t, int64(510), tr.outTok)  // output_tokens already includes reasoning
}

const a2aFixture = `data: {"jsonrpc":"2.0","result":{"kind":"status-update","status":{"state":"working"}}}
data: {"jsonrpc":"2.0","result":{"kind":"artifact-update","artifact":{"name":"step.toolCall","parts":[{"kind":"data","data":{"toolId":"t1","toolName":"get_documentation","inputs":{"slug":"using-k6/http"}}}]}}}
data: {"jsonrpc":"2.0","result":{"kind":"artifact-update","artifact":{"name":"step.complete","metadata":{"agent-traceability":{"usage":{"inputTokens":1200,"outputTokens":300}}}}}}
data: {"jsonrpc":"2.0","result":{"kind":"artifact-update","artifact":{"name":"step.toolResult","parts":[{"kind":"data","data":{"toolId":"t1","result":"docs..."}}]}}}
data: {"jsonrpc":"2.0","result":{"kind":"artifact-update","artifact":{"name":"step.message","parts":[{"kind":"text","text":"Here is your k6 script."}]}}}
data: {"jsonrpc":"2.0","result":{"kind":"status-update","status":{"state":"completed"}}}
`

func TestAdapterA2A(t *testing.T) {
	t.Parallel()
	tr := adapterA2A(a2aFixture)
	assert.Equal(t, "Here is your k6 script.", tr.output)
	require.Len(t, tr.toolCalls, 1)
	assert.Equal(t, "get_documentation", tr.toolCalls[0].Name)
	assert.Equal(t, "using-k6/http", tr.toolCalls[0].Input["slug"])
	assert.Equal(t, "docs...", tr.toolCalls[0].Output)
	assert.Equal(t, int64(1200), tr.inTok)
	assert.Equal(t, int64(300), tr.outTok)
}

func TestAdapterOpenAI(t *testing.T) {
	t.Parallel()
	body := `{"choices":[{"message":{"content":"done","tool_calls":[{"id":"c1","function":{"name":"get_weather","arguments":"{\"city\":\"Paris\"}"}}]}}],"usage":{"prompt_tokens":40,"completion_tokens":12}}`
	tr := adapterOpenAI(body)
	assert.Equal(t, "done", tr.output)
	require.Len(t, tr.toolCalls, 1)
	assert.Equal(t, "get_weather", tr.toolCalls[0].Name)
	assert.Equal(t, "Paris", tr.toolCalls[0].Input["city"])
	assert.Equal(t, int64(40), tr.inTok)
	assert.Equal(t, int64(12), tr.outTok)
}

func TestAdapterAnthropic(t *testing.T) {
	t.Parallel()
	body := `{"content":[{"type":"text","text":"all done"},{"type":"tool_use","name":"get_invoice","input":{"id":"INV-1"}}],"usage":{"input_tokens":80,"output_tokens":9}}`
	tr := adapterAnthropic(body)
	assert.Equal(t, "all done", tr.output)
	require.Len(t, tr.toolCalls, 1)
	assert.Equal(t, "get_invoice", tr.toolCalls[0].Name)
	assert.Equal(t, "INV-1", tr.toolCalls[0].Input["id"])
	assert.Equal(t, int64(80), tr.inTok)
	assert.Equal(t, int64(9), tr.outTok)
}

func TestAgentTestCaseWithFormatAdapter(t *testing.T) {
	t.Parallel()
	ts := newTestSetup(t)
	// new AgentTestCase({ format: 'a2a', raw: <sse> }) should parse via the adapter.
	require.NoError(t, ts.rt.VU.Runtime().Set("a2aRaw", a2aFixture))
	v, err := ts.rt.VU.Runtime().RunString(`
		const r = new AgentTestCase({ format: "a2a", raw: a2aRaw, input: "make a script", tags: { case: "fmt" } });
		[r.calledTool("get_documentation"), r.toolCalls.length, r.output, r.usage.inputTokens];
	`)
	require.NoError(t, err)
	rt := ts.rt.VU.Runtime()
	arr := v.ToObject(rt)
	assert.True(t, arr.Get("0").ToBoolean())
	assert.Equal(t, int64(1), arr.Get("1").ToInteger())
	assert.Equal(t, "Here is your k6 script.", arr.Get("2").String())
	assert.Equal(t, int64(1200), arr.Get("3").ToInteger())

	samples := drainSamples(ts.samples)
	require.Len(t, samples["agent_tool_calls"], 1)
	require.Len(t, samples["agent_tokens"], 2)
}

func TestUnknownFormatThrows(t *testing.T) {
	t.Parallel()
	ts := newTestSetup(t)
	_, err := ts.rt.VU.Runtime().RunString(`new AgentTestCase({ format: "nope", raw: "x" });`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("unknown format %q", "nope"))
}
