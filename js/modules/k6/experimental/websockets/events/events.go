// Package events represent the events that can be sent to the client
// https://dom.spec.whatwg.org/#event
package events

const (
	// OPEN is the event name for the open event
	OPEN = "open"
	// CLOSE is the event name for the close event
	CLOSE = "close"
	// MESSAGE is the event name for the message event
	MESSAGE = "message"
	// ERROR is the event name for the error event
	ERROR = "error"
	// PING is the event name for the ping event
	PING = "ping"
	// PONG is the event name for the pong event
	PONG = "pong"
)
