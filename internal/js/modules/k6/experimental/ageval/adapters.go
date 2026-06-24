package ageval

import (
	"encoding/json"
	"sort"
	"strings"
)

// trajectory is the normalized agent run an adapter produces from a raw payload.
// It is the single intermediate shape that every wire format maps into before
// becoming an AgentTestCase.
type trajectory struct {
	output    string
	toolCalls []ToolCall
	inTok     int64
	outTok    int64
}

// adapters maps a `format` name to a converter from a raw payload (an agent's
// stdout, an HTTP response body, etc.) into a normalized trajectory. New agent
// wire formats are added here once, rather than re-parsed in every test script.
// The `parse` callback on ExternalAgent remains the escape hatch for bespoke
// formats not worth a built-in adapter.
//
//nolint:gochecknoglobals // package-level registry, like the provider registry
var adapters = map[string]func(string) trajectory{
	"claude-code": adapterClaudeCode,
	"codex":       adapterCodex,
	"a2a":         adapterA2A,
	"openai":      adapterOpenAI,
	"anthropic":   adapterAnthropic,
}

func lookupAdapter(name string) (func(string) trajectory, bool) {
	fn, ok := adapters[name]
	return fn, ok
}

// adapterNames returns the supported format names, sorted, for error messages.
func adapterNames() []string {
	out := make([]string, 0, len(adapters))
	for name := range adapters {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// --- claude-code: `claude --output-format stream-json` (JSONL) ---

type ccBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
}

type ccEvent struct {
	Type    string `json:"type"`
	Message struct {
		Content []ccBlock `json:"content"`
	} `json:"message"`
	Result *string `json:"result"`
	Usage  *struct {
		InputTokens   int64 `json:"input_tokens"`
		OutputTokens  int64 `json:"output_tokens"`
		CacheRead     int64 `json:"cache_read_input_tokens"`
		CacheCreation int64 `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

func adapterClaudeCode(jsonl string) trajectory {
	t := trajectory{toolCalls: []ToolCall{}}
	byID := map[string]int{}
	for line := range strings.SplitSeq(jsonl, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e ccEvent
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		switch e.Type {
		case "assistant":
			ccHandleAssistant(e.Message.Content, &t.toolCalls, byID, &t.output)
		case "user":
			ccHandleToolResults(e.Message.Content, t.toolCalls, byID)
		case "result":
			if e.Result != nil {
				t.output = *e.Result
			}
			if e.Usage != nil {
				t.inTok = e.Usage.InputTokens + e.Usage.CacheRead + e.Usage.CacheCreation
				t.outTok = e.Usage.OutputTokens
			}
		}
	}
	return t
}

func ccHandleAssistant(content []ccBlock, toolCalls *[]ToolCall, byID map[string]int, output *string) {
	for _, b := range content {
		switch b.Type {
		case blockToolUse:
			*toolCalls = append(*toolCalls, ToolCall{Name: normalizeMCPToolName(b.Name), Input: rawToMap(b.Input)})
			byID[b.ID] = len(*toolCalls) - 1
		case blockText:
			if b.Text != "" {
				*output = b.Text
			}
		}
	}
}

func ccHandleToolResults(content []ccBlock, toolCalls []ToolCall, byID map[string]int) {
	for _, b := range content {
		if b.Type != blockToolResult {
			continue
		}
		if idx, ok := byID[b.ToolUseID]; ok {
			toolCalls[idx].Output = stringifyJSONContent(b.Content)
		}
	}
}

// --- codex: `codex exec --json` (JSONL thread events) ---

type codexItem struct {
	Type string `json:"type"`
	Text string `json:"text"` // agent_message
	// command_execution
	Command          string `json:"command"`
	AggregatedOutput string `json:"aggregated_output"`
	// mcp_tool_call
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
	Result    json.RawMessage `json:"result"`
}

type codexEvent struct {
	Type  string     `json:"type"`
	Item  *codexItem `json:"item"`
	Usage *struct {
		InputTokens  int64 `json:"input_tokens"` // already includes cached_input_tokens
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage"`
}

// adapterCodex parses Codex CLI's `codex exec --json` JSONL stream. Each line is a
// thread event; `item.completed` carries the finished items (a shell command, an
// MCP tool call, the agent's message), and `turn.completed` carries token usage.
func adapterCodex(jsonl string) trajectory {
	t := trajectory{toolCalls: []ToolCall{}}
	for line := range strings.SplitSeq(jsonl, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e codexEvent
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		switch {
		case e.Type == "item.completed" && e.Item != nil:
			codexHandleItem(e.Item, &t)
		case e.Type == "turn.completed" && e.Usage != nil:
			t.inTok += e.Usage.InputTokens
			t.outTok += e.Usage.OutputTokens
		}
	}
	return t
}

func codexHandleItem(item *codexItem, t *trajectory) {
	switch item.Type {
	case "agent_message":
		if item.Text != "" {
			t.output = item.Text // keep the last message as the final answer
		}
	case "reasoning":
		// internal chain-of-thought, not a tool call or answer — ignore.
	case "command_execution":
		t.toolCalls = append(t.toolCalls, ToolCall{
			Name:   "shell",
			Input:  map[string]any{"command": item.Command},
			Output: item.AggregatedOutput,
		})
	case "mcp_tool_call":
		t.toolCalls = append(t.toolCalls, ToolCall{
			Name:   item.Tool,
			Input:  rawToMap(item.Arguments),
			Output: stringifyJSONContent(item.Result),
		})
	default:
		// file_change, web_search, todo_list, etc. — record under the item type so
		// they're still visible to trajectory assertions and metrics.
		t.toolCalls = append(t.toolCalls, ToolCall{Name: item.Type, Input: map[string]any{}})
	}
}

// --- a2a: Grafana Assistant A2A protocol (JSON-RPC over SSE) ---

type a2aPart struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
	Data struct {
		ToolID   string          `json:"toolId"`
		ToolName string          `json:"toolName"`
		Inputs   json.RawMessage `json:"inputs"`
		Result   json.RawMessage `json:"result"`
	} `json:"data"`
}

type a2aArtifact struct {
	Name     string    `json:"name"`
	Parts    []a2aPart `json:"parts"`
	Metadata struct {
		Trace struct {
			Usage struct {
				InputTokens  int64 `json:"inputTokens"`
				OutputTokens int64 `json:"outputTokens"`
			} `json:"usage"`
		} `json:"agent-traceability"`
	} `json:"metadata"`
}

type a2aEvent struct {
	Result *struct {
		Kind     string       `json:"kind"`
		Artifact *a2aArtifact `json:"artifact"`
		Status   *struct {
			Message *struct {
				Parts []a2aPart `json:"parts"`
			} `json:"message"`
		} `json:"status"`
	} `json:"result"`
}

func adapterA2A(sse string) trajectory {
	t := trajectory{toolCalls: []ToolCall{}}
	byID := map[string]int{}
	for line := range strings.SplitSeq(sse, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var e a2aEvent
		if json.Unmarshal([]byte(strings.TrimSpace(line[len("data:"):])), &e) != nil || e.Result == nil {
			continue
		}
		switch {
		case e.Result.Kind == "artifact-update" && e.Result.Artifact != nil:
			a2aHandleArtifact(e.Result.Artifact, &t, byID)
		case e.Result.Kind == "status-update" && e.Result.Status != nil && e.Result.Status.Message != nil:
			for _, p := range e.Result.Status.Message.Parts {
				if p.Kind == "text" && p.Text != "" {
					t.output = p.Text
				}
			}
		}
	}
	return t
}

func a2aHandleArtifact(art *a2aArtifact, t *trajectory, byID map[string]int) {
	switch art.Name {
	case "step.toolCall":
		for _, p := range art.Parts {
			if p.Kind == "data" && p.Data.ToolName != "" {
				t.toolCalls = append(t.toolCalls, ToolCall{Name: p.Data.ToolName, Input: rawToMap(p.Data.Inputs)})
				byID[p.Data.ToolID] = len(t.toolCalls) - 1
			}
		}
	case "step.message":
		for _, p := range art.Parts {
			if p.Kind == "text" && p.Text != "" {
				t.output += p.Text
			}
		}
	case "step.toolResult":
		for _, p := range art.Parts {
			if idx, ok := byID[p.Data.ToolID]; ok {
				t.toolCalls[idx].Output = stringifyJSONContent(p.Data.Result)
			}
		}
	case "step.complete":
		t.inTok += art.Metadata.Trace.Usage.InputTokens
		t.outTok += art.Metadata.Trace.Usage.OutputTokens
	}
}

// --- openai: a Chat Completions response object ---

type openAIResp struct {
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
}

func adapterOpenAI(body string) trajectory {
	t := trajectory{toolCalls: []ToolCall{}, inTok: 0, outTok: 0}
	var r openAIResp
	if json.Unmarshal([]byte(body), &r) != nil {
		return t
	}
	t.inTok = r.Usage.PromptTokens
	t.outTok = r.Usage.CompletionTokens
	if len(r.Choices) == 0 {
		return t
	}
	msg := r.Choices[0].Message
	t.output = msg.Content
	for _, tc := range msg.ToolCalls {
		t.toolCalls = append(t.toolCalls, ToolCall{Name: tc.Function.Name, Input: jsonStrToMap(tc.Function.Arguments)})
	}
	return t
}

// --- anthropic: a Messages API response object ---

type anthropicResp struct {
	Content []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content"`
	Usage struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage"`
}

func adapterAnthropic(body string) trajectory {
	t := trajectory{toolCalls: []ToolCall{}}
	var r anthropicResp
	if json.Unmarshal([]byte(body), &r) != nil {
		return t
	}
	t.inTok = r.Usage.InputTokens
	t.outTok = r.Usage.OutputTokens
	for _, b := range r.Content {
		switch b.Type {
		case blockText:
			if b.Text != "" {
				t.output = b.Text
			}
		case blockToolUse:
			t.toolCalls = append(t.toolCalls, ToolCall{Name: b.Name, Input: rawToMap(b.Input)})
		}
	}
	return t
}

// --- shared helpers ---

// normalizeMCPToolName strips the "mcp__<server>__" prefix Claude Code adds to
// MCP tool calls, so assertions use the server's own tool names.
func normalizeMCPToolName(name string) string {
	parts := strings.Split(name, "__")
	if len(parts) >= 3 && parts[0] == "mcp" {
		return strings.Join(parts[2:], "__")
	}
	return name
}

// stringifyJSONContent renders a tool result content (a string, an array of
// `{text}` blocks, or any JSON) as a plain string.
func stringifyJSONContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		parts := make([]string, 0, len(blocks))
		for _, b := range blocks {
			parts = append(parts, b.Text)
		}
		return strings.Join(parts, "\n")
	}
	return string(raw)
}

// rawToMap unmarshals a JSON object into map[string]any (empty on failure).
func rawToMap(raw json.RawMessage) map[string]any {
	m := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	return m
}

// jsonStrToMap unmarshals a JSON-string-encoded object into map[string]any.
func jsonStrToMap(s string) map[string]any {
	m := map[string]any{}
	if s != "" {
		_ = json.Unmarshal([]byte(s), &m)
	}
	return m
}
