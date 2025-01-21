package common

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventEmitterSpecificEvent(t *testing.T) {
	t.Parallel()

	t.Run("add event handler", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		emitter := NewBaseEventEmitter(ctx)
		ch := make(chan Event)

		emitter.on(ctx, []string{cdproto.EventTargetTargetCreated}, ch)
		emitter.sync(func() {
			require.Len(t, emitter.handlers, 1)
			require.Contains(t, emitter.handlers, cdproto.EventTargetTargetCreated)
			require.Len(t, emitter.handlers[cdproto.EventTargetTargetCreated], 1)
			require.Equal(t, ch, emitter.handlers[cdproto.EventTargetTargetCreated][0].ch)
			require.Empty(t, emitter.handlersAll)
		})
	})

	t.Run("remove event handler", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		cancelCtx, cancelFn := context.WithCancel(ctx)
		emitter := NewBaseEventEmitter(cancelCtx)
		ch := make(chan Event)

		emitter.on(cancelCtx, []string{cdproto.EventTargetTargetCreated}, ch)
		cancelFn()
		emitter.emit(cdproto.EventTargetTargetCreated, nil) // Event handlers are removed as part of event emission

		emitter.sync(func() {
			require.Contains(t, emitter.handlers, cdproto.EventTargetTargetCreated)
			require.Len(t, emitter.handlers[cdproto.EventTargetTargetCreated], 0)
		})
	})

	t.Run("emit event", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		emitter := NewBaseEventEmitter(ctx)
		ch := make(chan Event, 1)

		emitter.on(ctx, []string{cdproto.EventTargetTargetCreated}, ch)
		emitter.emit(cdproto.EventTargetTargetCreated, "hello world")
		msg := <-ch

		emitter.sync(func() {
			require.Equal(t, cdproto.EventTargetTargetCreated, msg.typ)
			require.Equal(t, "hello world", msg.data)
		})
	})
}

func TestEventEmitterAllEvents(t *testing.T) {
	t.Parallel()

	t.Run("add catch-all event handler", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		emitter := NewBaseEventEmitter(ctx)
		ch := make(chan Event)

		emitter.onAll(ctx, ch)

		emitter.sync(func() {
			require.Len(t, emitter.handlersAll, 1)
			require.Equal(t, ch, emitter.handlersAll[0].ch)
			require.Empty(t, emitter.handlers)
		})
	})

	t.Run("remove catch-all event handler", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		emitter := NewBaseEventEmitter(ctx)
		cancelCtx, cancelFn := context.WithCancel(ctx)
		ch := make(chan Event)

		emitter.onAll(cancelCtx, ch)
		cancelFn()
		emitter.emit(cdproto.EventTargetTargetCreated, nil) // Event handlers are removed as part of event emission

		emitter.sync(func() {
			require.Len(t, emitter.handlersAll, 0)
		})
	})

	t.Run("emit event", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		emitter := NewBaseEventEmitter(ctx)
		ch := make(chan Event, 1)

		emitter.onAll(ctx, ch)
		emitter.emit(cdproto.EventTargetTargetCreated, "hello world")
		msg := <-ch

		emitter.sync(func() {
			require.Equal(t, cdproto.EventTargetTargetCreated, msg.typ)
			require.Equal(t, "hello world", msg.data)
		})
	})
}

