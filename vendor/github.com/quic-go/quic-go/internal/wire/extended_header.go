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

// ErrInvalidReservedBits is returned when the reserved bits are incorrect.
// When this error is returned, parsing continues, and an ExtendedHeader is returned.
// This is necessary because we need to decrypt the packet in that case,
// in order to avoid a timing side-channel.
var ErrInvalidReservedBits = errors.New("invalid reserved bits")

// ExtendedHeader is the header of a QUIC packet.
type ExtendedHeader struct {
	Header

	typeByte byte

	KeyPhase protocol.KeyPhaseBit

	PacketNumberLen protocol.PacketNumberLen
	PacketNumber    protocol.PacketNumber

	parsedLen protocol.ByteCount
}

func (h *ExtendedHeader) parse(b *bytes.Reader, v protocol.VersionNumber) (bool /* reserved bits valid */, error) {
	startLen := b.Len()
	// read the (now unencrypted) first byte
	var err error
	h.typeByte, err = b.ReadByte()
	if err != nil {
		return false, err
	}
	if _, err := b.Seek(int64(h.Header.ParsedLen())-1, io.SeekCurrent); err != nil {
		return false, err
	}
	reservedBitsValid, err := h.parseLongHeader(b, v)
	if err != nil {
		return false, err
	}
	h.parsedLen = protocol.ByteCount(startLen - b.Len())
	return reservedBitsValid, err
}

func (h *ExtendedHeader) parseLongHeader(b *bytes.Reader, _ protocol.VersionNumber) (bool /* reserved bits valid */, error) {
	if err := h.readPacketNumber(b); err != nil {
		return false, err
	}
	if h.typeByte&0xc != 0 {
		return false, nil
	}
	return true, nil
}

func (h *ExtendedHeader) readPacketNumber(b *bytes.Reader) error {
	h.PacketNumberLen = protocol.PacketNumberLen(h.typeByte&0x3) + 1
	switch h.PacketNumberLen {
	case protocol.PacketNumberLen1:
		n, err := b.ReadByte()
		if err != nil {
			return err
		}
		h.PacketNumber = protocol.PacketNumber(n)
	case protocol.PacketNumberLen2:
		n, err := utils.BigEndian.ReadUint16(b)
		if err != nil {
			return err
		}
		h.PacketNumber = protocol.PacketNumber(n)
	case protocol.PacketNumberLen3:
		n, err := utils.BigEndian.ReadUint24(b)
		if err != nil {
			return err
		}
		h.PacketNumber = protocol.PacketNumber(n)
	case protocol.PacketNumberLen4:
		n, err := utils.BigEndian.ReadUint32(b)
		if err != nil {
			return err
		}
		h.PacketNumber = protocol.PacketNumber(n)
	default:
		return fmt.Errorf("invalid packet number length: %d", h.PacketNumberLen)
	}
	return nil
}

