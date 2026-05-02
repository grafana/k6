package common

import (
	"context"
	"errors"
	"testing"

	"github.com/chromedp/cdproto/cdp"
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

func (m *mockSession) ID() target.SessionID { return "mock-session-id" }
func (m *mockSession) TargetID() target.ID  { return "mock-target-id" }
func (m *mockSession) Done() <-chan struct{} { return m.doneCh }

func TestDialogAccept(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ms := newMockSession(ctx)

	d := newDialog(cdp.WithExecutor(ctx, ms), ms)
	require.NotNil(t, d)
	assert.False(t, d.handled)

	err := d.Accept()
	require.NoError(t, err)

	assert.True(t, d.handled)
	assert.Equal(t, "Page.handleJavaScriptDialog", ms.lastMethod)
}

func TestDialogDismiss(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ms := newMockSession(ctx)

	d := newDialog(cdp.WithExecutor(ctx, ms), ms)
	require.NotNil(t, d)
	assert.False(t, d.handled)

	err := d.Dismiss()
	require.NoError(t, err)

	assert.True(t, d.handled)
	assert.Equal(t, "Page.handleJavaScriptDialog", ms.lastMethod)
}

func TestDialogAcceptPropagatesError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ms := newMockSession(ctx)
	ms.err = errors.New("cdp error")

	d := newDialog(cdp.WithExecutor(ctx, ms), ms)
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

	d := newDialog(cdp.WithExecutor(ctx, ms), ms)
	err := d.Dismiss()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cdp error")
	assert.False(t, d.handled)
}
