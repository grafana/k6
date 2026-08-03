package ageval

import "context"

// role identifies the author of a conversation message.
const (
	roleUser      = "user"
	roleAssistant = "assistant"
)

// block kinds used in the neutral conversation representation. Providers
// translate these to and from their own wire formats.
const (
	blockText       = "text"
	blockToolUse    = "tool_use"
	blockToolResult = "tool_result"
)

// block is a single piece of a message: model text, a tool call the model
// requested, or a tool result we feed back. It is intentionally provider
// neutral so the agent loop never depends on a specific vendor's wire types.
type block struct {
	kind string

	// text block
	text string

	// tool_use block
	id    string
	name  string
	input map[string]any

	// tool_result block
	toolUseID string
	content   string
}

// message is one turn in the conversation.
type message struct {
	role   string
	blocks []block
}

// toolSchema describes a tool the model may call. inputSchema is a JSON Schema
// object as provided by the test author.
type toolSchema struct {
	name        string
	description string
	inputSchema map[string]any
}

// usage reports token consumption for a single model call.
type usage struct {
	inputTokens  int64
	outputTokens int64
}

// conversation is the full input to a single model call.
type conversation struct {
	model     string
	apiKey    string
	baseURL   string
	system    string
	maxTokens int
	messages  []message
	tools     []toolSchema
}

// reply is the normalized result of a single model call.
type reply struct {
	blocks     []block
	stopReason string
	usage      usage
}

// modelInfo holds the per-model data the module needs: the wire identifier and
// the pricing used to compute the agent_cost_usd metric.
type modelInfo struct {
	wireModel     string
	inUSDPerMTok  float64
	outUSDPerMTok float64
	maxOutput     int
}

// provider is an LLM backend. Only Anthropic is registered today; the interface
// keeps the agent loop vendor neutral so a second provider can be added without
// touching the loop.
type provider interface {
	// name returns the provider identifier used in the JS `provider` field.
	name() string
	// model validates and returns the info for a model name.
	model(name string) (modelInfo, bool)
	// modelNames returns the supported model names, for error messages.
	modelNames() []string
	// createMessage performs a single (blocking) model call.
	createMessage(ctx context.Context, conv conversation) (reply, error)
}

// providers is the registry of available providers, keyed by name.
//
//nolint:gochecknoglobals // package-level registry, mirrors how k6 modules expose constants
var providers = map[string]provider{
	"anthropic": newAnthropicProvider(),
}

// lookupProvider returns the registered provider for the given name.
func lookupProvider(name string) (provider, bool) {
	p, ok := providers[name]
	return p, ok
}
