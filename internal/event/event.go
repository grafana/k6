package event

// Event is the emitted object sent to all subscribers of its type.
// The subscriber should call its Done method when finished processing
// to notify the emitter, though this is not required for all events.
type Event struct {
	Type Type
	Data any
	Done func()
}
