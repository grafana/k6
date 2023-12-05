package grpc

import (
	"fmt"

	"github.com/dop251/goja"
)

const (
	eventData   = "data"
	eventError  = "error"
	eventEnd    = "end"
	eventStatus = "status"
)

// eventListeners keeps track of the eventListeners for each event type
type eventListeners struct {
	data   *eventListener
	error  *eventListener
	end    *eventListener
	status *eventListener
}

// eventListener keeps listeners of a certain type
type eventListener struct {
	eventType string

	// this return goja.value *and* error in order to return error on exception instead of panic
	// https://pkg.go.dev/github.com/dop251/goja#hdr-Functions
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

// getTypes return event listener of a certain type
func (l *eventListeners) getType(t string) *eventListener {
	switch t {
	case eventData:
		return l.data
	case eventError:
		return l.error
	case eventStatus:
		return l.status
	case eventEnd:
		return l.end
	default:
		return nil
	}
}

// add adds a listener to the listeners
func (l *eventListeners) add(t string, f func(goja.Value) (goja.Value, error)) error {
	list := l.getType(t)

	if list == nil {
		return fmt.Errorf("unknown GRPC stream's event type: %s", t)
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

	return list.list
}

func newEventListeners() *eventListeners {
	return &eventListeners{
		data:   newListener(eventData),
		error:  newListener(eventError),
		status: newListener(eventStatus),
		end:    newListener(eventEnd),
	}
}
