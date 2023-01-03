package tracing

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
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
// used by w3c, b3 and jaeger propagators. See [Considerations for trace-id field generation]
// for more information.
//
// [W3c specification]: https://www.w3.org/TR/trace-context/#trace-id
// [Considerations for trace-id field generation]: https://www.w3.org/TR/trace-context/#considerations-for-trace-id-field-generation
//
//nolint:lll
type TraceID struct {
	Prefix            int16
	Code              int8
	UnixTimestampNano uint64
}

// NewTraceID returns a new TraceID with the given prefix, code and unix timestamp in nanoseconds.
func NewTraceID(prefix int16, code int8, unixTimestampNano uint64) TraceID {
	return TraceID{
		Prefix:            prefix,
		Code:              code,
		UnixTimestampNano: unixTimestampNano,
	}
}

// Encode encodes the TraceID into a hex string and a byte slice.
func (t TraceID) Encode() (string, []byte, error) {
	if !t.isValid() {
		return "", nil, fmt.Errorf("failed to encode traceID: %v", t)
	}

	buf := make([]byte, 16)

	n := binary.PutVarint(buf, int64(t.Prefix))
	n += binary.PutVarint(buf[n:], int64(t.Code))
	n += binary.PutUvarint(buf[n:], t.UnixTimestampNano)

	randomness := make([]byte, 16-n)
	err := binary.Read(rand.Reader, binary.BigEndian, randomness)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read random bytes from os; reason: %w", err)
	}

	buf = append(buf[:n], randomness...)
	hx := hex.EncodeToString(buf)

	return hx, buf, nil
}

func (t TraceID) isValid() bool {
	var (
		isk6Prefix = t.Prefix == k6Prefix
		isk6Cloud  = t.Code == k6CloudCode
		isk6Local  = t.Code == k6LocalCode
	)

	return isk6Prefix && (isk6Cloud || isk6Local)
}
