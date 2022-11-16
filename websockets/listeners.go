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
}

func newEventListeners() *eventListeners {
	return &eventListeners{
		open:    newListener(events.OPEN),
		message: newListener(events.MESSAGE),
		error:   newListener(events.ERROR),
		close:   newListener(events.CLOSE),
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
	// TODO: tread safe?
	l.on = fn
}

// return all possible listeners for a certain event type
func (l *eventListener) all() []func(goja.Value) (goja.Value, error) {
	if l.on == nil {
		return l.list
	}

	return append([]func(goja.Value) (goja.Value, error){l.on}, l.list...)
}

// add adds a listener to the listeners
func (l *eventListeners) add(t string, f func(goja.Value) (goja.Value, error)) error {
	switch t {
	case events.OPEN:
		l.open.add(f)
	case events.MESSAGE:
		l.message.add(f)
	case events.ERROR:
		l.error.add(f)
	case events.CLOSE:
		l.close.add(f)
	default:
		return fmt.Errorf("unknown event type: %s", t)
	}
	return nil
}

// Message returns all event listeners that listen for the message event
func (l *eventListeners) Message() []func(goja.Value) (goja.Value, error) {
	return l.message.all()
}

// Error returns all event listeners that listen for the error event
func (l *eventListeners) Error() []func(goja.Value) (goja.Value, error) {
	return l.error.all()
}

// Close returns all event listeners that listen for the close event
func (l *eventListeners) Close() []func(goja.Value) (goja.Value, error) {
	return l.close.all()
}

// Open returns all event listeners that listen for the open event
func (l *eventListeners) Open() []func(goja.Value) (goja.Value, error) {
	return l.open.all()
}
