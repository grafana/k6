package wire

import (
	"github.com/quic-go/quic-go/internal/protocol"
)

// A HandshakeDoneFrame is a HANDSHAKE_DONE frame
type HandshakeDoneFrame struct{}

func (f *HandshakeDoneFrame) Append(b []byte, _ protocol.VersionNumber) ([]byte, error) {
	return append(b, handshakeDoneFrameType), nil
}

// Length of a written frame
func (f *HandshakeDoneFrame) Length(_ protocol.VersionNumber) protocol.ByteCount {
	return 1
}
