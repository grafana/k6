package ageval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// anthropicVersion is the required API version header value.
const anthropicVersion = "2023-06-01"

// defaultBaseURL is the Anthropic API host. It is overridable per call so tests
// can point at an httptest server without any network access.
const defaultBaseURL = "https://api.anthropic.com"

// modelSonnet45 is the legacy-but-active model used by the ABT eval suite.
const modelSonnet45 = "claude-sonnet-4-5"

// anthropicProvider implements provider over the raw Messages API. We use a
// minimal net/http client rather than the official SDK to keep k6's dependency
// surface unchanged; the wire shape used here is small and stable.
type anthropicProvider struct {
	client  *http.Client
	baseURL string
	models  map[string]modelInfo
}

func newAnthropicProvider() *anthropicProvider {
	return &anthropicProvider{
		client:  &http.Client{Timeout: 120 * time.Second},
		baseURL: defaultBaseURL,
		// Pricing in USD per million tokens. Model IDs are pinned (no
		// "-latest") for reproducible evals. Sourced from the claude-api
		// reference, 2026-06.
		models: map[string]modelInfo{
			"claude-opus-4-8":   {wireModel: "claude-opus-4-8", inUSDPerMTok: 5, outUSDPerMTok: 25, maxOutput: 32000},
			"claude-opus-4-7":   {wireModel: "claude-opus-4-7", inUSDPerMTok: 5, outUSDPerMTok: 25, maxOutput: 32000},
			"claude-sonnet-4-6": {wireModel: "claude-sonnet-4-6", inUSDPerMTok: 3, outUSDPerMTok: 15, maxOutput: 32000},
			modelSonnet45:       {wireModel: modelSonnet45, inUSDPerMTok: 3, outUSDPerMTok: 15, maxOutput: 8192},
			"claude-haiku-4-5":  {wireModel: "claude-haiku-4-5", inUSDPerMTok: 1, outUSDPerMTok: 5, maxOutput: 8192},
			"claude-fable-5":    {wireModel: "claude-fable-5", inUSDPerMTok: 10, outUSDPerMTok: 50, maxOutput: 32000},
		},
	}
}

func (p *anthropicProvider) name() string { return "anthropic" }

func (p *anthropicProvider) model(name string) (modelInfo, bool) {
	m, ok := p.models[name]
	return m, ok
}

// modelNames returns the supported model names for error messages.
func (p *anthropicProvider) modelNames() []string {
	out := make([]string, 0, len(p.models))
	for name := range p.models {
		out = append(out, name)
	}
	return out
}

// --- wire types (Anthropic Messages API) ---

type apiBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	// tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
	// tool_result
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

type apiMessage struct {
	Role    string     `json:"role"`
	Content []apiBlock `json:"content"`
}

type apiTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type apiRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system,omitempty"`
	Messages  []apiMessage `json:"messages"`
	Tools     []apiTool    `json:"tools,omitempty"`
}

type apiResponse struct {
	Content    []apiBlock `json:"content"`
	StopReason string     `json:"stop_reason"`
	Usage      struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage"`
}

type apiError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// createMessage performs a single blocking Messages API call and normalizes the
// response into a reply.
func (p *anthropicProvider) createMessage(ctx context.Context, conv conversation) (reply, error) {
	info, ok := p.model(conv.model)
	if !ok {
		return reply{}, fmt.Errorf("unsupported model %q", conv.model)
	}
	maxTokens := conv.maxTokens
	if maxTokens <= 0 || maxTokens > info.maxOutput {
		maxTokens = info.maxOutput
	}

	body := apiRequest{
		Model:     info.wireModel,
		MaxTokens: maxTokens,
		System:    conv.system,
		Messages:  toAPIMessages(conv.messages),
		Tools:     toAPITools(conv.tools),
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return reply{}, err
	}

	baseURL := conv.baseURL
	if baseURL == "" {
		baseURL = p.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return reply{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", conv.apiKey)
	req.Header.Set("Anthropic-Version", anthropicVersion)

	// The request URL is built from operator-supplied configuration (the model
	// provider endpoint, defaulting to the official Anthropic API; baseURL is
	// only overridable by the test author, who already controls the whole
	// script), not from untrusted external input, so this is not an SSRF risk.
	resp, err := p.client.Do(req) //nolint:gosec // G704: baseURL is operator config, not attacker-controlled
	if err != nil {
		return reply{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return reply{}, err
	}
	if resp.StatusCode != http.StatusOK {
		var ae apiError
		if json.Unmarshal(respBody, &ae) == nil && ae.Error.Message != "" {
			return reply{}, fmt.Errorf("anthropic API error (%d): %s", resp.StatusCode, ae.Error.Message)
		}
		return reply{}, fmt.Errorf("anthropic API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var ar apiResponse
	if err := json.Unmarshal(respBody, &ar); err != nil {
		return reply{}, fmt.Errorf("decoding anthropic response: %w", err)
	}

	return fromAPIResponse(ar), nil
}

func toAPIMessages(msgs []message) []apiMessage {
	out := make([]apiMessage, 0, len(msgs))
	for _, m := range msgs {
		am := apiMessage{Role: m.role, Content: make([]apiBlock, 0, len(m.blocks))}
		for _, b := range m.blocks {
			switch b.kind {
			case blockText:
				am.Content = append(am.Content, apiBlock{Type: blockText, Text: b.text})
			case blockToolUse:
				input := b.input
				if input == nil {
					input = map[string]any{}
				}
				rawInput, err := json.Marshal(input)
				if err != nil {
					rawInput = []byte("{}")
				}
				am.Content = append(am.Content, apiBlock{
					Type: blockToolUse, ID: b.id, Name: b.name, Input: rawInput,
				})
			case blockToolResult:
				am.Content = append(am.Content, apiBlock{
					Type: blockToolResult, ToolUseID: b.toolUseID, Content: b.content,
				})
			}
		}
		out = append(out, am)
	}
	return out
}

func toAPITools(tools []toolSchema) []apiTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]apiTool, 0, len(tools))
	for _, t := range tools {
		schema := t.inputSchema
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, apiTool{Name: t.name, Description: t.description, InputSchema: schema})
	}
	return out
}

func fromAPIResponse(ar apiResponse) reply {
	r := reply{
		stopReason: ar.StopReason,
		usage:      usage{inputTokens: ar.Usage.InputTokens, outputTokens: ar.Usage.OutputTokens},
	}
	for _, b := range ar.Content {
		switch b.Type {
		case blockText:
			r.blocks = append(r.blocks, block{kind: blockText, text: b.Text})
		case blockToolUse:
			input := map[string]any{}
			if len(b.Input) > 0 {
				_ = json.Unmarshal(b.Input, &input)
			}
			r.blocks = append(r.blocks, block{kind: blockToolUse, id: b.ID, name: b.Name, input: input})
		}
	}
	return r
}
