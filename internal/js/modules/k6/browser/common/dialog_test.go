package common

import (
	"context"
	"errors"
	"testing"

	"github.com/chromedp/cdproto/cdp"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSession struct {
	BaseEventEmitter
	lastMethod string
	lastParams any
	err        error
	doneCh     chan struct{}
}

func newMockSession(ctx context.Context) *mockSession {
	ms := &mockSession{
		BaseEventEmitter: NewBaseEventEmitter(ctx),
		doneCh:           make(chan struct{}),
	}
	return ms
}

func (m *mockSession) Execute(_ context.Context, method string, params, _ any) error {
	m.lastMethod = method
	m.lastParams = params
	return m.err
}

func (m *mockSession) ExecuteWithoutExpectationOnReply(_ context.Context, method string, params, _ any) error {
	m.lastMethod = method
	m.lastParams = params
	return m.err
}

func (m *mockSession) ID() target.SessionID  { return "mock-session-id" }
func (m *mockSession) TargetID() target.ID   { return "mock-target-id" }
func (m *mockSession) Done() <-chan struct{} { return m.doneCh }

func newTestDialog(ctx context.Context, ms *mockSession, opts ...func(*cdppage.EventJavascriptDialogOpening)) *Dialog {
	event := &cdppage.EventJavascriptDialogOpening{
		Type:          cdppage.DialogTypeAlert,
		Message:       "test message",
		DefaultPrompt: "default",
	}
	for _, o := range opts {
		o(event)
	}
	return newDialog(cdp.WithExecutor(ctx, ms), ms, event)
}

func TestDialogType(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ms := newMockSession(ctx)
	d := newTestDialog(ctx, ms, func(e *cdppage.EventJavascriptDialogOpening) {
		e.Type = cdppage.DialogTypeConfirm
	})
	assert.Equal(t, "confirm", d.Type())
}

func TestDialogMessage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ms := newMockSession(ctx)
	d := newTestDialog(ctx, ms, func(e *cdppage.EventJavascriptDialogOpening) {
		e.Message = "are you sure?"
	})
	assert.Equal(t, "are you sure?", d.Message())
}

func TestDialogDefaultValue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ms := newMockSession(ctx)
	d := newTestDialog(ctx, ms, func(e *cdppage.EventJavascriptDialogOpening) {
		e.DefaultPrompt = "hello"
	})
	assert.Equal(t, "hello", d.DefaultValue())
}

func TestDialogAccept(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ms := newMockSession(ctx)
	d := newTestDialog(ctx, ms)
	require.NotNil(t, d)
	assert.False(t, d.handled)

	err := d.Accept()
	require.NoError(t, err)

	assert.True(t, d.handled)
	assert.Equal(t, "Page.handleJavaScriptDialog", ms.lastMethod)
	params, ok := ms.lastParams.(*cdppage.HandleJavaScriptDialogParams)
	require.True(t, ok)
	assert.True(t, params.Accept)
	assert.Empty(t, params.PromptText)
}

func TestDialogAcceptWithPromptText(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ms := newMockSession(ctx)
	d := newTestDialog(ctx, ms)

	err := d.Accept("my answer")
	require.NoError(t, err)

	params, ok := ms.lastParams.(*cdppage.HandleJavaScriptDialogParams)
	require.True(t, ok)
	assert.True(t, params.Accept)
	assert.Equal(t, "my answer", params.PromptText)
}

func TestDialogDismiss(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ms := newMockSession(ctx)
	d := newTestDialog(ctx, ms)
	require.NotNil(t, d)
	assert.False(t, d.handled)

	err := d.Dismiss()
	require.NoError(t, err)

	assert.True(t, d.handled)
	assert.Equal(t, "Page.handleJavaScriptDialog", ms.lastMethod)
	params, ok := ms.lastParams.(*cdppage.HandleJavaScriptDialogParams)
	require.True(t, ok)
	assert.False(t, params.Accept)
}

func TestDialogAcceptIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ms := newMockSession(ctx)
	d := newTestDialog(ctx, ms)

	require.NoError(t, d.Accept())
	ms.lastMethod = ""
	require.NoError(t, d.Accept())
	assert.Empty(t, ms.lastMethod)
}

func TestDialogDismissIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ms := newMockSession(ctx)
	d := newTestDialog(ctx, ms)

	require.NoError(t, d.Dismiss())
	ms.lastMethod = ""
	require.NoError(t, d.Dismiss())
	assert.Empty(t, ms.lastMethod)
}

func TestDialogAcceptPropagatesError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ms := newMockSession(ctx)
	ms.err = errors.New("cdp error")

	d := newTestDialog(ctx, ms)
	err := d.Accept()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cdp error")
	assert.False(t, d.handled)
}

func TestDialogDismissPropagatesError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ms := newMockSession(ctx)
	ms.err = errors.New("cdp error")

	d := newTestDialog(ctx, ms)
	err := d.Dismiss()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cdp error")
	assert.False(t, d.handled)
}
