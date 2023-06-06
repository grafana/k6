package event

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// Subscriber is a limited interface of System that only allows subscribing and
// unsubscribing.
type Subscriber interface {
	Subscribe(events ...Type) (subID uint64, eventsCh <-chan *Event)
	Unsubscribe(subID uint64)
}

// System keeps track of subscribers, and allows subscribing to and emitting
// events.
type System struct {
	subMx       sync.RWMutex
	subIDCount  uint64
	subscribers map[Type]map[uint64]chan *Event
	eventBuffer int
	logger      logrus.FieldLogger
}

// NewEventSystem returns a new System.
// eventBuffer determines the size of the Event channel buffer. Events might be
// dropped if this buffer is full and there are no event listeners, or if events
// are emitted very quickly and the event handler goroutine is busy. It is
// recommended to handle events in a separate goroutine to not block the
// listener goroutine.
func NewEventSystem(eventBuffer int, logger logrus.FieldLogger) *System {
	return &System{
		subscribers: make(map[Type]map[uint64]chan *Event),
		eventBuffer: eventBuffer,
		logger:      logger,
	}
}

// Subscribe to one or more events. It returns a subscriber ID that can be
// used to unsubscribe, and an Event channel to receive events.
// It panics if events is empty.
func (s *System) Subscribe(events ...Type) (subID uint64, eventsCh <-chan *Event) {
	if len(events) == 0 {
		panic("must subscribe to at least 1 event type")
	}

	s.subMx.Lock()
	defer s.subMx.Unlock()
	s.subIDCount++
	subID = s.subIDCount

	evtCh := make(chan *Event, s.eventBuffer)
	for _, evt := range events {
		if s.subscribers[evt] == nil {
			s.subscribers[evt] = make(map[uint64]chan *Event)
		}
		s.subscribers[evt][subID] = evtCh
	}

	s.logger.WithFields(logrus.Fields{
		"subscriptionID": subID,
		"events":         events,
	}).Debug("Created event subscription")

	return subID, evtCh
}

// Emit the event to all subscribers of its type.
// It returns a function that can be optionally used to wait for all subscribers
// to process the event (by signalling via the Done method).
func (s *System) Emit(event *Event) (wait func(context.Context) error) {
	s.subMx.RLock()
	defer s.subMx.RUnlock()
	totalSubs := len(s.subscribers[event.Type])
	if totalSubs == 0 {
		return func(context.Context) error { return nil }
	}

	if event.Done == nil {
		event.Done = func() {}
	}
	origDoneFn := event.Done
	doneCh := make(chan struct{}, s.eventBuffer)
	doneFn := func() {
		origDoneFn()
		select {
		case doneCh <- struct{}{}:
		default:
		}
	}
	event.Done = doneFn

	for _, evtCh := range s.subscribers[event.Type] {
		select {
		case evtCh <- event:
		default:
		}
	}

	s.logger.WithFields(logrus.Fields{
		"subscribers": totalSubs,
		"event":       event.Type,
	}).Trace("Emitted event")

	return func(ctx context.Context) error {
		var doneCount int
		for {
			if doneCount == totalSubs {
				close(doneCh)
				return nil
			}
			select {
			case <-doneCh:
				doneCount++
			case <-ctx.Done():
				return fmt.Errorf("context is done before all '%s' events were processed", event.Type)
			}
		}
	}
}

// Unsubscribe closes the Event channel and removes the subscription with ID
// subID.
func (s *System) Unsubscribe(subID uint64) {
	s.subMx.Lock()
	defer s.subMx.Unlock()
	var seen bool
	for _, sub := range s.subscribers {
		if evtCh, ok := sub[subID]; ok {
			if !seen {
				close(evtCh)
			}
			delete(sub, subID)
			seen = true
		}
	}

	if seen {
		s.logger.WithFields(logrus.Fields{
			"subscriptionID": subID,
		}).Debug("Removed event subscription")
	}
}

// UnsubscribeAll closes all event channels and removes all subscriptions.
func (s *System) UnsubscribeAll() {
	s.subMx.Lock()
	defer s.subMx.Unlock()

	seenSubs := make(map[uint64]struct{})
	for _, sub := range s.subscribers {
		for subID, evtCh := range sub {
			if _, ok := seenSubs[subID]; !ok {
				close(evtCh)
				seenSubs[subID] = struct{}{}
			}
		}
	}

	if len(seenSubs) > 0 {
		s.logger.WithFields(logrus.Fields{
			"subscriptions": len(seenSubs),
		}).Debug("Removed all event subscriptions")
	}

	s.subscribers = make(map[Type]map[uint64]chan *Event)
}
