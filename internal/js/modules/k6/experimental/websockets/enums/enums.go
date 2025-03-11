package enums

// ReadyState is websocket specification's readystate
type ReadyState uint8

// Events represent the events that can be sent to the client
// https://dom.spec.whatwg.org/#event
const (
	// EVENT_OPEN is the event name for the open event
	EVENT_OPEN = "open"
	// EVENT_CLOSE is the event name for the close event
	EVENT_CLOSE = "close"
	// EVENT_MESSAGE is the event name for the message event
	EVENT_MESSAGE = "message"
	// EVENT_ERROR is the event name for the error event
	EVENT_ERROR = "error"
	// EVENT_PING is the event name for the ping event
	EVENT_PING = "ping"
	// EVENT_PONG is the event name for the pong event
	EVENT_PONG = "pong"
)

// ReadyState describes the possible states of a WebSocket connection.
const (
	// STATE_CONNECTING is the state while the web socket is connecting
	STATE_CONNECTING ReadyState = iota
	// STATE_OPEN is the state after the websocket is established and before it starts closing
	STATE_OPEN
	// STATE_CLOSING is while the websocket is closing but is *not* closed yet
	STATE_CLOSING
	// STATE_CLOSED is when the websocket is finally closed
	STATE_CLOSED
)

// BinaryType describes the possible types of binary data that can be
// transmitted over a Websocket connection.
const (
	BINARY_BLOB         = "blob"
	BINARY_ARRAY_BUFFER = "arraybuffer"
)

// WebSocket message types, as defined in RFC 6455, section 11.8.
const (
	//  The message is a text message. The text message payload is
	//  interpreted as UTF-8 encodedtext data.
	MESSAGE_TEXT = 1

	// The message is a binary message.
	MESSAGE_BINARY = 2

	// The message is a close control message. The optional message
	// payload contains a numeric code and a text reason.
	MESSAGE_CLOSE = 8

	// The message is a ping control message. The optional message
	// payload is UTF-8 encoded text.
	MESSAGE_PING = 9

	// The message is a pong control message. The optional message
	// payload is UTF-8 encoded text.
	MESSAGE_PONG = 10
)

// CompressionAlgorithm describes the possible compression algorithms.
const (
	// Deflate compression algorithm.
	// k6 supports only this compression algorithm.
	ALGORITHM_DEFLATE = "deflate"
)

func GetEventsName() map[string]any {
	return map[string]any{
		"Open":    EVENT_OPEN,
		"Close":   EVENT_CLOSE,
		"Error":   EVENT_ERROR,
		"Message": EVENT_MESSAGE,
		"Ping":    EVENT_PING,
		"Pong":    EVENT_PONG,
	}
}

func GetReadyState() map[string]any {
	return map[string]any{
		"Connecting": STATE_CONNECTING,
		"Open":       STATE_OPEN,
		"Closing":    STATE_CLOSING,
		"Closed":     STATE_CLOSED,
	}
}

func GetBinaryType() map[string]any {
	return map[string]any{
		"Blob":        BINARY_BLOB,
		"ArrayBuffer": BINARY_ARRAY_BUFFER,
	}
}

func GetMessageType() map[string]any {
	return map[string]any{
		"Text":        MESSAGE_TEXT,
		"Binary":      MESSAGE_BINARY,
		"Close":       MESSAGE_CLOSE,
		"PingMessage": MESSAGE_PING,
		"PongMessage": MESSAGE_PONG,
	}
}

func GetCompressionAlgorithm() map[string]any {
	return map[string]any{
		"Deflate": ALGORITHM_DEFLATE,
	}
}
