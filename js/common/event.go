package common

import "go.k6.io/k6/internal/event"

// Events are the event subscriber interfaces for the global event system, and
// the local (per-VU) event system.
type Events struct {
	Global, Local event.Subscriber
}
