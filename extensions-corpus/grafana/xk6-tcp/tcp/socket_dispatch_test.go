package tcp

import (
	"sync"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
)

func TestFireBlocksUntilLoopReceivesCallback(t *testing.T) {
	t.Parallel()

	mod := newRunningModuleInstance(t)
	s := newSocket(mod.log, mod.vu, mod.metrics)

	handler := mod.vu.Runtime().ToValue(func() {})
	callable, ok := sobek.AssertFunction(handler)
	require.True(t, ok)

	s.on("connect", callable)

	queued := make(chan struct{})

	go func() {
		s.fire("connect")
		close(queued)
	}()

	require.Never(t, func() bool {
		select {
		case <-queued:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, time.Millisecond)

	go func() {
		callback := <-s.callChan
		_ = callback()
	}()

	require.Eventually(t, func() bool {
		select {
		case <-queued:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)
}

func TestFirePreservesEventOrder(t *testing.T) {
	t.Parallel()

	mod := newRunningModuleInstance(t)
	s := newSocket(mod.log, mod.vu, mod.metrics)

	var (
		mu     sync.Mutex
		events []string
	)

	record := func(name string) {
		mu.Lock()
		defer mu.Unlock()

		events = append(events, name)
	}

	connectHandler := mod.vu.Runtime().ToValue(func() {
		record("connect")
	})
	connectCallable, ok := sobek.AssertFunction(connectHandler)
	require.True(t, ok)

	closeHandler := mod.vu.Runtime().ToValue(func() {
		record("close")
	})
	closeCallable, ok := sobek.AssertFunction(closeHandler)
	require.True(t, ok)

	s.on("connect", connectCallable)
	s.on("close", closeCallable)

	done := make(chan struct{})

	go func() {
		defer close(done)

		for range 2 {
			callback := <-s.callChan
			_ = callback()
		}
	}()

	s.fire("connect")
	s.fire("close")

	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)

	require.Equal(t, []string{"connect", "close"}, events)
}
