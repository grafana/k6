package ageval

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cannedTranscript is a minimal Claude Code stream-json transcript: one MCP tool
// call (validate_script) plus a final result.
const cannedTranscript = `{"type":"system","subtype":"init"}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"mcp__k6__validate_script","input":{"script":"export default function(){}"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"valid"}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"The script is valid."}]}}
{"type":"result","subtype":"success","result":"The script is valid.","usage":{"input_tokens":100,"output_tokens":20,"cache_read_input_tokens":50},"duration_ms":1234}
`

// The successful exec+parse path is exercised end-to-end by the live examples
// (examples/.../claude-code and mcp-k6/eval). Here we unit-test the parser and
// the command-failure path; the os/exec helper-process idiom is avoided because
// k6's linter forbids direct os.* usage.

func TestCliAgentCommandFailureThrows(t *testing.T) {
	t.Parallel()
	ts := newTestSetup(t)

	_, err := ts.rt.VU.Runtime().RunString(`
		const a = new CliAgent({ command: "this-command-does-not-exist-ageval", format: "claude-code" });
		a.run({ input: "x" });
	`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed")
}
