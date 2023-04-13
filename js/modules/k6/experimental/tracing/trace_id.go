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
)

// TraceID represents a trace-id as defined by the [W3c specification], and
// used by w3c, b3 and jaeger propagators. See Considerations for trace-id field [generation]
// for more information.
//
// [W3c specification]: https://www.w3.org/TR/trace-context/#trace-id
// [generation]: https://www.w3.org/TR/trace-context/#considerations-for-trace-id-field-generation
type TraceID struct {
	// Prefix is the first 2 bytes of the trace-id, and is used to identify the
	// vendor of the trace-id.
	Prefix int16

	// Code is the third byte of the trace-id, and is used to identify the
	// vendor's specific trace-id format.
	Code int8

	// Time is the time at which the trace-id was generated.
	//
	// The time component is used as a source of randomness, and to ensure
	// uniqueness of the trace-id.
	//
	// When encoded, it should be in a format occupying the last 8 bytes of
	// the trace-id, and should ideally be encoded as nanoseconds.
	Time time.Time

	// randSource holds the randomness source to use when encoding the
	// trace-id. The `rand.Reader` should be your default pick. But
	// you can replace it with a different source for testing purposes.
	randSource io.Reader
}

// Encode encodes the TraceID into a hex string.
//
// The trace id is first encoded as a 16 bytes sequence, as follows:
// 1. Up to 2 bytes are encoded as the Prefix
// 2. The third byte is the Code.
// 3. Up to the following 8 bytes are UnixTimestampNano.
// 4. The remaining bytes are filled with random bytes.
//
// The resulting 16 bytes sequence is then encoded as a hex string.
func (t TraceID) Encode() (string, error) {
	if !t.isValid() {
		return "", fmt.Errorf("failed to encode traceID: %v", t)
	}

	// TraceID is specified to be 16 bytes long.
	buf := make([]byte, 16)

	// The `PutVarint` and `PutUvarint` functions encode the given value into
	// the provided buffer, and return the number of bytes written. Thus, it
	// allows us to keep track of the number of bytes written, as we go, and
	// to pack the values to use as less space as possible.
	n := binary.PutVarint(buf, int64(t.Prefix))
	n += binary.PutVarint(buf[n:], int64(t.Code))
	n += binary.PutVarint(buf[n:], t.Time.UnixNano())

	// The rest of the space in the 16 bytes buffer, equivalent to the number
	// of available bytes left after writing the prefix, code and timestamp (index n)
	// is filled with random bytes.
	randomness := make([]byte, 16-n)
	err := binary.Read(t.randSource, binary.BigEndian, randomness)
	if err != nil {
		return "", fmt.Errorf("failed to read random bytes from os; reason: %w", err)
	}

	buf = append(buf[:n], randomness...)
	hx := hex.EncodeToString(buf)

	return hx, nil
}

func (t TraceID) isValid() bool {
	var (
		isk6Prefix = t.Prefix == k6Prefix
		isk6Cloud  = t.Code == k6CloudCode
		isk6Local  = t.Code == k6LocalCode
	)

	return isk6Prefix && (isk6Cloud || isk6Local)
}
