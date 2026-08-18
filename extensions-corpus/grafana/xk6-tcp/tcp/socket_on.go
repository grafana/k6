package tcp

import (
	"fmt"

	"github.com/grafana/sobek"
	extensionapi "go.k6.io/k6-extension-api"
)

var events = map[string]struct{}{ //nolint:gochecknoglobals
	"connect": {},
	"data":    {},
	"close":   {},
	"error":   {},
	"timeout": {},
}

func (s *socket) on(event string, handler sobek.Callable) {
	if _, ok := events[event]; !ok {
		s.log.Warn("Unknown event type", "event", event)

		return
	}

	if _, ok := s.handlers.Load(event); ok {
		s.log.Warn("Event handler already registered, overriding", "event", event)
	}

	s.log.Debug("Event handler registered", "event", event)

	s.handlers.Store(event, handler)
}

// fire queues an event to be fired in the VU's event loop.
// Args are converted to sobek.Value inside the event loop to avoid race conditions.
func (s *socket) fire(event string, args ...any) bool {
	return s.fireAndCleanup(nil, event, args...)
}

// fireAndCleanup fires an event with a cleanup callback.
// Args are converted to sobek.Value inside the event loop to avoid race conditions.
func (s *socket) fireAndCleanup(cleanup func(), event string, args ...any) bool {
	f, ok := s.handlers.Load(event)
	if !ok {
		if cleanup != nil {
			cleanup()
		}

		return false
	}

	fn, ok := f.(sobek.Callable)
	if !ok {
		if cleanup != nil {
			cleanup()
		}

		return false
	}

	s.log.Debug("Queuing event handler", "event", event)

	// Queue synchronously so the caller's event order is preserved across goroutines.
	select {
	case s.callChan <- func() error {
		if cleanup != nil {
			defer cleanup()
		}

		s.log.Debug("Firing event handler", "event", event)

		// Convert raw Go values to sobek.Value in the event loop
		sobekArgs := make([]sobek.Value, len(args))
		for i, arg := range args {
			sobekArgs[i] = s.vu.Runtime().ToValue(arg)
		}

		_, err := fn(sobek.Undefined(), sobekArgs...)

		return err
	}:
		return true

	case <-s.vu.Context().Done():
		s.log.Debug("Context cancelled, skipping event", "event", event)

		if cleanup != nil {
			cleanup()
		}

		return false
	}
}

func (s *socket) handleError(err error, method string, tags extensionapi.Tags) error {
	s.log.Error("Handling TCP error", "error", err, "method", method)

	s.addErrorMetrics(tags)

	wrapped := newTCPError(err, method)

	if s.fire("error", wrapped) {
		return nil
	}

	return wrapped
}

func (s *socket) rejectWithTCPError(
	reject extensionapi.PromiseResolver, err error, method string, tags extensionapi.Tags,
) {
	tcpErr := s.handleError(err, method, tags)
	if tcpErr == nil {
		tcpErr = newTCPError(err, method)
	}

	reject.Reject(tcpErr)
}

// TCPError represents an error that occurred during a TCP operation.
type TCPError struct { //nolint:revive
	Name    string
	Method  string
	Message string
}

func newTCPError(err error, method string) *TCPError {
	return &TCPError{
		Name:    "TCPError",
		Method:  method,
		Message: err.Error(),
	}
}

func (e *TCPError) Error() string {
	return fmt.Sprintf("TCP error during %s: %v", e.Method, e.Message)
}
