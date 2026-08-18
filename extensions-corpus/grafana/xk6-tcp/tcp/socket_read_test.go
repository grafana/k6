package tcp

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	extensionapitest "go.k6.io/k6-extension-api/test"
)

var errConnectionResetByPeer = errors.New("connection reset by peer")

func newRunningModuleInstance(t *testing.T) *module {
	t.Helper()

	vu := extensionapitest.NewVU()
	root := new(rootModule)
	moduleInstance := root.NewModuleInstance(vu)

	mod, ok := moduleInstance.(*module)
	if !ok {
		t.Fatalf("failed to assert module instance")
	}

	return mod
}

type stubConn struct {
	net.Conn

	readErr error
}

func (c *stubConn) Read(_ []byte) (int, error)        { return 0, c.readErr }
func (c *stubConn) SetReadDeadline(_ time.Time) error { return nil }
func (c *stubConn) Close() error                      { return nil }

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "i/o timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

func TestReadLoopStepFatalErrorReturnsFalse(t *testing.T) {
	t.Parallel()

	mod := newRunningModuleInstance(t)
	s := newSocket(mod.log, mod.vu, mod.metrics)
	_, cancel := context.WithCancel(mod.vu.Context())
	s.cancel = cancel
	s.state = socketStateOpen

	conn := &stubConn{readErr: errConnectionResetByPeer}
	require.False(t, s.readLoopStep(conn, 0))
}

func TestReadLoopStepEOFReturnsFalse(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)
	s := newSocket(mod.log, mod.vu, mod.metrics)
	_, cancel := context.WithCancel(mod.vu.Context())
	s.cancel = cancel
	s.state = socketStateOpen

	conn := &stubConn{readErr: io.EOF}
	require.False(t, s.readLoopStep(conn, 0))
}

func TestReadLoopStepTimeoutReturnsTrue(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)
	s := newSocket(mod.log, mod.vu, mod.metrics)
	_, cancel := context.WithCancel(mod.vu.Context())
	s.cancel = cancel
	s.state = socketStateOpen

	conn := &stubConn{readErr: &net.OpError{Op: "read", Err: &timeoutError{}}}
	require.True(t, s.readLoopStep(conn, 0))
}

func TestFatalReadErrorDestroysSocket(t *testing.T) {
	t.Parallel()

	mod := newRunningModuleInstance(t)
	s := newSocket(mod.log, mod.vu, mod.metrics)
	s.socketOpts = new(socketOptions)

	ctx, cancel := context.WithCancel(mod.vu.Context())
	s.cancel = cancel

	go s.loop(ctx)

	s.state = socketStateOpen
	s.conn = &stubConn{readErr: errConnectionResetByPeer}

	go s.read()

	require.Eventually(t, func() bool {
		s.mu.RLock()
		state := s.state
		s.mu.RUnlock()

		return state == socketStateDestroyed
	}, time.Second, time.Millisecond)
}
