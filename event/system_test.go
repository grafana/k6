package event

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
)

func TestEventSystem(t *testing.T) {
	t.Parallel()
	t.Run("subscribe", func(t *testing.T) {
		t.Parallel()
		logger := logrus.New()
		logger.SetOutput(io.Discard)
		es := NewEventSystem(10, logger)

		require.Len(t, es.subscribers, 0)

		s1id, s1ch := es.Subscribe(Init)

		assert.Equal(t, uint64(1), s1id)
		assert.NotNil(t, s1ch)
		assert.Len(t, es.subscribers, 1)
		assert.Len(t, es.subscribers[Init], 1)
		assert.Equal(t, (<-chan *Event)(es.subscribers[Init][s1id]), s1ch)

		s2id, s2ch := es.Subscribe(Init, TestStart)

		assert.Equal(t, uint64(2), s2id)
		assert.NotNil(t, s2ch)
		assert.Len(t, es.subscribers, 2)
		assert.Len(t, es.subscribers[Init], 2)
		assert.Len(t, es.subscribers[TestStart], 1)
		assert.Equal(t, (<-chan *Event)(es.subscribers[Init][s2id]), s2ch)
		assert.Equal(t, (<-chan *Event)(es.subscribers[TestStart][s2id]), s2ch)
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

		s1id, s1ch := es.Subscribe(Init, Exit)
		s2id, s2ch := es.Subscribe(Init, TestStart, TestEnd, Exit)

		type result struct {
			sid    uint64
			events []*Event
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
			processed  = make(map[Type]int)
			emitEvents = []Type{Init, TestStart, IterStart, IterEnd, TestEnd, Exit}
			data       int
		)
		for _, et := range emitEvents {
			et := et
			evt := &Event{Type: et, Data: data, Done: func() {
				doneMx.Lock()
				processed[et]++
				doneMx.Unlock()
			}}
			es.Emit(evt)
			data++
		}

		for i := 0; i < 2; i++ {
			select {
			case result := <-resultCh:
				require.NoError(t, result.err)
				switch result.sid {
				case s1id:
					require.Len(t, result.events, 2)
					assert.Equal(t, Init, result.events[0].Type)
					assert.Equal(t, 0, result.events[0].Data)
					assert.Equal(t, Exit, result.events[1].Type)
					assert.Equal(t, 5, result.events[1].Data)
				case s2id:
					require.Len(t, result.events, 4)
					assert.Equal(t, Init, result.events[0].Type)
					assert.Equal(t, 0, result.events[0].Data)
					assert.Equal(t, TestStart, result.events[1].Type)
					assert.Equal(t, 1, result.events[1].Data)
					assert.Equal(t, TestEnd, result.events[2].Type)
					assert.Equal(t, 4, result.events[2].Data)
					assert.Equal(t, Exit, result.events[3].Type)
					assert.Equal(t, 5, result.events[3].Data)
				}
			case <-ctx.Done():
				t.Fatalf("test timed out after %s", testTimeout)
			}
		}

		expProcessed := map[Type]int{
			Init:      2,
			TestStart: 1,
			TestEnd:   1,
			Exit:      2,
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
		for i := 0; i < numSubs; i++ {
			sid, evtCh := es.Subscribe(Exit)
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := processEvents(ctx, es, sid, evtCh)
				require.NoError(t, err)
			}()
		}

		var done uint32
		wait := es.Emit(&Event{Type: Exit, Done: func() {
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

		sid, evtCh := es.Subscribe(Exit)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := processEvents(ctx, es, sid, evtCh)
			assert.NoError(t, err)
		}()

		wait := es.Emit(&Event{Type: Exit, Done: func() {
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
		for i := 0; i < numSubs; i++ {
			sid, _ := es.Subscribe(Init)
			subs[i] = sid
		}

		require.Len(t, es.subscribers[Init], numSubs)

		es.Unsubscribe(subs[0])
		assert.Len(t, es.subscribers[Init], numSubs-1)
		es.Unsubscribe(subs[0]) // second unsubscribe does nothing
		assert.Len(t, es.subscribers[Init], numSubs-1)

		es.UnsubscribeAll()
		assert.Len(t, es.subscribers[Init], 0)
	})
}

func processEvents(ctx context.Context, es *System, sid uint64, evtCh <-chan *Event) ([]*Event, error) {
	result := make([]*Event, 0)

	for {
		select {
		case evt, ok := <-evtCh:
			if !ok {
				return result, nil
			}
			result = append(result, evt)
			evt.Done()
			if evt.Type == Exit {
				es.Unsubscribe(sid)
			}
		case <-ctx.Done():
			return nil, errors.New("test timed out")
		}
	}
}
