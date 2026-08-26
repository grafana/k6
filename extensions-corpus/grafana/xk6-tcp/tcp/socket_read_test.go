package tcp

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

var errConnectionResetByPeer = errors.New("connection reset by peer")

func newRunningModuleInstance(t *testing.T) *module {
	t.Helper()

	mod, _ := newRunningModuleInstanceWithSamples(t)

	return mod
}

// newRunningModuleInstanceWithSamples also returns the sample channel, since lib.State
// exposes it as send-only and tests cannot read back from it.
func newRunningModuleInstanceWithSamples(t *testing.T) (*module, chan metrics.SampleContainer) {
	t.Helper()

	runtime := modulestest.NewRuntime(t)
	root := new(rootModule)
	moduleInstance := root.NewModuleInstance(runtime.VU)

	mod, ok := moduleInstance.(*module)
	if !ok {
		t.Fatalf("failed to assert module instance")
	}

	samples := make(chan metrics.SampleContainer, 1000)

	registry := runtime.VU.InitEnvField.Registry
	runtime.MoveToVUContext(&lib.State{
		Samples: samples,
		Tags:    lib.NewVUStateTags(registry.RootTagSet()),
	})

	return mod, samples
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

// A read error must emit its tcp_errors sample with a non-nil TagSet. k6's outputs call
// Tags.Get on every sample during the periodic metric flush, so a nil TagSet panics the
// flusher goroutine and takes down the whole test run.
func TestReadLoopStepFatalErrorPushesTaggedSample(t *testing.T) {
	t.Parallel()

	mod, samples := newRunningModuleInstanceWithSamples(t)
	s := newSocket(mod.log, mod.vu, mod.metrics)
	_, cancel := context.WithCancel(mod.vu.Context())
	s.cancel = cancel
	s.state = socketStateOpen

	conn := &stubConn{readErr: errConnectionResetByPeer}
	require.False(t, s.readLoopStep(conn, 0))

	require.Len(t, samples, 1)

	emitted := (<-samples).GetSamples()
	require.Len(t, emitted, 1)
	require.Equal(t, tcpErrors, emitted[0].Metric.Name)
	require.NotNil(t, emitted[0].Tags)

	// mirrors what k6's summary output does with every flushed sample
	_, hasCheckTag := emitted[0].Tags.Get(metrics.TagCheck.String())
	require.False(t, hasCheckTag)
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
