package qlog

import (
	"fmt"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/logging"

	"github.com/francoispqt/gojay"
)

func getPacketTypeFromEncryptionLevel(encLevel protocol.EncryptionLevel) logging.PacketType {
	switch encLevel {
	case protocol.EncryptionInitial:
		return logging.PacketTypeInitial
	case protocol.EncryptionHandshake:
		return logging.PacketTypeHandshake
	case protocol.Encryption0RTT:
		return logging.PacketType0RTT
	case protocol.Encryption1RTT:
		return logging.PacketType1RTT
	default:
		panic("unknown encryption level")
	}
}

type token struct {
	Raw []byte
}

var _ gojay.MarshalerJSONObject = &token{}

func (t token) IsNil() bool { return false }
func (t token) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("data", fmt.Sprintf("%x", t.Raw))
}

// PacketHeader is a QUIC packet header.
// TODO: make this a long header
type packetHeader struct {
	PacketType logging.PacketType

	KeyPhaseBit  logging.KeyPhaseBit
	PacketNumber logging.PacketNumber

	Version          logging.VersionNumber
	SrcConnectionID  logging.ConnectionID
	DestConnectionID logging.ConnectionID

	Token *token
}

func transformHeader(hdr *logging.Header) *packetHeader {
	h := &packetHeader{
		PacketType:       logging.PacketTypeFromHeader(hdr),
		SrcConnectionID:  hdr.SrcConnectionID,
		DestConnectionID: hdr.DestConnectionID,
		Version:          hdr.Version,
	}
	if len(hdr.Token) > 0 {
		h.Token = &token{Raw: hdr.Token}
	}
	return h
}

func transformLongHeader(hdr *logging.ExtendedHeader) *packetHeader {
	h := transformHeader(&hdr.Header)
	h.PacketNumber = hdr.PacketNumber
	h.KeyPhaseBit = hdr.KeyPhase
	return h
}

func (h packetHeader) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("packet_type", packetType(h.PacketType).String())
	if h.PacketType != logging.PacketTypeRetry {
		enc.Int64Key("packet_number", int64(h.PacketNumber))
	}
	if h.Version != 0 {
		enc.StringKey("version", versionNumber(h.Version).String())
	}
	if h.PacketType != logging.PacketType1RTT {
		enc.IntKey("scil", h.SrcConnectionID.Len())
		if h.SrcConnectionID.Len() > 0 {
			enc.StringKey("scid", h.SrcConnectionID.String())
		}
	}
	enc.IntKey("dcil", h.DestConnectionID.Len())
	if h.DestConnectionID.Len() > 0 {
		enc.StringKey("dcid", h.DestConnectionID.String())
	}
	if h.KeyPhaseBit == logging.KeyPhaseZero || h.KeyPhaseBit == logging.KeyPhaseOne {
		enc.StringKey("key_phase_bit", h.KeyPhaseBit.String())
	}
	if h.Token != nil {
		enc.ObjectKey("token", h.Token)
	}
}

type packetHeaderVersionNegotiation struct {
	SrcConnectionID  logging.ArbitraryLenConnectionID
	DestConnectionID logging.ArbitraryLenConnectionID
}

func (h packetHeaderVersionNegotiation) IsNil() bool { return false }
func (h packetHeaderVersionNegotiation) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("packet_type", "version_negotiation")
	enc.IntKey("scil", h.SrcConnectionID.Len())
	enc.StringKey("scid", h.SrcConnectionID.String())
	enc.IntKey("dcil", h.DestConnectionID.Len())
	enc.StringKey("dcid", h.DestConnectionID.String())
}

// a minimal header that only outputs the packet type
type packetHeaderWithType struct {
	PacketType logging.PacketType
}

func (h packetHeaderWithType) IsNil() bool { return false }
func (h packetHeaderWithType) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("packet_type", packetType(h.PacketType).String())
}

// a minimal header that only outputs the packet type
type packetHeaderWithTypeAndPacketNumber struct {
	PacketType   logging.PacketType
	PacketNumber logging.PacketNumber
}

func (h packetHeaderWithTypeAndPacketNumber) IsNil() bool { return false }
func (h packetHeaderWithTypeAndPacketNumber) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("packet_type", packetType(h.PacketType).String())
	enc.Int64Key("packet_number", int64(h.PacketNumber))
}

type shortHeader struct {
	DestConnectionID logging.ConnectionID
	PacketNumber     logging.PacketNumber
	KeyPhaseBit      logging.KeyPhaseBit
}

func transformShortHeader(hdr *logging.ShortHeader) *shortHeader {
	return &shortHeader{
		DestConnectionID: hdr.DestConnectionID,
		PacketNumber:     hdr.PacketNumber,
		KeyPhaseBit:      hdr.KeyPhase,
	}
}

func (h shortHeader) IsNil() bool { return false }
func (h shortHeader) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("packet_type", packetType(logging.PacketType1RTT).String())
	if h.DestConnectionID.Len() > 0 {
		enc.StringKey("dcid", h.DestConnectionID.String())
	}
	enc.Int64Key("packet_number", int64(h.PacketNumber))
	enc.StringKey("key_phase_bit", h.KeyPhaseBit.String())
}
