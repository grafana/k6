package websockets

import (
	"fmt"

	"github.com/grafana/sobek"
	"go.k6.io/k6/internal/js/modules/k6/experimental/websockets/enums"
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
		open:    newListener(enums.EVENT_OPEN),
		message: newListener(enums.EVENT_MESSAGE),
		error:   newListener(enums.EVENT_ERROR),
		close:   newListener(enums.EVENT_CLOSE),
		ping:    newListener(enums.EVENT_PING),
		pong:    newListener(enums.EVENT_PONG),
	}
}

// eventListener represents a tuple of listeners of a certain type
// property on represents the eventListener that serves for the on* properties, like onopen, onmessage, etc.
// property list keeps any other listeners that were added with addEventListener
type eventListener struct {
	eventType string

	// this return sobek.value *and* error in order to return error on exception instead of panic
	// https://pkg.go.dev/github.com/dop251/goja#hdr-Functions
	on   func(sobek.Value) (sobek.Value, error)
	list []func(sobek.Value) (sobek.Value, error)
}

// newListener creates a new listener of a certain type
func newListener(eventType string) *eventListener {
	return &eventListener{
		eventType: eventType,
	}
}

// add adds a listener to the listener list
func (l *eventListener) add(fn func(sobek.Value) (sobek.Value, error)) {
	l.list = append(l.list, fn)
}

// setOn sets a listener for the on* properties, like onopen, onmessage, etc.
func (l *eventListener) setOn(fn func(sobek.Value) (sobek.Value, error)) {
	l.on = fn
}

// getOn returns the on* property for a certain event type
func (l *eventListener) getOn() func(sobek.Value) (sobek.Value, error) {
	return l.on
}

// return all possible listeners for a certain event type
func (l *eventListener) all() []func(sobek.Value) (sobek.Value, error) {
	if l.on == nil {
		return l.list
	}

	return append([]func(sobek.Value) (sobek.Value, error){l.on}, l.list...)
}

// getTypes return event listener of a certain type
func (l *eventListeners) getType(t string) *eventListener {
	switch t {
	case enums.EVENT_OPEN:
		return l.open
	case enums.EVENT_MESSAGE:
		return l.message
	case enums.EVENT_ERROR:
		return l.error
	case enums.EVENT_CLOSE:
		return l.close
	case enums.EVENT_PING:
		return l.ping
	case enums.EVENT_PONG:
		return l.pong
	default:
		return nil
	}
}

// add adds a listener to the listeners
func (l *eventListeners) add(t string, f func(sobek.Value) (sobek.Value, error)) error {
	list := l.getType(t)

	if list == nil {
		return fmt.Errorf("unknown event type: %s", t)
	}

	list.add(f)

	return nil
}

// all returns all possible listeners for a certain event type or an empty array
func (l *eventListeners) all(t string) []func(sobek.Value) (sobek.Value, error) {
	list := l.getType(t)

	if list == nil {
		return []func(sobek.Value) (sobek.Value, error){}
	}

	return list.all()
}
