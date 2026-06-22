package ageval

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnthropicCreateMessageToolUse(t *testing.T) {
	t.Parallel()
	srv := cannedServer(t, toolUseResponse)
	p := newAnthropicProvider()

	rep, err := p.createMessage(context.Background(), conversation{
		model:    modelSonnet45,
		apiKey:   "test-key",
		baseURL:  srv.URL,
		system:   "be helpful",
		messages: []message{{role: roleUser, blocks: []block{{kind: blockText, text: "hi"}}}},
		tools:    []toolSchema{{name: "echo", description: "echo", inputSchema: map[string]any{"type": "object"}}},
	})
	require.NoError(t, err)
	assert.Equal(t, "tool_use", rep.stopReason)
	assert.Equal(t, int64(10), rep.usage.inputTokens)
	assert.Equal(t, int64(5), rep.usage.outputTokens)

	require.Len(t, rep.blocks, 2)
	assert.Equal(t, blockText, rep.blocks[0].kind)
	assert.Equal(t, blockToolUse, rep.blocks[1].kind)
	assert.Equal(t, "echo", rep.blocks[1].name)
	assert.Equal(t, "hi", rep.blocks[1].input["msg"])
}

func TestAnthropicCreateMessageError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"bad key"}}`))
	}))
	t.Cleanup(srv.Close)

	p := newAnthropicProvider()
	_, err := p.createMessage(context.Background(), conversation{
		model:    modelSonnet45,
		apiKey:   "k",
		baseURL:  srv.URL,
		messages: []message{{role: roleUser, blocks: []block{{kind: blockText, text: "x"}}}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad key")
}

func TestAnthropicModelRegistry(t *testing.T) {
	t.Parallel()
	p := newAnthropicProvider()

	info, ok := p.model(modelSonnet45)
	require.True(t, ok)
	assert.Equal(t, 3.0, info.inUSDPerMTok)
	assert.Equal(t, 15.0, info.outUSDPerMTok)

	_, ok = p.model("gpt-4")
	assert.False(t, ok)
	assert.NotEmpty(t, p.modelNames())
}
