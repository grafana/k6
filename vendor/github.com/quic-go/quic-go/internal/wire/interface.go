package wire

import (
	"github.com/quic-go/quic-go/internal/protocol"
)

// A Frame in QUIC
type Frame interface {
	Append(b []byte, version protocol.VersionNumber) ([]byte, error)
	Length(version protocol.VersionNumber) protocol.ByteCount
}

// A FrameParser parses QUIC frames, one by one.
type FrameParser interface {
	ParseNext([]byte, protocol.EncryptionLevel, protocol.VersionNumber) (int, Frame, error)
	SetAckDelayExponent(uint8)
}
