package wire

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/quicvarint"
)

// ParseConnectionID parses the destination connection ID of a packet.
func ParseConnectionID(data []byte, shortHeaderConnIDLen int) (protocol.ConnectionID, error) {
	if len(data) == 0 {
		return protocol.ConnectionID{}, io.EOF
	}
	if !IsLongHeaderPacket(data[0]) {
		if len(data) < shortHeaderConnIDLen+1 {
			return protocol.ConnectionID{}, io.EOF
		}
		return protocol.ParseConnectionID(data[1 : 1+shortHeaderConnIDLen]), nil
	}
	if len(data) < 6 {
		return protocol.ConnectionID{}, io.EOF
	}
	destConnIDLen := int(data[5])
	if destConnIDLen > protocol.MaxConnIDLen {
		return protocol.ConnectionID{}, protocol.ErrInvalidConnectionIDLen
	}
	if len(data) < 6+destConnIDLen {
		return protocol.ConnectionID{}, io.EOF
	}
	return protocol.ParseConnectionID(data[6 : 6+destConnIDLen]), nil
}

// ParseArbitraryLenConnectionIDs parses the most general form of a Long Header packet,
// using only the version-independent packet format as described in Section 5.1 of RFC 8999:
// https://datatracker.ietf.org/doc/html/rfc8999#section-5.1.
// This function should only be called on Long Header packets for which we don't support the version.
func ParseArbitraryLenConnectionIDs(data []byte) (bytesParsed int, dest, src protocol.ArbitraryLenConnectionID, _ error) {
	r := bytes.NewReader(data)
	remaining := r.Len()
	src, dest, err := parseArbitraryLenConnectionIDs(r)
	return remaining - r.Len(), src, dest, err
}

func parseArbitraryLenConnectionIDs(r *bytes.Reader) (dest, src protocol.ArbitraryLenConnectionID, _ error) {
	r.Seek(5, io.SeekStart) // skip first byte and version field
	destConnIDLen, err := r.ReadByte()
	if err != nil {
		return nil, nil, err
	}
	destConnID := make(protocol.ArbitraryLenConnectionID, destConnIDLen)
	if _, err := io.ReadFull(r, destConnID); err != nil {
		if err == io.ErrUnexpectedEOF {
			err = io.EOF
		}
		return nil, nil, err
	}
	srcConnIDLen, err := r.ReadByte()
	if err != nil {
		return nil, nil, err
	}
	srcConnID := make(protocol.ArbitraryLenConnectionID, srcConnIDLen)
	if _, err := io.ReadFull(r, srcConnID); err != nil {
		if err == io.ErrUnexpectedEOF {
			err = io.EOF
		}
		return nil, nil, err
	}
	return destConnID, srcConnID, nil
}

func IsPotentialQUICPacket(firstByte byte) bool {
	return firstByte&0x40 > 0
}

// IsLongHeaderPacket says if this is a Long Header packet
func IsLongHeaderPacket(firstByte byte) bool {
	return firstByte&0x80 > 0
}

// ParseVersion parses the QUIC version.
// It should only be called for Long Header packets (Short Header packets don't contain a version number).
func ParseVersion(data []byte) (protocol.VersionNumber, error) {
	if len(data) < 5 {
		return 0, io.EOF
	}
	return protocol.VersionNumber(binary.BigEndian.Uint32(data[1:5])), nil
}

// IsVersionNegotiationPacket says if this is a version negotiation packet
func IsVersionNegotiationPacket(b []byte) bool {
	if len(b) < 5 {
		return false
	}
	return IsLongHeaderPacket(b[0]) && b[1] == 0 && b[2] == 0 && b[3] == 0 && b[4] == 0
}

// Is0RTTPacket says if this is a 0-RTT packet.
// A packet sent with a version we don't understand can never be a 0-RTT packet.
func Is0RTTPacket(b []byte) bool {
	if len(b) < 5 {
		return false
	}
	if !IsLongHeaderPacket(b[0]) {
		return false
	}
	version := protocol.VersionNumber(binary.BigEndian.Uint32(b[1:5]))
	//nolint:exhaustive // We only need to test QUIC versions that we support.
	switch version {
	case protocol.Version1:
		return b[0]>>4&0b11 == 0b01
	case protocol.Version2:
		return b[0]>>4&0b11 == 0b10
	default:
		return false
	}
}

var ErrUnsupportedVersion = errors.New("unsupported version")

// The Header is the version independent part of the header
type Header struct {
	typeByte byte
	Type     protocol.PacketType

	Version          protocol.VersionNumber
	SrcConnectionID  protocol.ConnectionID
	DestConnectionID protocol.ConnectionID

	Length protocol.ByteCount

	Token []byte

	parsedLen protocol.ByteCount // how many bytes were read while parsing this header
}

