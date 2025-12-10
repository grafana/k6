package common

import (
	"context"
	"sync"
)

const (
	// Browser

	EventBrowserDisconnected string = "disconnected"

	// BrowserContext

	EventBrowserContextPage string = "page"

	// Connection

	EventConnectionClose string = "close"

	// Frame

	EventFrameNavigation   string = "navigation"
	EventFrameAddLifecycle string = "addlifecycle"
)

// Event as emitted by an EventEmiter.
type Event struct {
	typ  string
	data any
}

// NavigationEvent is emitted when we receive a Page.frameNavigated or
// Page.navigatedWithinDocument CDP event.
// See:
// - https://chromedevtools.github.io/devtools-protocol/tot/Page/#event-frameNavigated
// - https://chromedevtools.github.io/devtools-protocol/tot/Page/#event-navigatedWithinDocument
type NavigationEvent struct {
	newDocument *DocumentInfo
	url         string
	name        string
	err         error
}

type queue struct {
	writeMutex sync.Mutex
	write      []Event
	readMutex  sync.Mutex
	read       []Event
}

type eventHandler struct {
	ctx   context.Context
	ch    chan Event
	queue *queue
}

// EventEmitter that all event emitters need to implement.
type EventEmitter interface {
	emit(event string, data any)
	on(ctx context.Context, events []string, ch chan Event)
	onAll(ctx context.Context, ch chan Event)
}

// syncFunc functions are passed through the syncCh for synchronously handling
// eventHandler requests.
type syncFunc func() (done chan struct{})

// BaseEventEmitter emits events to registered handlers.
type BaseEventEmitter struct {
	handlers    map[string][]*eventHandler
	handlersAll []*eventHandler

	queues map[chan Event]*queue

	syncCh chan syncFunc
	ctx    context.Context
}

// NewBaseEventEmitter creates a new instance of a base event emitter.
func NewBaseEventEmitter(ctx context.Context) BaseEventEmitter {
	bem := BaseEventEmitter{
		handlers: make(map[string][]*eventHandler),
		syncCh:   make(chan syncFunc),
		ctx:      ctx,
		queues:   make(map[chan Event]*queue),
	}
	go bem.syncAll(ctx)
	return bem
}

// syncAll receives work requests from BaseEventEmitter methods
// and processes them one at a time for synchronization.
//
// It returns when the BaseEventEmitter context is done.
func (e *BaseEventEmitter) syncAll(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case fn := <-e.syncCh:
			// run the function and signal when it's done
			done := fn()
			done <- struct{}{}
		}
	}
}

// sync is a helper for sychronized access to the BaseEventEmitter.
func (e *BaseEventEmitter) sync(fn func()) {
	done := make(chan struct{})
	select {
	case <-e.ctx.Done():
		return
	case e.syncCh <- func() chan struct{} {
		fn()
		return done
	}:
	}
	// wait for the function to return
	<-done
}

func (e *BaseEventEmitter) emit(event string, data any) {
	emitEvent := func(eh *eventHandler) {
		eh.queue.readMutex.Lock()
		defer eh.queue.readMutex.Unlock()

		// We try to read from the read queue (queue.read).
		// If there isn't anything on the read queue, then there must
		// be something being populated by the synched emitTo
		// func below.
		// Swap around the read queue with the write queue.
		// Queue is now being populated again by emitTo, and all
		// emitEvent goroutines can continue to consume from
		// the read queue until that is again depleted.
		if len(eh.queue.read) == 0 {
			eh.queue.writeMutex.Lock()
			// Clear all data references in the old read slice to help GC
			for i := range eh.queue.read {
				eh.queue.read[i].data = nil
			}
			// Use nil instead of make([]Event, 0) to allow GC to reclaim the underlying array
			eh.queue.read = nil
			eh.queue.read, eh.queue.write = eh.queue.write, eh.queue.read
			// Ensure read is a valid slice (not nil) after swap, even if write was empty
			if eh.queue.read == nil {
				eh.queue.read = make([]Event, 0)
			}
			// If write queue capacity is much larger than length, shrink it to reduce memory
			if cap(eh.queue.write) > 64 && cap(eh.queue.write) > len(eh.queue.write)*4 {
				// Reallocate with smaller capacity to allow GC to reclaim memory
				oldWrite := eh.queue.write
				eh.queue.write = make([]Event, len(oldWrite), len(oldWrite)*2)
				copy(eh.queue.write, oldWrite)
			}
			eh.queue.writeMutex.Unlock()
		}

		// If read is still empty after swap, nothing to emit
		if len(eh.queue.read) == 0 {
			return
		}

		select {
		case eh.ch <- eh.queue.read[0]:
			// Explicitly clear data reference to help GC before removing from slice
			eh.queue.read[0].data = nil
			eh.queue.read[0] = Event{}
			eh.queue.read = eh.queue.read[1:]
			// Shrink read slice capacity if it's much larger than length to reduce memory
			if len(eh.queue.read) == 0 && cap(eh.queue.read) > 64 {
				eh.queue.read = nil
			} else if cap(eh.queue.read) > 64 && cap(eh.queue.read) > len(eh.queue.read)*4 {
				// Reallocate with smaller capacity
				oldRead := eh.queue.read
				eh.queue.read = make([]Event, len(oldRead), len(oldRead)*2)
				copy(eh.queue.read, oldRead)
			}
		case <-eh.ctx.Done():
			// TODO: handle the error
		}
	}
	emitTo := func(handlers []*eventHandler) (updated []*eventHandler) {
		for i := 0; i < len(handlers); {
			handler := handlers[i]
			select {
			case <-handler.ctx.Done():
				handlers = append(handlers[:i], handlers[i+1:]...)
				continue
			default:
				handler.queue.writeMutex.Lock()
				handler.queue.write = append(handler.queue.write, Event{typ: event, data: data})
				handler.queue.writeMutex.Unlock()

				go emitEvent(handler)
				i++
			}
		}
		return handlers
	}
	e.sync(func() {
		e.handlers[event] = emitTo(e.handlers[event])
		e.handlersAll = emitTo(e.handlersAll)
	})
}

// On registers a handler for a specific event.
func (e *BaseEventEmitter) on(ctx context.Context, events []string, ch chan Event) {
	e.sync(func() {
		q, ok := e.queues[ch]
		if !ok {
			q = &queue{}
			e.queues[ch] = q
		}

		for _, event := range events {
			e.handlers[event] = append(e.handlers[event], &eventHandler{ctx: ctx, ch: ch, queue: q})
		}
	})
}

// OnAll registers a handler for all events.
func (e *BaseEventEmitter) onAll(ctx context.Context, ch chan Event) {
	e.sync(func() {
		q, ok := e.queues[ch]
		if !ok {
			q = &queue{}
			e.queues[ch] = q
		}

		e.handlersAll = append(e.handlersAll, &eventHandler{ctx: ctx, ch: ch, queue: q})
	})
}
