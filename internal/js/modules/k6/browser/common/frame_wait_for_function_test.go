package common

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

func TestWaitForFunctionCanceledSend(t *testing.T) {
	t.Parallel()

	for _, state := range []string{"timeout", "cancel"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()

			// Bound the unpatched implementation, which has no Go-side wait timeout.
			owner, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			logger := log.NewNullLogger()
			// No reader accepts a command: cancellation reaches Connection.send
			// before the predicate or polling evaluation can reach the browser.
			conn := &Connection{ctx: owner, logger: logger, sendCh: make(chan *cdproto.Message), done: make(chan struct{})}
			s := NewSession(owner, conn, "blocked", "blocked", logger, &msgID{})
			defer s.close()
			manager := NewFrameManager(owner, nil, &Page{}, NewTimeoutSettings(nil), logger)
			frame := NewFrame(owner, manager, nil, cdp.FrameID("blocked"), logger)
			ec := NewExecutionContext(owner, s, frame, 1, logger)
			// A previously used context already has its injected script cached.
			ec.injectedScript = NewJSHandle(owner, s, ec, frame,
				&runtime.RemoteObject{Type: runtime.TypeObject, ObjectID: "injected"}, logger)
			frame.setContext(mainWorld, ec)
			budget := 25 * time.Millisecond
			want := context.DeadlineExceeded
			if state == "cancel" {
				budget = time.Second
				want = context.Canceled
				timer := time.AfterFunc(25*time.Millisecond, cancel)
				defer timer.Stop()
			}
			started := time.Now()
			result, err := frame.WaitForFunction(`() => false`, NewFrameWaitForFunctionOptions(budget))
			t.Logf("elapsed=%s result=%v error=%v", time.Since(started), result, err)
			require.ErrorIs(t, err, want, "a canceled send must not become successful completion")
			require.Nil(t, result)
			require.Less(t, time.Since(started), 500*time.Millisecond)
		})
	}
}

func TestWaitForFunctionWithoutContext(t *testing.T) {
	t.Parallel()

	for _, state := range []string{"timeout", "closed", "detached", "canceled"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()

			// The parent deadline bounds the unpatched implementation, which only
			// checks the frame context while waiting for an execution context.
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			logger := log.NewNullLogger()
			manager := NewFrameManager(ctx, nil, &Page{}, NewTimeoutSettings(nil), logger)
			frame := NewFrame(ctx, manager, nil, cdp.FrameID("waiting"), logger)
			switch state {
			case "closed":
				frame.page.closed = true
			case "detached":
				frame.setDetached(true)
			case "canceled":
				cancel()
			}

			started := time.Now()
			_, err := frame.WaitForFunction(`() => true`, NewFrameWaitForFunctionOptions(50*time.Millisecond))
			require.Less(t, time.Since(started), 500*time.Millisecond)
			switch state {
			case "timeout":
				require.ErrorIs(t, err, context.DeadlineExceeded)
				require.ErrorContains(t, err, "timed out after 50ms")
			case "closed":
				require.ErrorContains(t, err, "page is closed")
			case "detached":
				require.ErrorIs(t, err, ErrFrameDetached)
			case "canceled":
				require.ErrorIs(t, err, context.Canceled)
			}
		})
	}
}
