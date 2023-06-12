package tracing

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"time"
)

const (
	// Being 075 the ASCII code for 'K' :)
	k6Prefix = 0o756

	// To ingest and process the related spans in k6 Cloud.
	k6CloudCode = 12

	// To not ingest and process the related spans, b/c they are part of a non-cloud run.
	k6LocalCode = 33

	// metadataTraceIDKeyName is the key name of the traceID in the output metadata.
	metadataTraceIDKeyName = "trace_id"

	// traceIDEncodedSize is the size of the encoded traceID.
	traceIDEncodedSize = 16
)

// newTraceID generates a new hexadecimal-encoded trace ID as defined by the [W3C specification].
//
// `prefix` is the first 2 bytes of the trace ID, and is used to identify the
// vendor of the trace ID. `code` is the third byte of the trace ID, and is
// used to identify the type of the trace ID. `t` is the time at which the trace
// ID was generated. `randSource` is the source of randomness used to fill the rest
// of bytes of the trace ID.
//
// [W3C specification]: https://www.w3.org/TR/trace-context/#trace-id
func newTraceID(prefix int16, code int8, t time.Time, randSource io.Reader) (string, error) {
	if prefix != k6Prefix {
		return "", fmt.Errorf("invalid prefix 0o%o, expected 0o%o", prefix, k6Prefix)
	}

	if (code != k6CloudCode) && (code != k6LocalCode) {
		return "", fmt.Errorf("invalid code 0o%d, accepted values are 0o%d and 0o%d", code, k6CloudCode, k6LocalCode)
	}

	// Encode The trace ID into a binary buffer.
	buf := make([]byte, traceIDEncodedSize)
	n := binary.PutVarint(buf, int64(prefix))
	n += binary.PutVarint(buf[n:], int64(code))
	n += binary.PutVarint(buf[n:], t.UnixNano())

	// Calculate the number of random bytes needed.
	randomBytesSize := traceIDEncodedSize - n

	// Generate the random bytes.
	randomness := make([]byte, randomBytesSize)
	err := binary.Read(randSource, binary.BigEndian, randomness)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes from os; reason: %w", err)
	}

	// Combine the values and random bytes to form the encoded trace ID buffer.
	buf = append(buf[:n], randomness...)

	return hex.EncodeToString(buf), nil
}
