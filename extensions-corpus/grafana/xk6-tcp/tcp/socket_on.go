package tcp

import (
	"fmt"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/metrics"
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
		s.log.WithField("event", event).Warn("Unknown event type")

		return
	}

	if _, ok := s.handlers.Load(event); ok {
		s.log.WithField("event", event).Warn("Event handler already registered, overriding")
	}

	s.log.WithField("event", event).Debug("Event handler registered")

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

	s.log.WithField("event", event).Debug("Queuing event handler")

	// Queue synchronously so the caller's event order is preserved across goroutines.
	select {
	case s.callChan <- func() error {
		if cleanup != nil {
			defer cleanup()
		}

		s.log.WithField("event", event).Debug("Firing event handler")

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
		s.log.WithField("event", event).Debug("Context cancelled, skipping event")

		if cleanup != nil {
			cleanup()
		}

		return false
	}
}

func (s *socket) handleError(err error, method string, ts *metrics.TagSet) error {
	s.log.WithField("error", err).WithField("method", method).Error("Handling TCP error")

	s.addErrorMetrics(ts)

	wrapped := newTCPError(err, method)

	if s.fire("error", wrapped) {
		return nil
	}

	return wrapped
}

func (s *socket) rejectWithTCPError(reject func(any), err error, method string, ts *metrics.TagSet) {
	tcpErr := s.handleError(err, method, ts)
	if tcpErr == nil {
		tcpErr = newTCPError(err, method)
	}

	reject(tcpErr)
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
