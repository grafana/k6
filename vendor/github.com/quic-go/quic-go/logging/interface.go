// Package logging defines a logging interface for quic-go.
// This package should not be considered stable
package logging

import (
	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/qerr"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/internal/wire"
)

type (
	// A ByteCount is used to count bytes.
	ByteCount = protocol.ByteCount
	// ECN is the ECN value
	ECN = protocol.ECN
	// A ConnectionID is a QUIC Connection ID.
	ConnectionID = protocol.ConnectionID
	// An ArbitraryLenConnectionID is a QUIC Connection ID that can be up to 255 bytes long.
	ArbitraryLenConnectionID = protocol.ArbitraryLenConnectionID
	// The EncryptionLevel is the encryption level of a packet.
	EncryptionLevel = protocol.EncryptionLevel
	// The KeyPhase is the key phase of the 1-RTT keys.
	KeyPhase = protocol.KeyPhase
	// The KeyPhaseBit is the value of the key phase bit of the 1-RTT packets.
	KeyPhaseBit = protocol.KeyPhaseBit
	// The PacketNumber is the packet number of a packet.
	PacketNumber = protocol.PacketNumber
	// The Perspective is the role of a QUIC endpoint (client or server).
	Perspective = protocol.Perspective
	// A StatelessResetToken is a stateless reset token.
	StatelessResetToken = protocol.StatelessResetToken
	// The StreamID is the stream ID.
	StreamID = protocol.StreamID
	// The StreamNum is the number of the stream.
	StreamNum = protocol.StreamNum
	// The StreamType is the type of the stream (unidirectional or bidirectional).
	StreamType = protocol.StreamType
	// The VersionNumber is the QUIC version.
	VersionNumber = protocol.VersionNumber

	// The Header is the QUIC packet header, before removing header protection.
	Header = wire.Header
	// The ExtendedHeader is the QUIC Long Header packet header, after removing header protection.
	ExtendedHeader = wire.ExtendedHeader
	// The TransportParameters are QUIC transport parameters.
	TransportParameters = wire.TransportParameters
	// The PreferredAddress is the preferred address sent in the transport parameters.
	PreferredAddress = wire.PreferredAddress

	// A TransportError is a transport-level error code.
	TransportError = qerr.TransportErrorCode
	// An ApplicationError is an application-defined error code.
	ApplicationError = qerr.TransportErrorCode

	// The RTTStats contain statistics used by the congestion controller.
	RTTStats = utils.RTTStats
)

const (
	// ECNUnsupported means that no ECN value was set / received
	ECNUnsupported = protocol.ECNUnsupported
	// ECTNot is Not-ECT
	ECTNot = protocol.ECNNon
	// ECT0 is ECT(0)
	ECT0 = protocol.ECT0
	// ECT1 is ECT(1)
	ECT1 = protocol.ECT1
	// ECNCE is CE
	ECNCE = protocol.ECNCE
)

const (
	// KeyPhaseZero is key phase bit 0
	KeyPhaseZero KeyPhaseBit = protocol.KeyPhaseZero
	// KeyPhaseOne is key phase bit 1
	KeyPhaseOne KeyPhaseBit = protocol.KeyPhaseOne
)

const (
	// PerspectiveServer is used for a QUIC server
	PerspectiveServer Perspective = protocol.PerspectiveServer
	// PerspectiveClient is used for a QUIC client
	PerspectiveClient Perspective = protocol.PerspectiveClient
)

const (
	// EncryptionInitial is the Initial encryption level
	EncryptionInitial EncryptionLevel = protocol.EncryptionInitial
	// EncryptionHandshake is the Handshake encryption level
	EncryptionHandshake EncryptionLevel = protocol.EncryptionHandshake
	// Encryption1RTT is the 1-RTT encryption level
	Encryption1RTT EncryptionLevel = protocol.Encryption1RTT
	// Encryption0RTT is the 0-RTT encryption level
	Encryption0RTT EncryptionLevel = protocol.Encryption0RTT
)

const (
	// StreamTypeUni is a unidirectional stream
	StreamTypeUni = protocol.StreamTypeUni
	// StreamTypeBidi is a bidirectional stream
	StreamTypeBidi = protocol.StreamTypeBidi
)

// The ShortHeader is the QUIC Short Header packet header, after removing header protection.
type ShortHeader struct {
	DestConnectionID ConnectionID
	PacketNumber     PacketNumber
	PacketNumberLen  protocol.PacketNumberLen
	KeyPhase         KeyPhaseBit
}
