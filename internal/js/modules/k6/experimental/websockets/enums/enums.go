// Package enums is responsible for defining the enums available in the websocket module
package enums

// ReadyState is websocket specification's readystate
type ReadyState uint8

// Events represent the events that can be sent to the client
// https://dom.spec.whatwg.org/#event
const (
	// EventOpen is the event name for the open event
	EventOpen = "open"
	// EventClose is the event name for the close event
	EventClose = "close"
	// EventMessage is the event name for the message event
	EventMessage = "message"
	// EventError is the event name for the error event
	EventError = "error"
	// EventPing is the event name for the ping event
	EventPing = "ping"
	// EventPong is the event name for the pong event
	EventPong = "pong"
)

// ReadyState describes the possible states of a WebSocket connection.
const (
	// StateConnecting is the state while the web socket is connecting
	StateConnecting ReadyState = iota
	// StateOpen is the state after the websocket is established and before it starts closing
	StateOpen
	// StateClosing is while the websocket is closing but is *not* closed yet
	StateClosing
	// StateClosed is when the websocket is finally closed
	StateClosed
)

// BinaryType describes the possible types of binary data that can be
// transmitted over a Websocket connection.
const (
	BinaryBlob        = "blob"
	BinaryArrayBuffer = "arraybuffer"
)

// WebSocket message types, as defined in RFC 6455, section 11.8.
const (
	//  The message is a text message. The text message payload is
	//  interpreted as UTF-8 encodedtext data.
	MessageText = 1

	// The message is a binary message.
	MessageBinary = 2

	// The message is a close control message. The optional message
	// payload contains a numeric code and a text reason.
	MessageClose = 8

	// The message is a ping control message. The optional message
	// payload is UTF-8 encoded text.
	MessagePing = 9

	// The message is a pong control message. The optional message
	// payload is UTF-8 encoded text.
	MessagePong = 10
)

// CompressionAlgorithm describes the possible compression algorithms.
const (
	// Deflate compression algorithm.
	// k6 supports only this compression algorithm.
	AlgorithmDeflate = "deflate"
)

// GetEventsName maps field names to enum value
func GetEventsName() map[string]any {
	return map[string]any{
		"Open":    EventOpen,
		"Close":   EventClose,
		"Error":   EventError,
		"Message": EventMessage,
		"Ping":    EventPing,
		"Pong":    EventPong,
	}
}

// GetReadyState maps field names to enum value
func GetReadyState() map[string]any {
	return map[string]any{
		"Connecting": StateConnecting,
		"Open":       StateOpen,
		"Closing":    StateClosing,
		"Closed":     StateClosed,
	}
}

// GetBinaryType maps field names to enum value
func GetBinaryType() map[string]any {
	return map[string]any{
		"Blob":        BinaryBlob,
		"ArrayBuffer": BinaryArrayBuffer,
	}
}

// GetMessageType maps field names to enum value
func GetMessageType() map[string]any {
	return map[string]any{
		"Text":        MessageText,
		"Binary":      MessageBinary,
		"Close":       MessageClose,
		"PingMessage": MessagePing,
		"PongMessage": MessagePong,
	}
}

// GetCompressionAlgorithm maps field names to enum value
func GetCompressionAlgorithm() map[string]any {
	return map[string]any{
		"Deflate": AlgorithmDeflate,
	}
}
