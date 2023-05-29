package event

import (
	"fmt"
	"sync"
	"time"
)

type Type string

const (
	Init      Type = "init"
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
	subIDCount  uint64
	subscribers map[Type]map[uint64]chan *Event
}

func NewEventSystem() *System {
	return &System{
		subscribers: make(map[Type]map[uint64]chan *Event),
	}
}

func (s *System) Subscribe(events ...Type) (subID uint64, eventsCh <-chan *Event) {
	s.subMx.Lock()
	defer s.subMx.Unlock()
	s.subIDCount++
	subID = s.subIDCount
	evtCh := make(chan *Event)
	for _, evt := range events {
		if s.subscribers[evt] == nil {
			s.subscribers[evt] = make(map[uint64]chan *Event)
		}
		s.subscribers[evt][subID] = evtCh
	}
	return subID, evtCh
}

func (s *System) Notify(event *Event, wait time.Duration) {
	s.subMx.RLock()
	defer s.subMx.RUnlock()
	totalSubs := len(s.subscribers[event.Type])
	if totalSubs == 0 {
		return
	}

	if event.Done == nil {
		event.Done = func() {}
	}
	if wait > 0 {
		defer s.waitForDone(event, wait, totalSubs)
	}

	for _, evtCh := range s.subscribers[event.Type] {
		select {
		case evtCh <- event:
		default:
		}
	}
}

// Unsubscribe the subscription with ID subID.
func (s *System) Unsubscribe(subID uint64) {
	s.subMx.Lock()
	defer s.subMx.Unlock()
	for _, sub := range s.subscribers {
		if evtCh, ok := sub[subID]; ok {
			close(evtCh)
			delete(sub, subID)
		}
	}
}

// UnsubscribeAll removes all subscriptions.
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
			delete(sub, subID)
		}
	}
}

func (s *System) waitForDone(evt *Event, wait time.Duration, expDoneCount int) {
	origDoneFn := evt.Done
	doneCh := make(chan struct{})
	doneFn := func() {
		origDoneFn()
		doneCh <- struct{}{}
	}
	evt.Done = doneFn

	var (
		doneCount int
		timeout   = time.After(wait)
	)
	for {
		if doneCount == expDoneCount {
			fmt.Printf(">>> received all %d done signals\n", doneCount)
			return
		}
		select {
		case <-doneCh:
			fmt.Printf(">>> received 1 done signal\n")
			doneCount++
		case <-timeout:
			fmt.Printf(">>> timed out after waiting %s for done signals\n", wait)
			return
		}
	}
}
