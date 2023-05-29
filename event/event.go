package event

import (
	"sync"
	"time"
)

const testEndTimeout = 5 * time.Second

type Type string

const (
	InitVUs   Type = "initVUs"
	TestStart Type = "testStart"
	TestEnd   Type = "testEnd"
)

type Event struct {
	Type Type
	Data any
	Done func()
}

type System struct {
	subMx       sync.RWMutex
	subscribers map[Type][]chan *Event
}

func NewEventSystem() *System {
	return &System{
		subscribers: make(map[Type][]chan *Event),
	}
}

func (s *System) Subscribe(events ...Type) <-chan *Event {
	s.subMx.Lock()
	defer s.subMx.Unlock()
	evtCh := make(chan *Event)
	for _, evt := range events {
		s.subscribers[evt] = append(s.subscribers[evt], evtCh)
	}
	return evtCh
}

func (s *System) Notify(event *Event) {
	if event.Done == nil {
		event.Done = func() {}
	}
	s.subMx.RLock()
	defer s.subMx.RUnlock()
	for _, evtCh := range s.subscribers[event.Type] {
		select {
		case evtCh <- event:
		default:
		}
	}
}

// Stop sends a TestEnd event to all subscribers, and waits up to 5s for it to
// be processed before closing all subscribed channels.
func (s *System) Stop() {
	s.notifyTestEnd()

	s.subMx.RLock()
	defer s.subMx.RUnlock()
	// To avoid a double channel close, in case there's a subscription for
	// multiple events with a single channel.
	seenChs := make(map[chan *Event]struct{})
	for _, evtSub := range s.subscribers {
		for _, evtCh := range evtSub {
			if _, ok := seenChs[evtCh]; ok {
				continue
			} else {
				seenChs[evtCh] = struct{}{}
			}
			close(evtCh)
		}
	}
}

func (s *System) notifyTestEnd() {
	doneCh := make(chan struct{})
	done := func() { doneCh <- struct{}{} }
	s.Notify(&Event{Type: TestEnd, Done: done})

	var (
		doneCount int
		timeout   = time.After(testEndTimeout)
	)
	for {
		if doneCount == len(s.subscribers[TestEnd]) {
			return
		}
		select {
		case <-doneCh:
			doneCount++
		case <-timeout:
			return
		}
	}
}
