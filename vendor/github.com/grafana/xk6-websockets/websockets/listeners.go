package websockets

import (
	"fmt"

	"github.com/dop251/goja"
	"github.com/grafana/xk6-websockets/websockets/events"
)

// eventListeners keeps track of the eventListeners for each event type
type eventListeners struct {
	open    *eventListener
	message *eventListener
	error   *eventListener
	close   *eventListener
	ping    *eventListener
	pong    *eventListener
}

func newEventListeners() *eventListeners {
	return &eventListeners{
		open:    newListener(events.OPEN),
		message: newListener(events.MESSAGE),
		error:   newListener(events.ERROR),
		close:   newListener(events.CLOSE),
		ping:    newListener(events.PING),
		pong:    newListener(events.PONG),
	}
}

// eventListener represents a tuple of listeners of a certain type
// property on represents the eventListener that serves for the on* properties, like onopen, onmessage, etc.
// property list keeps any other listeners that were added with addEventListener
type eventListener struct {
	eventType string

	// this return goja.value *and* error in order to return error on exception instead of panic
	// https://pkg.go.dev/github.com/dop251/goja#hdr-Functions
	on   func(goja.Value) (goja.Value, error)
	list []func(goja.Value) (goja.Value, error)
}

// newListener creates a new listener of a certain type
func newListener(eventType string) *eventListener {
	return &eventListener{
		eventType: eventType,
	}
}

// add adds a listener to the listener list
func (l *eventListener) add(fn func(goja.Value) (goja.Value, error)) {
	l.list = append(l.list, fn)
}

// setOn sets a listener for the on* properties, like onopen, onmessage, etc.
func (l *eventListener) setOn(fn func(goja.Value) (goja.Value, error)) {
	l.on = fn
}

// getOn returns the on* property for a certain event type
func (l *eventListener) getOn() func(goja.Value) (goja.Value, error) {
	return l.on
}

// return all possible listeners for a certain event type
func (l *eventListener) all() []func(goja.Value) (goja.Value, error) {
	if l.on == nil {
		return l.list
	}

	return append([]func(goja.Value) (goja.Value, error){l.on}, l.list...)
}

// getTypes return event listener of a certain type
func (l *eventListeners) getType(t string) *eventListener {
	switch t {
	case events.OPEN:
		return l.open
	case events.MESSAGE:
		return l.message
	case events.ERROR:
		return l.error
	case events.CLOSE:
		return l.close
	case events.PING:
		return l.ping
	case events.PONG:
		return l.pong
	default:
		return nil
	}
}

// add adds a listener to the listeners
func (l *eventListeners) add(t string, f func(goja.Value) (goja.Value, error)) error {
	list := l.getType(t)

	if list == nil {
		return fmt.Errorf("unknown event type: %s", t)
	}

	list.add(f)

	return nil
}

// all returns all possible listeners for a certain event type or an empty array
func (l *eventListeners) all(t string) []func(goja.Value) (goja.Value, error) {
	list := l.getType(t)

	if list == nil {
		return []func(goja.Value) (goja.Value, error){}
	}

	return list.all()
}
