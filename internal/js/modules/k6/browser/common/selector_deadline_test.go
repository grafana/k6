package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

func TestSelectorWaitDeadline(t *testing.T) {
	t.Parallel()
	for _, retry := range []bool{false, true} {
		t.Run(fmt.Sprintf("retry=%t", retry), func(t *testing.T) {
			t.Parallel()
			parent, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			frame := &Frame{ctx: parent, log: log.NewNullLogger(), executionContexts: make(map[executionWorld]frameExecutionContext)}
			calls := 0
			var deadlines []time.Time
			if retry {
				frame.executionContexts[mainWorld] = &executionContextTestStub{evalFn: func(ctx context.Context, _ evalOptions, _ string, _ ...any) (any, error) {
					calls++
					deadline, ok := ctx.Deadline()
					require.True(t, ok)
					deadlines = append(deadlines, deadline)
					if calls == 1 {
						time.Sleep(25 * time.Millisecond)
						return nil, fmt.Errorf("Cannot find context with specified id")
					}
					<-ctx.Done()
					return nil, ctx.Err()
				}}
			}
			start := time.Now()
			_, err := frame.waitForWithContext(parent, "canvas", NewFrameWaitForSelectorOptions(40*time.Millisecond), 20)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.Less(t, time.Since(start), 200*time.Millisecond)
			require.NoError(t, parent.Err())
			if retry {
				require.Equal(t, 2, calls)
				require.Equal(t, deadlines[0], deadlines[1], "retry must inherit the same absolute deadline")
			}
		})
	}
}

func TestSelectorWaitZeroTimeoutUsesParent(t *testing.T) {
	t.Parallel()
	parent, cancel := context.WithTimeout(t.Context(), 80*time.Millisecond)
	defer cancel()
	frame := &Frame{ctx: parent, log: log.NewNullLogger(), executionContexts: make(map[executionWorld]frameExecutionContext)}
	start := time.Now()
	_, err := frame.waitForWithContext(parent, "canvas", NewFrameWaitForSelectorOptions(0), 20)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.GreaterOrEqual(t, time.Since(start), 60*time.Millisecond)
}

type selectorDeadlineSession struct{ session }

func (*selectorDeadlineSession) Execute(ctx context.Context, _ string, _, _ any) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestSelectorLazyInjectionUsesOperationContext(t *testing.T) {
	t.Parallel()
	parent, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	operation, stop := context.WithTimeout(parent, 40*time.Millisecond)
	defer stop()
	handle := &ElementHandle{BaseJSHandle: BaseJSHandle{ctx: parent, execCtx: &ExecutionContext{ctx: parent, session: &selectorDeadlineSession{}, logger: log.NewNullLogger()}}}
	start := time.Now()
	_, err := handle.evalWithScript(operation, evalOptions{}, "() => true")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), 200*time.Millisecond)
	require.NoError(t, parent.Err())
}

func TestSelectorContentFrameUsesOperationContext(t *testing.T) {
	t.Parallel()
	parent, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	operation, stop := context.WithTimeout(parent, 40*time.Millisecond)
	defer stop()
	handle := &ElementHandle{BaseJSHandle: BaseJSHandle{ctx: parent, session: &selectorDeadlineSession{}, remoteObject: &runtime.RemoteObject{ObjectID: "iframe"}}}
	start := time.Now()
	_, err := handle.contentFrame(operation)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), 200*time.Millisecond)
	require.NoError(t, parent.Err())
}

func TestSelectorWaitTimeoutMessage(t *testing.T) {
	t.Parallel()
	for _, caller := range []string{"frame", "locator"} {
		for _, state := range []string{"timeout", "canceled", "parent deadline"} {
			t.Run(caller+"/"+state, func(t *testing.T) {
				t.Parallel()
				parent, cancel := context.WithCancel(t.Context())
				defer cancel()
				budget := 10 * time.Millisecond
				want := context.DeadlineExceeded
				switch state {
				case "canceled":
					cancel()
					want = context.Canceled
				case "parent deadline":
					var stop context.CancelFunc
					parent, stop = context.WithDeadline(parent, time.Now().Add(-time.Second))
					defer stop()
				}
				frame := &Frame{ctx: parent, log: log.NewNullLogger(), executionContexts: make(map[executionWorld]frameExecutionContext)}
				opts := NewFrameWaitForSelectorOptions(budget)
				var err error
				if caller == "frame" {
					_, err = frame.WaitForSelector("#missing", opts)
				} else {
					locator := &Locator{frame: frame, selector: "#missing", log: log.NewNullLogger()}
					err = locator.WaitFor(opts)
				}
				require.ErrorIs(t, err, want)
				if state == "timeout" {
					require.ErrorContains(t, err, "timed out after 10ms")
					require.NoError(t, parent.Err())
				} else {
					require.NotContains(t, err.Error(), "timed out after")
				}
			})
		}
	}
}
