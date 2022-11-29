package websockets

import (
	"fmt"
	"sync"

	"github.com/dop251/goja"
	"github.com/grafana/xk6-websockets/websockets/events"
)

// eventListeners keeps track of the eventListeners for each event type
type eventListeners struct {
	list map[string]*eventListener
}

func newEventListeners() *eventListeners {
	return &eventListeners{
		list: map[string]*eventListener{
			events.OPEN:    newListener(events.OPEN),
			events.MESSAGE: newListener(events.MESSAGE),
			events.ERROR:   newListener(events.ERROR),
			events.CLOSE:   newListener(events.CLOSE),
			events.PING:    newListener(events.PING),
			events.PONG:    newListener(events.PONG),
		},
	}
}

// eventListener represents a tuple of listeners of a certain type
// property on represents the eventListener that serves for the on* properties, like onopen, onmessage, etc.
// property list keeps any other listeners that were added with addEventListener
type eventListener struct {
	*sync.Mutex
	eventType string

	// this return goja.value *and* error in order to return error on exception instead of panic
	// https://pkg.go.dev/github.com/dop251/goja#hdr-Functions
	on   func(goja.Value) (goja.Value, error)
	list []func(goja.Value) (goja.Value, error)
}

// newListener creates a new listener of a certain type
func newListener(eventType string) *eventListener {
	return &eventListener{
		Mutex:     &sync.Mutex{},
		eventType: eventType,
	}
}

// add adds a listener to the listener list
func (l *eventListener) add(fn func(goja.Value) (goja.Value, error)) {
	l.list = append(l.list, fn)
}

// setOn sets a listener for the on* properties, like onopen, onmessage, etc.
func (l *eventListener) setOn(fn func(goja.Value) (goja.Value, error)) {
	l.Lock()
	l.on = fn
	l.Unlock()
}

// getOn returns the on* property for a certain event type
func (l *eventListener) getOn() func(goja.Value) (goja.Value, error) {
	l.Lock()
	defer l.Unlock()

	return l.on
}

// return all possible listeners for a certain event type
func (l *eventListener) all() []func(goja.Value) (goja.Value, error) {
	l.Lock()
	defer l.Unlock()

	if l.on == nil {
		return l.list
	}

	return append([]func(goja.Value) (goja.Value, error){l.on}, l.list...)
}

// add adds a listener to the listeners
func (l *eventListeners) add(t string, f func(goja.Value) (goja.Value, error)) error {
	if _, ok := l.list[t]; !ok {
		return fmt.Errorf("unknown event type: %s", t)
	}

	l.list[t].add(f)

	return nil
}

func (l *eventListeners) all(t string) []func(goja.Value) (goja.Value, error) {
	if _, ok := l.list[t]; !ok {
		return []func(goja.Value) (goja.Value, error){}
	}

	return l.list[t].all()
}