// Append appends the Header.
func (h *ExtendedHeader) Append(b []byte, v protocol.VersionNumber) ([]byte, error) {
	if h.DestConnectionID.Len() > protocol.MaxConnIDLen {
		return nil, fmt.Errorf("invalid connection ID length: %d bytes", h.DestConnectionID.Len())
	}
	if h.SrcConnectionID.Len() > protocol.MaxConnIDLen {
		return nil, fmt.Errorf("invalid connection ID length: %d bytes", h.SrcConnectionID.Len())
	}

	var packetType uint8
	if v == protocol.Version2 {
		//nolint:exhaustive
		switch h.Type {
		case protocol.PacketTypeInitial:
			packetType = 0b01
		case protocol.PacketType0RTT:
			packetType = 0b10
		case protocol.PacketTypeHandshake:
			packetType = 0b11
		case protocol.PacketTypeRetry:
			packetType = 0b00
		}
	} else {
		//nolint:exhaustive
		switch h.Type {
		case protocol.PacketTypeInitial:
			packetType = 0b00
		case protocol.PacketType0RTT:
			packetType = 0b01
		case protocol.PacketTypeHandshake:
			packetType = 0b10
		case protocol.PacketTypeRetry:
			packetType = 0b11
		}
	}
	firstByte := 0xc0 | packetType<<4
	if h.Type != protocol.PacketTypeRetry {
		// Retry packets don't have a packet number
		firstByte |= uint8(h.PacketNumberLen - 1)
	}

	b = append(b, firstByte)
	b = append(b, make([]byte, 4)...)
	binary.BigEndian.PutUint32(b[len(b)-4:], uint32(h.Version))
	b = append(b, uint8(h.DestConnectionID.Len()))
	b = append(b, h.DestConnectionID.Bytes()...)
	b = append(b, uint8(h.SrcConnectionID.Len()))
	b = append(b, h.SrcConnectionID.Bytes()...)

	//nolint:exhaustive
	switch h.Type {
	case protocol.PacketTypeRetry:
		b = append(b, h.Token...)
		return b, nil
	case protocol.PacketTypeInitial:
		b = quicvarint.Append(b, uint64(len(h.Token)))
		b = append(b, h.Token...)
	}
	b = quicvarint.AppendWithLen(b, uint64(h.Length), 2)
	return appendPacketNumber(b, h.PacketNumber, h.PacketNumberLen)
}

// ParsedLen returns the number of bytes that were consumed when parsing the header
func (h *ExtendedHeader) ParsedLen() protocol.ByteCount {
	return h.parsedLen
}

// GetLength determines the length of the Header.
func (h *ExtendedHeader) GetLength(_ protocol.VersionNumber) protocol.ByteCount {
	length := 1 /* type byte */ + 4 /* version */ + 1 /* dest conn ID len */ + protocol.ByteCount(h.DestConnectionID.Len()) + 1 /* src conn ID len */ + protocol.ByteCount(h.SrcConnectionID.Len()) + protocol.ByteCount(h.PacketNumberLen) + 2 /* length */
	if h.Type == protocol.PacketTypeInitial {
		length += quicvarint.Len(uint64(len(h.Token))) + protocol.ByteCount(len(h.Token))
	}
	return length
}

// Log logs the Header
func (h *ExtendedHeader) Log(logger utils.Logger) {
	var token string
	if h.Type == protocol.PacketTypeInitial || h.Type == protocol.PacketTypeRetry {
		if len(h.Token) == 0 {
			token = "Token: (empty), "
		} else {
			token = fmt.Sprintf("Token: %#x, ", h.Token)
		}
		if h.Type == protocol.PacketTypeRetry {
			logger.Debugf("\tLong Header{Type: %s, DestConnectionID: %s, SrcConnectionID: %s, %sVersion: %s}", h.Type, h.DestConnectionID, h.SrcConnectionID, token, h.Version)
			return
		}
	}
	logger.Debugf("\tLong Header{Type: %s, DestConnectionID: %s, SrcConnectionID: %s, %sPacketNumber: %d, PacketNumberLen: %d, Length: %d, Version: %s}", h.Type, h.DestConnectionID, h.SrcConnectionID, token, h.PacketNumber, h.PacketNumberLen, h.Length, h.Version)
}

func appendPacketNumber(b []byte, pn protocol.PacketNumber, pnLen protocol.PacketNumberLen) ([]byte, error) {
	switch pnLen {
	case protocol.PacketNumberLen1:
		b = append(b, uint8(pn))
	case protocol.PacketNumberLen2:
		buf := make([]byte, 2)
		binary.BigEndian.PutUint16(buf, uint16(pn))
		b = append(b, buf...)
	case protocol.PacketNumberLen3:
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, uint32(pn))
		b = append(b, buf[1:]...)
	case protocol.PacketNumberLen4:
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, uint32(pn))
		b = append(b, buf...)
	default:
		return nil, fmt.Errorf("invalid packet number length: %d", pnLen)
	}
	return b, nil
}
