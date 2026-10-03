package eventdispatcher

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/event"
)

func TestEventSystem(t *testing.T) {
	t.Parallel()
	t.Run("subscribe", func(t *testing.T) {
		t.Parallel()
		logger := logrus.New()
		logger.SetOutput(io.Discard)
		es := NewEventSystem(10, logger)

		require.Len(t, es.subscribers, 0)

		s1id, s1ch := es.Subscribe(event.Init)

		assert.Equal(t, uint64(1), s1id)
		assert.NotNil(t, s1ch)
		assert.Len(t, es.subscribers, 1)
		assert.Len(t, es.subscribers[event.Init], 1)
		assert.Equal(t, (<-chan *event.Event)(es.subscribers[event.Init][s1id]), s1ch)

		s2id, s2ch := es.Subscribe(event.Init, event.TestStart)

		assert.Equal(t, uint64(2), s2id)
		assert.NotNil(t, s2ch)
		assert.Len(t, es.subscribers, 2)
		assert.Len(t, es.subscribers[event.Init], 2)
		assert.Len(t, es.subscribers[event.TestStart], 1)
		assert.Equal(t, (<-chan *event.Event)(es.subscribers[event.Init][s2id]), s2ch)
		assert.Equal(t, (<-chan *event.Event)(es.subscribers[event.TestStart][s2id]), s2ch)
	})

	t.Run("subscribe/panic", func(t *testing.T) {
		t.Parallel()
		logger := logrus.New()
		logger.SetOutput(io.Discard)
		es := NewEventSystem(10, logger)
		assert.PanicsWithValue(t, "must subscribe to at least 1 event type", func() {
			es.Subscribe()
		})
	})

	t.Run("emit_and_process", func(t *testing.T) {
		t.Parallel()
		testTimeout := 5 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()
		logger := logrus.New()
		logger.SetOutput(io.Discard)
		es := NewEventSystem(10, logger)

		s1id, s1ch := es.Subscribe(event.Init, event.Exit)
		s2id, s2ch := es.Subscribe(event.Init, event.TestStart, event.TestEnd, event.Exit)

		type result struct {
			sid    uint64
			events []*event.Event
			err    error
		}
		resultCh := make(chan result, 2)
		go func() {
			s1result, err := processEvents(ctx, es, s1id, s1ch)
			resultCh <- result{s1id, s1result, err}
		}()

		go func() {
			s2result, err := processEvents(ctx, es, s2id, s2ch)
			resultCh <- result{s2id, s2result, err}
		}()

		var (
			doneMx     sync.RWMutex
			processed  = make(map[event.Type]int)
			emitEvents = []event.Type{event.Init, event.TestStart, event.IterStart, event.IterEnd, event.TestEnd, event.Exit}
			data       int
		)
		for _, et := range emitEvents {
			evt := &event.Event{Type: et, Data: data, Done: func() {
				doneMx.Lock()
				processed[et]++
				doneMx.Unlock()
			}}
			es.Emit(evt)
			data++
		}

		for range 2 {
			select {
			case result := <-resultCh:
				require.NoError(t, result.err)
				switch result.sid {
				case s1id:
					require.Len(t, result.events, 2)
					assert.Equal(t, event.Init, result.events[0].Type)
					assert.Equal(t, 0, result.events[0].Data)
					assert.Equal(t, event.Exit, result.events[1].Type)
					assert.Equal(t, 5, result.events[1].Data)
				case s2id:
					require.Len(t, result.events, 4)
					assert.Equal(t, event.Init, result.events[0].Type)
					assert.Equal(t, 0, result.events[0].Data)
					assert.Equal(t, event.TestStart, result.events[1].Type)
					assert.Equal(t, 1, result.events[1].Data)
					assert.Equal(t, event.TestEnd, result.events[2].Type)
					assert.Equal(t, 4, result.events[2].Data)
					assert.Equal(t, event.Exit, result.events[3].Type)
					assert.Equal(t, 5, result.events[3].Data)
				}
			case <-ctx.Done():
				t.Fatalf("test timed out after %s", testTimeout)
			}
		}

		expProcessed := map[event.Type]int{
			event.Init:      2,
			event.TestStart: 1,
			event.TestEnd:   1,
			event.Exit:      2,
		}
		assert.Equal(t, expProcessed, processed)
	})

	t.Run("emit_and_wait/ok", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		logger := logrus.New()
		logger.SetOutput(io.Discard)
		es := NewEventSystem(100, logger)

		var (
			wg      sync.WaitGroup
			numSubs = 100
		)
		for range numSubs {
			sid, evtCh := es.Subscribe(event.Exit)
			wg.Go(func() {
				_, err := processEvents(ctx, es, sid, evtCh)
				require.NoError(t, err)
			})
		}

		var done uint32
		wait := es.Emit(&event.Event{Type: event.Exit, Done: func() {
			atomic.AddUint32(&done, 1)
		}})
		waitCtx, waitCancel := context.WithTimeout(ctx, time.Second)
		defer waitCancel()
		err := wait(waitCtx)
		require.NoError(t, err)
		assert.Equal(t, uint32(numSubs), done)

		wg.Wait()
	})

	// This ensures that the system still works even when the buffer size of
	// the event system is smaller than the numSubs. We had an issue where
	// when all the sub were trying to call done it would fail since the buffer
	// was full and the event would never fully complete and wait indefinitely.
	t.Run("emit_and_wait/buffer", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		logger := logrus.New()
		logger.SetOutput(io.Discard)
		// Not buffered
		es := NewEventSystem(0, logger)

		var (
			wg      sync.WaitGroup
			numSubs = 100
		)
		for range numSubs {
			sid, evtCh := es.Subscribe(event.Exit)
			wg.Go(func() {
				_, err := processEvents(ctx, es, sid, evtCh)
				require.NoError(t, err)
			})
		}

		var done uint32
		wait := es.Emit(&event.Event{Type: event.Exit, Done: func() {
			atomic.AddUint32(&done, 1)
		}})
		waitCtx, waitCancel := context.WithTimeout(ctx, time.Second)
		defer waitCancel()
		err := wait(waitCtx)
		require.NoError(t, err)
		assert.Equal(t, uint32(numSubs), done)

		wg.Wait()
	})

	t.Run("emit_and_wait/error", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		logger := logrus.New()
		logger.SetOutput(io.Discard)
		es := NewEventSystem(10, logger)

		sid, evtCh := es.Subscribe(event.Exit)
		var wg sync.WaitGroup
		wg.Go(func() {
			_, err := processEvents(ctx, es, sid, evtCh)
			assert.NoError(t, err)
		})

		wait := es.Emit(&event.Event{Type: event.Exit, Done: func() {
			time.Sleep(200 * time.Millisecond)
		}})
		waitCtx, waitCancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer waitCancel()
		err := wait(waitCtx)
		assert.EqualError(t, err, "context is done before all 'Exit' events were processed")

		wg.Wait()
	})

	t.Run("unsubscribe", func(t *testing.T) {
		t.Parallel()
		logger := logrus.New()
		logger.SetOutput(io.Discard)
		es := NewEventSystem(10, logger)

		require.Len(t, es.subscribers, 0)

		var (
			numSubs = 5
			subs    = make([]uint64, numSubs)
		)
		for i := range numSubs {
			sid, _ := es.Subscribe(event.Init)
			subs[i] = sid
		}

		require.Len(t, es.subscribers[event.Init], numSubs)

		es.Unsubscribe(subs[0])
		assert.Len(t, es.subscribers[event.Init], numSubs-1)
		es.Unsubscribe(subs[0]) // second unsubscribe does nothing
		assert.Len(t, es.subscribers[event.Init], numSubs-1)

		es.UnsubscribeAll()
		assert.Len(t, es.subscribers[event.Init], 0)
	})
}

func processEvents(ctx context.Context, es *System, sid uint64, evtCh <-chan *event.Event) ([]*event.Event, error) {
	result := make([]*event.Event, 0)

	for {
		select {
		case evt, ok := <-evtCh:
			if !ok {
				return result, nil
			}
			result = append(result, evt)
			evt.Done()
			if evt.Type == event.Exit {
				es.Unsubscribe(sid)
			}
		case <-ctx.Done():
			return nil, errors.New("test timed out")
		}
	}
}
