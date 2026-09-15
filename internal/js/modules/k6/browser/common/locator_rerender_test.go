package common

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

func TestLocatorDetachedPointerReturnsForResolution(t *testing.T) {
	for _, locator := range []bool{false, true} {
		for _, detachAt := range []int{1, 3} {
			t.Run(fmt.Sprintf("locator=%t/detachAt=%d", locator, detachAt), func(t *testing.T) {
				calls := 0
				opts := NewElementHandleBasePointerOptions(time.Second)
				opts.retry = locator
				_, err := retryPointerAction(context.Background(), func(context.Context, *ScrollIntoViewOptions) (any, error) {
					calls++
					if calls < detachAt {
						return nil, ErrElementNotVisible
					}
					return nil, fmt.Errorf("detached: %w", ErrElementNotAttachedToDOM)
				}, opts)
				require.ErrorIs(t, err, ErrElementNotAttachedToDOM)
				if locator {
					require.Equal(t, detachAt, calls)
				} else {
					require.Equal(t, 5, calls)
				}
			})
		}
	}
}

func TestLocatorDetachedRetryCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	retry, err := shouldRetry(ctx, ErrElementNotAttachedToDOM)
	require.False(t, retry)
	require.True(t, errors.Is(err, context.Canceled))
}

func TestLocatorDetachedSelectorCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// A canceled action must stop before touching frame/document state.
	var frame Frame
	_, err := frame.waitForWithContext(ctx, "#removed", NewFrameWaitForSelectorOptions(30*time.Second), 20)
	require.ErrorIs(t, err, context.Canceled)
}

func TestLocatorDetachedMissingDocumentCancellation(t *testing.T) {
	frame := &Frame{ctx: context.Background(), log: log.NewNullLogger(), executionContexts: make(map[executionWorld]frameExecutionContext)}
	ctx, cancel := context.WithCancel(frame.ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := frame.documentWithContext(ctx); done <- err }()
	select {
	case err := <-done:
		t.Fatalf("document wait returned before cancellation: %v", err)
	case <-time.After(75 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("missing execution context ignored action cancellation")
	}
	require.NoError(t, frame.ctx.Err())
}

func TestLocatorDetachedDocumentEvaluationCancellation(t *testing.T) {
	frame := &Frame{ctx: context.Background(), log: log.NewNullLogger(), executionContexts: make(map[executionWorld]frameExecutionContext)}
	entered := make(chan struct{})
	frame.executionContexts[mainWorld] = &executionContextTestStub{evalFn: func(ctx context.Context, _ evalOptions, _ string, _ ...any) (any, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	ctx, cancel := context.WithCancel(frame.ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := frame.documentWithContext(ctx); done <- err }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("document evaluation did not start")
	}
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("document evaluation ignored action cancellation")
	}
	require.NoError(t, frame.ctx.Err())
}
