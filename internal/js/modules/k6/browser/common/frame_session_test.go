package common

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/log"
)

// newClosingTestPage returns a minimal Page with closingCh already closed,
// simulating a page in closing state. Used by tests that verify attachment
// rejection during shutdown.
func newClosingTestPage(ctx context.Context, logger *log.Logger) *Page {
	p := &Page{
		ctx:           ctx,
		teardownCtx:   ctx,
		session:       &Session{id: "test-session"},
		closingCh:     make(chan struct{}),
		frameSessions: make(map[cdp.FrameID]*FrameSession),
		logger:        logger,
	}
	close(p.closingCh)
	return p
}

// TestAttachIFrameToTargetPageClosing verifies that when the page is already
// closing, attachIFrameToTarget does NOT register the frame session and
// instead issues the best-effort CDP unblock+detach calls:
//
//  1. Runtime.runIfWaitingForDebugger
//  2. Target.detachFromTarget
func TestAttachIFrameToTargetPageClosing(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	logger := log.NewNullLogger()

	p := newClosingTestPage(ctx, logger)
	fs := &FrameSession{
		ctx:         ctx,
		teardownCtx: ctx,
		page:        p,
		logger:      logger,
	}

	sendCh := make(chan *cdproto.Message, 16)
	session := &Session{
		BaseEventEmitter: NewBaseEventEmitter(ctx),
		conn: &Connection{
			BaseEventEmitter: NewBaseEventEmitter(ctx),
			sendCh:           sendCh,
			closeCh:          make(chan int),
			errorCh:          make(chan error),
			done:             make(chan struct{}),
			closing:          make(chan struct{}),
			logger:           logger,
		},
		id:       "iframe-session-1",
		done:     make(chan struct{}),
		logger:   logger,
		msgIDGen: &msgID{},
	}

	ti := &target.Info{
		TargetID: "iframe-target-1",
		Type:     "iframe",
	}

	err := fs.attachIFrameToTarget(ti, session)
	require.NoError(t, err)

	p.frameSessionsMu.RLock()
	assert.Empty(t, p.frameSessions, "expected no frame sessions to be registered when page is closing")
	p.frameSessionsMu.RUnlock()

	// Drain sendCh to collect the CDP methods issued, in order.
	var methods []string
	for {
		select {
		case msg := <-sendCh:
			methods = append(methods, string(msg.Method))
		default:
			goto done
		}
	}
done:
	assert.Equal(t, []string{
		"Runtime.runIfWaitingForDebugger",
		"Target.detachFromTarget",
	}, methods)
}

func TestFrameSessionWaitDrainsAllTrackedWorkers(t *testing.T) {
	t.Parallel()

	fsCtx, fsCancel := context.WithCancelCause(t.Context())
	nmCtx, nmCancel := context.WithCancelCause(fsCtx)

	nm := &NetworkManager{
		ctx:    nmCtx,
		cancel: nmCancel,
	}

	const (
		fsWorkers = 3
		nmWorkers = 4
	)
	var fsStopped atomic.Int32
	var nmStopped atomic.Int32

	fs := &FrameSession{
		ctx:            fsCtx,
		cancel:         fsCancel,
		networkManager: nm,
	}

	for range fsWorkers {
		fs.wg.Add(1)
		go func() {
			defer fs.wg.Done()
			<-fs.ctx.Done()
			fsStopped.Add(1)
		}()
	}

	for range nmWorkers {
		nm.wg.Add(1)
		go func() {
			defer nm.wg.Done()
			<-nm.ctx.Done()
			nmStopped.Add(1)
		}()
	}

	ctx, closeCancel := context.WithTimeout(t.Context(), time.Second)
	defer closeCancel()

	fs.cancel(errors.New("test cancel frame session"))
	require.NoError(t, fs.wait(ctx))
	require.EqualValues(t, fsWorkers, fsStopped.Load(), "all frame-session workers must stop before wait returns")
	require.EqualValues(t, nmWorkers, nmStopped.Load(), "all network-manager workers must stop before wait returns")
}