// ParsePacket parses a packet.
// If the packet has a long header, the packet is cut according to the length field.
// If we understand the version, the packet is header up unto the packet number.
// Otherwise, only the invariant part of the header is parsed.
func ParsePacket(data []byte) (*Header, []byte, []byte, error) {
	if len(data) == 0 || !IsLongHeaderPacket(data[0]) {
		return nil, nil, nil, errors.New("not a long header packet")
	}
	hdr, err := parseHeader(bytes.NewReader(data))
	if err != nil {
		if err == ErrUnsupportedVersion {
			return hdr, nil, nil, ErrUnsupportedVersion
		}
		return nil, nil, nil, err
	}
	if protocol.ByteCount(len(data)) < hdr.ParsedLen()+hdr.Length {
		return nil, nil, nil, fmt.Errorf("packet length (%d bytes) is smaller than the expected length (%d bytes)", len(data)-int(hdr.ParsedLen()), hdr.Length)
	}
	packetLen := int(hdr.ParsedLen() + hdr.Length)
	return hdr, data[:packetLen], data[packetLen:], nil
}

// ParseHeader parses the header.
// For short header packets: up to the packet number.
// For long header packets:
// * if we understand the version: up to the packet number
// * if not, only the invariant part of the header
func parseHeader(b *bytes.Reader) (*Header, error) {
	startLen := b.Len()
	typeByte, err := b.ReadByte()
	if err != nil {
		return nil, err
	}

	h := &Header{typeByte: typeByte}
	err = h.parseLongHeader(b)
	h.parsedLen = protocol.ByteCount(startLen - b.Len())
	return h, err
}

func (h *Header) parseLongHeader(b *bytes.Reader) error {
	v, err := utils.BigEndian.ReadUint32(b)
	if err != nil {
		return err
	}
	h.Version = protocol.VersionNumber(v)
	if h.Version != 0 && h.typeByte&0x40 == 0 {
		return errors.New("not a QUIC packet")
	}
	destConnIDLen, err := b.ReadByte()
	if err != nil {
		return err
	}
	h.DestConnectionID, err = protocol.ReadConnectionID(b, int(destConnIDLen))
	if err != nil {
		return err
	}
	srcConnIDLen, err := b.ReadByte()
	if err != nil {
		return err
	}
	h.SrcConnectionID, err = protocol.ReadConnectionID(b, int(srcConnIDLen))
	if err != nil {
		return err
	}
	if h.Version == 0 { // version negotiation packet
		return nil
	}
	// If we don't understand the version, we have no idea how to interpret the rest of the bytes
	if !protocol.IsSupportedVersion(protocol.SupportedVersions, h.Version) {
		return ErrUnsupportedVersion
	}

	if h.Version == protocol.Version2 {
		switch h.typeByte >> 4 & 0b11 {
		case 0b00:
			h.Type = protocol.PacketTypeRetry
		case 0b01:
			h.Type = protocol.PacketTypeInitial
		case 0b10:
			h.Type = protocol.PacketType0RTT
		case 0b11:
			h.Type = protocol.PacketTypeHandshake
		}
	} else {
		switch h.typeByte >> 4 & 0b11 {
		case 0b00:
			h.Type = protocol.PacketTypeInitial
		case 0b01:
			h.Type = protocol.PacketType0RTT
		case 0b10:
			h.Type = protocol.PacketTypeHandshake
		case 0b11:
			h.Type = protocol.PacketTypeRetry
		}
	}

	if h.Type == protocol.PacketTypeRetry {
		tokenLen := b.Len() - 16
		if tokenLen <= 0 {
			return io.EOF
		}
		h.Token = make([]byte, tokenLen)
		if _, err := io.ReadFull(b, h.Token); err != nil {
			return err
		}
		_, err := b.Seek(16, io.SeekCurrent)
		return err
	}

	if h.Type == protocol.PacketTypeInitial {
		tokenLen, err := quicvarint.Read(b)
		if err != nil {
			return err
		}
		if tokenLen > uint64(b.Len()) {
			return io.EOF
		}
		h.Token = make([]byte, tokenLen)
		if _, err := io.ReadFull(b, h.Token); err != nil {
			return err
		}
	}

	pl, err := quicvarint.Read(b)
	if err != nil {
		return err
	}
	h.Length = protocol.ByteCount(pl)
	return nil
}

// ParsedLen returns the number of bytes that were consumed when parsing the header
func (h *Header) ParsedLen() protocol.ByteCount {
	return h.parsedLen
}

// ParseExtended parses the version dependent part of the header.
// The Reader has to be set such that it points to the first byte of the header.
func (h *Header) ParseExtended(b *bytes.Reader, ver protocol.VersionNumber) (*ExtendedHeader, error) {
	extHdr := h.toExtendedHeader()
	reservedBitsValid, err := extHdr.parse(b, ver)
	if err != nil {
		return nil, err
	}
	if !reservedBitsValid {
		return extHdr, ErrInvalidReservedBits
	}
	return extHdr, nil
}

func (h *Header) toExtendedHeader() *ExtendedHeader {
	return &ExtendedHeader{Header: *h}
}

// PacketType is the type of the packet, for logging purposes
func (h *Header) PacketType() string {
	return h.Type.String()
}