func TestBaseEventEmitter(t *testing.T) {
	t.Parallel()

	t.Run("order of emitted events kept", func(t *testing.T) {
		t.Parallel()

		// Test description
		//
		// 1. Emit many events from the emitWorker.
		// 2. Handler receives the emitted events.
		//
		// Success criteria: Ensure that the ordering of events is
		//                   received in the order they're emitted.

		eventName := "AtomicIntEvent"
		maxInt := 100

		ctx, cancel := context.WithCancel(context.Background())
		emitter := NewBaseEventEmitter(ctx)
		ch := make(chan Event)
		emitter.on(ctx, []string{eventName}, ch)

		var expectedI int
		handler := func() {
			defer cancel()

			for expectedI != maxInt {
				e := <-ch

				i, ok := e.data.(int)
				if !ok {
					assert.FailNow(t, "unexpected type read from channel", e.data)
				}

				assert.Equal(t, eventName, e.typ)
				assert.Equal(t, expectedI, i)

				expectedI++
			}

			close(ch)
		}
		go handler()

		emitWorker := func() {
			for i := 0; i < maxInt; i++ {
				emitter.emit(eventName, i)
			}
		}
		go emitWorker()

		select {
		case <-ctx.Done():
		case <-time.After(time.Second * 2):
			assert.FailNow(t, "test timed out, deadlock?")
		}
	})

	t.Run("order of emitted different event types kept", func(t *testing.T) {
		t.Parallel()

		// Test description
		//
		// 1. Emit many different event types from the emitWorker.
		// 2. Handler receives the emitted events.
		//
		// Success criteria: Ensure that the ordering of events is
		//                   received in the order they're emitted.

		eventName1 := "AtomicIntEvent1"
		eventName2 := "AtomicIntEvent2"
		eventName3 := "AtomicIntEvent3"
		eventName4 := "AtomicIntEvent4"
		maxInt := 100

		ctx, cancel := context.WithCancel(context.Background())
		emitter := NewBaseEventEmitter(ctx)
		ch := make(chan Event)
		// Calling on twice to ensure that the same queue is used
		// internally for the same channel and handler.
		emitter.on(ctx, []string{eventName1, eventName2}, ch)
		emitter.on(ctx, []string{eventName3, eventName4}, ch)

		var expectedI int
		handler := func() {
			defer cancel()

			for expectedI != maxInt {
				e := <-ch

				i, ok := e.data.(int)
				if !ok {
					assert.FailNow(t, "unexpected type read from channel", e.data)
				}

				assert.Equal(t, expectedI, i)

				expectedI++
			}

			close(ch)
		}
		go handler()

		emitWorker := func() {
			for i := 0; i < maxInt; i += 4 {
				emitter.emit(eventName1, i)
				emitter.emit(eventName2, i+1)
				emitter.emit(eventName3, i+2)
				emitter.emit(eventName4, i+3)
			}
		}
		go emitWorker()

		select {
		case <-ctx.Done():
		case <-time.After(time.Second * 2):
			assert.FailNow(t, "test timed out, deadlock?")
		}
	})

	t.Run("handler can emit without deadlocking", func(t *testing.T) {
		t.Parallel()

		// Test description
		//
		// 1. Emit many events from the emitWorker.
		// 2. Handler receives emitted events (AtomicIntEvent1).
		// 3. Handler emits event as AtomicIntEvent2.
		// 4. Handler received emitted events again (AtomicIntEvent2).
		//
		// Success criteria: No deadlock should occur between receiving,
		//                   emitting, and receiving of events.

		eventName1 := "AtomicIntEvent1"
		eventName2 := "AtomicIntEvent2"
		maxInt := 100

		ctx, cancel := context.WithCancel(context.Background())
		emitter := NewBaseEventEmitter(ctx)
		ch := make(chan Event)
		emitter.on(ctx, []string{eventName1, eventName2}, ch)

		var expectedI2 int
		handler := func() {
			defer cancel()

			for expectedI2 != maxInt {
				e := <-ch

				switch e.typ {
				case eventName1:
					i, ok := e.data.(int)
					if !ok {
						assert.FailNow(t, "unexpected type read from channel", e.data)
					}
					emitter.emit(eventName2, i)
				case eventName2:
					expectedI2++
				default:
					assert.FailNow(t, "unexpected event type received")
				}
			}

			close(ch)
		}
		go handler()

		emitWorker := func() {
			for i := 0; i < maxInt; i++ {
				emitter.emit(eventName1, i)
			}
		}
		go emitWorker()

		select {
		case <-ctx.Done():
		case <-time.After(time.Second * 2):
			assert.FailNow(t, "test timed out, deadlock?")
		}
	})
}
