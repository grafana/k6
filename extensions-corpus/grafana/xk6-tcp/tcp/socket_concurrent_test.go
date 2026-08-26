package tcp

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConcurrentPropertyAccess(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	s := newSocket(mod.log, mod.vu, mod.metrics)

	s.state = socketStateOpen
	s.totalWritten = 1000
	s.totalRead = 2000

	_, cancel := context.WithCancel(mod.vu.Context())

	s.cancel = cancel

	// Simulate concurrent access to properties
	var wg sync.WaitGroup

	concurrency := 10
	iterations := 100

	// Test concurrent reads of state
	wg.Add(concurrency)

	for range concurrency {
		go func() {
			defer wg.Done()

			for range iterations {
				_ = s.readyState()
				_ = s.isConnected()
				_ = s.isReadable()
				_ = s.bytesWritten()
				_ = s.bytesRead()
			}
		}()
	}

	wg.Wait()

	// Verify state is still consistent
	require.Equal(t, "open", s.readyState())
	require.Equal(t, int64(1000), s.bytesWritten())
	require.Equal(t, int64(2000), s.bytesRead())
}

func TestConcurrentSetTimeout(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	s := newSocket(mod.log, mod.vu, mod.metrics)

	s.this = mod.vu.Runtime().NewObject()

	_, cancel := context.WithCancel(mod.vu.Context())

	s.cancel = cancel
	s.vu = mod.vu

	var wg sync.WaitGroup

	concurrency := 5

	// Test concurrent setTimeout calls
	wg.Add(concurrency)

	for i := range concurrency {
		timeout := int64((i + 1) * 1000)

		go func(t int64) {
			defer wg.Done()

			_, _ = s.setTimeout(t)
		}(timeout)
	}

	wg.Wait()

	// Verify timeout is set (could be any of the values)
	s.mu.RLock()
	require.Greater(t, s.timeout, time.Duration(0))
	s.mu.RUnlock()
}

func TestConcurrentHandlerRegistration(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	s := newSocket(mod.log, mod.vu, mod.metrics)

	jsruntime := mod.vu.Runtime()

	handler := jsruntime.ToValue(func() {})
	callable := jsruntime.ToValue(handler).ToObject(jsruntime).Get("call").ToObject(jsruntime)

	events := []string{"connect", "data", "close", "error", "timeout"}

	var wg sync.WaitGroup

	wg.Add(len(events))

	// Register handlers concurrently
	for _, event := range events {
		go func() {
			defer wg.Done()

			s.handlers.Store(event, callable)
		}()
	}

	wg.Wait()

	// Verify all handlers are registered
	for _, event := range events {
		_, ok := s.handlers.Load(event)
		require.True(t, ok, "Handler for %s should be registered", event)
	}
}

func TestRaceConditionInEventFiring(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)
	jsruntime := mod.vu.Runtime()

	s := newSocket(mod.log, mod.vu, mod.metrics)
	s.this = jsruntime.NewObject()
	ctx, cancel := context.WithTimeout(mod.vu.Context(), 5*time.Second)

	defer cancel()

	s.cancel = cancel
	s.vu = mod.vu

	// Start event loop
	go s.loop(ctx)

	var callCount int32

	handler := jsruntime.ToValue(func() {})
	callable := jsruntime.ToValue(handler).ToObject(jsruntime).Get("call").ToObject(jsruntime)

	s.handlers.Store("connect", callable)

	var wg sync.WaitGroup

	concurrency := 10

	// Fire events concurrently
	wg.Add(concurrency)

	for range concurrency {
		go func() {
			defer wg.Done()

			for range 10 {
				s.fire("connect")
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond) // Allow events to process

	// Test passes if no race conditions detected
	_ = callCount
}
