package wire

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"time"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/qerr"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/quicvarint"
)

// AdditionalTransportParametersClient are additional transport parameters that will be added
// to the client's transport parameters.
// This is not intended for production use, but _only_ to increase the size of the ClientHello beyond
// the usual size of less than 1 MTU.
var AdditionalTransportParametersClient map[uint64][]byte

const transportParameterMarshalingVersion = 1

type transportParameterID uint64

const (
	originalDestinationConnectionIDParameterID transportParameterID = 0x0
	maxIdleTimeoutParameterID                  transportParameterID = 0x1
	statelessResetTokenParameterID             transportParameterID = 0x2
	maxUDPPayloadSizeParameterID               transportParameterID = 0x3
	initialMaxDataParameterID                  transportParameterID = 0x4
	initialMaxStreamDataBidiLocalParameterID   transportParameterID = 0x5
	initialMaxStreamDataBidiRemoteParameterID  transportParameterID = 0x6
	initialMaxStreamDataUniParameterID         transportParameterID = 0x7
	initialMaxStreamsBidiParameterID           transportParameterID = 0x8
	initialMaxStreamsUniParameterID            transportParameterID = 0x9
	ackDelayExponentParameterID                transportParameterID = 0xa
	maxAckDelayParameterID                     transportParameterID = 0xb
	disableActiveMigrationParameterID          transportParameterID = 0xc
	preferredAddressParameterID                transportParameterID = 0xd
	activeConnectionIDLimitParameterID         transportParameterID = 0xe
	initialSourceConnectionIDParameterID       transportParameterID = 0xf
	retrySourceConnectionIDParameterID         transportParameterID = 0x10
	// RFC 9221
	maxDatagramFrameSizeParameterID transportParameterID = 0x20
)

// PreferredAddress is the value encoding in the preferred_address transport parameter
type PreferredAddress struct {
	IPv4                net.IP
	IPv4Port            uint16
	IPv6                net.IP
	IPv6Port            uint16
	ConnectionID        protocol.ConnectionID
	StatelessResetToken protocol.StatelessResetToken
}

// TransportParameters are parameters sent to the peer during the handshake
type TransportParameters struct {
	InitialMaxStreamDataBidiLocal  protocol.ByteCount
	InitialMaxStreamDataBidiRemote protocol.ByteCount
	InitialMaxStreamDataUni        protocol.ByteCount
	InitialMaxData                 protocol.ByteCount

	MaxAckDelay      time.Duration
	AckDelayExponent uint8

	DisableActiveMigration bool

	MaxUDPPayloadSize protocol.ByteCount

	MaxUniStreamNum  protocol.StreamNum
	MaxBidiStreamNum protocol.StreamNum

	MaxIdleTimeout time.Duration

	PreferredAddress *PreferredAddress

	OriginalDestinationConnectionID protocol.ConnectionID
	InitialSourceConnectionID       protocol.ConnectionID
	RetrySourceConnectionID         *protocol.ConnectionID // use a pointer here to distinguish zero-length connection IDs from missing transport parameters

	StatelessResetToken     *protocol.StatelessResetToken
	ActiveConnectionIDLimit uint64

	MaxDatagramFrameSize protocol.ByteCount
}

// Unmarshal the transport parameters
func (p *TransportParameters) Unmarshal(data []byte, sentBy protocol.Perspective) error {
	if err := p.unmarshal(bytes.NewReader(data), sentBy, false); err != nil {
		return &qerr.TransportError{
			ErrorCode:    qerr.TransportParameterError,
			ErrorMessage: err.Error(),
		}
	}
	return nil
}

func (p *TransportParameters) unmarshal(r *bytes.Reader, sentBy protocol.Perspective, fromSessionTicket bool) error {
	// needed to check that every parameter is only sent at most once
	var parameterIDs []transportParameterID

	var (
		readOriginalDestinationConnectionID bool
		readInitialSourceConnectionID       bool
		readActiveConnectionIDLimit         bool
	)

	p.AckDelayExponent = protocol.DefaultAckDelayExponent
	p.MaxAckDelay = protocol.DefaultMaxAckDelay
	p.MaxDatagramFrameSize = protocol.InvalidByteCount

	for r.Len() > 0 {
		paramIDInt, err := quicvarint.Read(r)
		if err != nil {
			return err
		}
		paramID := transportParameterID(paramIDInt)
		paramLen, err := quicvarint.Read(r)
		if err != nil {
			return err
		}
		if uint64(r.Len()) < paramLen {
			return fmt.Errorf("remaining length (%d) smaller than parameter length (%d)", r.Len(), paramLen)
		}
		parameterIDs = append(parameterIDs, paramID)
		switch paramID {
		case activeConnectionIDLimitParameterID:
			readActiveConnectionIDLimit = true
			fallthrough
		case maxIdleTimeoutParameterID,
			maxUDPPayloadSizeParameterID,
			initialMaxDataParameterID,
			initialMaxStreamDataBidiLocalParameterID,
			initialMaxStreamDataBidiRemoteParameterID,
			initialMaxStreamDataUniParameterID,
			initialMaxStreamsBidiParameterID,
			initialMaxStreamsUniParameterID,
			maxAckDelayParameterID,
			maxDatagramFrameSizeParameterID,
			ackDelayExponentParameterID:
			if err := p.readNumericTransportParameter(r, paramID, int(paramLen)); err != nil {
				return err
			}
		case preferredAddressParameterID:
			if sentBy == protocol.PerspectiveClient {
				return errors.New("client sent a preferred_address")
			}
			if err := p.readPreferredAddress(r, int(paramLen)); err != nil {
				return err
			}
		case disableActiveMigrationParameterID:
			if paramLen != 0 {
				return fmt.Errorf("wrong length for disable_active_migration: %d (expected empty)", paramLen)
			}
			p.DisableActiveMigration = true
		case statelessResetTokenParameterID:
			if sentBy == protocol.PerspectiveClient {
				return errors.New("client sent a stateless_reset_token")
			}
			if paramLen != 16 {
				return fmt.Errorf("wrong length for stateless_reset_token: %d (expected 16)", paramLen)
			}
			var token protocol.StatelessResetToken
			r.Read(token[:])
			p.StatelessResetToken = &token
		case originalDestinationConnectionIDParameterID:
			if sentBy == protocol.PerspectiveClient {
				return errors.New("client sent an original_destination_connection_id")
			}
			p.OriginalDestinationConnectionID, _ = protocol.ReadConnectionID(r, int(paramLen))
			readOriginalDestinationConnectionID = true
		case initialSourceConnectionIDParameterID:
			p.InitialSourceConnectionID, _ = protocol.ReadConnectionID(r, int(paramLen))
			readInitialSourceConnectionID = true
		case retrySourceConnectionIDParameterID:
			if sentBy == protocol.PerspectiveClient {
				return errors.New("client sent a retry_source_connection_id")
			}
			connID, _ := protocol.ReadConnectionID(r, int(paramLen))
			p.RetrySourceConnectionID = &connID
		default:
			r.Seek(int64(paramLen), io.SeekCurrent)
		}
	}

	if !readActiveConnectionIDLimit {
		p.ActiveConnectionIDLimit = protocol.DefaultActiveConnectionIDLimit
	}
	if !fromSessionTicket {
		if sentBy == protocol.PerspectiveServer && !readOriginalDestinationConnectionID {
			return errors.New("missing original_destination_connection_id")
		}
		if p.MaxUDPPayloadSize == 0 {
			p.MaxUDPPayloadSize = protocol.MaxByteCount
		}
		if !readInitialSourceConnectionID {
			return errors.New("missing initial_source_connection_id")
		}
	}

	// check that every transport parameter was sent at most once
	sort.Slice(parameterIDs, func(i, j int) bool { return parameterIDs[i] < parameterIDs[j] })
	for i := 0; i < len(parameterIDs)-1; i++ {
		if parameterIDs[i] == parameterIDs[i+1] {
			return fmt.Errorf("received duplicate transport parameter %#x", parameterIDs[i])
		}
	}

	return nil
}

func (p *TransportParameters) readPreferredAddress(r *bytes.Reader, expectedLen int) error {
	remainingLen := r.Len()
	pa := &PreferredAddress{}
	ipv4 := make([]byte, 4)
	if _, err := io.ReadFull(r, ipv4); err != nil {
		return err
	}
	pa.IPv4 = net.IP(ipv4)
	port, err := utils.BigEndian.ReadUint16(r)
	if err != nil {
		return err
	}
	pa.IPv4Port = port
	ipv6 := make([]byte, 16)
	if _, err := io.ReadFull(r, ipv6); err != nil {
		return err
	}
	pa.IPv6 = net.IP(ipv6)
	port, err = utils.BigEndian.ReadUint16(r)
	if err != nil {
		return err
	}
	pa.IPv6Port = port
	connIDLen, err := r.ReadByte()
	if err != nil {
		return err
	}
	if connIDLen == 0 || connIDLen > protocol.MaxConnIDLen {
		return fmt.Errorf("invalid connection ID length: %d", connIDLen)
	}
	connID, err := protocol.ReadConnectionID(r, int(connIDLen))
	if err != nil {
		return err
	}
	pa.ConnectionID = connID
	if _, err := io.ReadFull(r, pa.StatelessResetToken[:]); err != nil {
		return err
	}
	if bytesRead := remainingLen - r.Len(); bytesRead != expectedLen {
		return fmt.Errorf("expected preferred_address to be %d long, read %d bytes", expectedLen, bytesRead)
	}
	p.PreferredAddress = pa
	return nil
}

func (p *TransportParameters) readNumericTransportParameter(
	r *bytes.Reader,
	paramID transportParameterID,
	expectedLen int,
) error {
	remainingLen := r.Len()
	val, err := quicvarint.Read(r)
	if err != nil {
		return fmt.Errorf("error while reading transport parameter %d: %s", paramID, err)
	}
	if remainingLen-r.Len() != expectedLen {
		return fmt.Errorf("inconsistent transport parameter length for transport parameter %#x", paramID)
	}
	//nolint:exhaustive // This only covers the numeric transport parameters.
	switch paramID {
	case initialMaxStreamDataBidiLocalParameterID:
		p.InitialMaxStreamDataBidiLocal = protocol.ByteCount(val)
	case initialMaxStreamDataBidiRemoteParameterID:
		p.InitialMaxStreamDataBidiRemote = protocol.ByteCount(val)
	case initialMaxStreamDataUniParameterID:
		p.InitialMaxStreamDataUni = protocol.ByteCount(val)
	case initialMaxDataParameterID:
		p.InitialMaxData = protocol.ByteCount(val)
	case initialMaxStreamsBidiParameterID:
		p.MaxBidiStreamNum = protocol.StreamNum(val)
		if p.MaxBidiStreamNum > protocol.MaxStreamCount {
			return fmt.Errorf("initial_max_streams_bidi too large: %d (maximum %d)", p.MaxBidiStreamNum, protocol.MaxStreamCount)
		}
	case initialMaxStreamsUniParameterID:
		p.MaxUniStreamNum = protocol.StreamNum(val)
		if p.MaxUniStreamNum > protocol.MaxStreamCount {
			return fmt.Errorf("initial_max_streams_uni too large: %d (maximum %d)", p.MaxUniStreamNum, protocol.MaxStreamCount)
		}
	case maxIdleTimeoutParameterID:
		p.MaxIdleTimeout = utils.Max(protocol.MinRemoteIdleTimeout, time.Duration(val)*time.Millisecond)
	case maxUDPPayloadSizeParameterID:
		if val < 1200 {
			return fmt.Errorf("invalid value for max_packet_size: %d (minimum 1200)", val)
		}
		p.MaxUDPPayloadSize = protocol.ByteCount(val)
	case ackDelayExponentParameterID:
		if val > protocol.MaxAckDelayExponent {
			return fmt.Errorf("invalid value for ack_delay_exponent: %d (maximum %d)", val, protocol.MaxAckDelayExponent)
		}
		p.AckDelayExponent = uint8(val)
	case maxAckDelayParameterID:
		if val > uint64(protocol.MaxMaxAckDelay/time.Millisecond) {
			return fmt.Errorf("invalid value for max_ack_delay: %dms (maximum %dms)", val, protocol.MaxMaxAckDelay/time.Millisecond)
		}
		p.MaxAckDelay = time.Duration(val) * time.Millisecond
	case activeConnectionIDLimitParameterID:
		if val < 2 {
			return fmt.Errorf("invalid value for active_connection_id_limit: %d (minimum 2)", val)
		}
		p.ActiveConnectionIDLimit = val
	case maxDatagramFrameSizeParameterID:
		p.MaxDatagramFrameSize = protocol.ByteCount(val)
	default:
		return fmt.Errorf("TransportParameter BUG: transport parameter %d not found", paramID)
	}
	return nil
}

// Marshal the transport parameters
func (p *TransportParameters) Marshal(pers protocol.Perspective) []byte {
	// Typical Transport Parameters consume around 110 bytes, depending on the exact values,
	// especially the lengths of the Connection IDs.
	// Allocate 256 bytes, so we won't have to grow the slice in any case.
	b := make([]byte, 0, 256)

	// add a greased value
	random := make([]byte, 18)
	rand.Read(random)
	b = quicvarint.Append(b, 27+31*uint64(random[0]))
	length := random[1] % 16
	b = quicvarint.Append(b, uint64(length))
	b = append(b, random[2:2+length]...)

	// initial_max_stream_data_bidi_local
	b = p.marshalVarintParam(b, initialMaxStreamDataBidiLocalParameterID, uint64(p.InitialMaxStreamDataBidiLocal))
	// initial_max_stream_data_bidi_remote
	b = p.marshalVarintParam(b, initialMaxStreamDataBidiRemoteParameterID, uint64(p.InitialMaxStreamDataBidiRemote))
	// initial_max_stream_data_uni
	b = p.marshalVarintParam(b, initialMaxStreamDataUniParameterID, uint64(p.InitialMaxStreamDataUni))
	// initial_max_data
	b = p.marshalVarintParam(b, initialMaxDataParameterID, uint64(p.InitialMaxData))
	// initial_max_bidi_streams
	b = p.marshalVarintParam(b, initialMaxStreamsBidiParameterID, uint64(p.MaxBidiStreamNum))
	// initial_max_uni_streams
	b = p.marshalVarintParam(b, initialMaxStreamsUniParameterID, uint64(p.MaxUniStreamNum))
	// idle_timeout
	b = p.marshalVarintParam(b, maxIdleTimeoutParameterID, uint64(p.MaxIdleTimeout/time.Millisecond))
	// max_packet_size
	b = p.marshalVarintParam(b, maxUDPPayloadSizeParameterID, uint64(protocol.MaxPacketBufferSize))
	// max_ack_delay
	// Only send it if is different from the default value.
	if p.MaxAckDelay != protocol.DefaultMaxAckDelay {
		b = p.marshalVarintParam(b, maxAckDelayParameterID, uint64(p.MaxAckDelay/time.Millisecond))
	}
	// ack_delay_exponent
	// Only send it if is different from the default value.
	if p.AckDelayExponent != protocol.DefaultAckDelayExponent {
		b = p.marshalVarintParam(b, ackDelayExponentParameterID, uint64(p.AckDelayExponent))
	}
	// disable_active_migration
	if p.DisableActiveMigration {
		b = quicvarint.Append(b, uint64(disableActiveMigrationParameterID))
		b = quicvarint.Append(b, 0)
	}
	if pers == protocol.PerspectiveServer {
		// stateless_reset_token
		if p.StatelessResetToken != nil {
			b = quicvarint.Append(b, uint64(statelessResetTokenParameterID))
			b = quicvarint.Append(b, 16)
			b = append(b, p.StatelessResetToken[:]...)
		}
		// original_destination_connection_id
		b = quicvarint.Append(b, uint64(originalDestinationConnectionIDParameterID))
		b = quicvarint.Append(b, uint64(p.OriginalDestinationConnectionID.Len()))
		b = append(b, p.OriginalDestinationConnectionID.Bytes()...)
		// preferred_address
		if p.PreferredAddress != nil {
			b = quicvarint.Append(b, uint64(preferredAddressParameterID))
			b = quicvarint.Append(b, 4+2+16+2+1+uint64(p.PreferredAddress.ConnectionID.Len())+16)
			ipv4 := p.PreferredAddress.IPv4
			b = append(b, ipv4[len(ipv4)-4:]...)
			b = append(b, []byte{0, 0}...)
			binary.BigEndian.PutUint16(b[len(b)-2:], p.PreferredAddress.IPv4Port)
			b = append(b, p.PreferredAddress.IPv6...)
			b = append(b, []byte{0, 0}...)
			binary.BigEndian.PutUint16(b[len(b)-2:], p.PreferredAddress.IPv6Port)
			b = append(b, uint8(p.PreferredAddress.ConnectionID.Len()))
			b = append(b, p.PreferredAddress.ConnectionID.Bytes()...)
			b = append(b, p.PreferredAddress.StatelessResetToken[:]...)
		}
	}
	// active_connection_id_limit
	if p.ActiveConnectionIDLimit != protocol.DefaultActiveConnectionIDLimit {
		b = p.marshalVarintParam(b, activeConnectionIDLimitParameterID, p.ActiveConnectionIDLimit)
	}
	// initial_source_connection_id
	b = quicvarint.Append(b, uint64(initialSourceConnectionIDParameterID))
	b = quicvarint.Append(b, uint64(p.InitialSourceConnectionID.Len()))
	b = append(b, p.InitialSourceConnectionID.Bytes()...)
	// retry_source_connection_id
	if pers == protocol.PerspectiveServer && p.RetrySourceConnectionID != nil {
		b = quicvarint.Append(b, uint64(retrySourceConnectionIDParameterID))
		b = quicvarint.Append(b, uint64(p.RetrySourceConnectionID.Len()))
		b = append(b, p.RetrySourceConnectionID.Bytes()...)
	}
	if p.MaxDatagramFrameSize != protocol.InvalidByteCount {
		b = p.marshalVarintParam(b, maxDatagramFrameSizeParameterID, uint64(p.MaxDatagramFrameSize))
	}

	if pers == protocol.PerspectiveClient && len(AdditionalTransportParametersClient) > 0 {
		for k, v := range AdditionalTransportParametersClient {
			b = quicvarint.Append(b, k)
			b = quicvarint.Append(b, uint64(len(v)))
			b = append(b, v...)
		}
	}

	return b
}

func (p *TransportParameters) marshalVarintParam(b []byte, id transportParameterID, val uint64) []byte {
	b = quicvarint.Append(b, uint64(id))
	b = quicvarint.Append(b, uint64(quicvarint.Len(val)))
	return quicvarint.Append(b, val)
}

// MarshalForSessionTicket marshals the transport parameters we save in the session ticket.
// When sending a 0-RTT enabled TLS session tickets, we need to save the transport parameters.
// The client will remember the transport parameters used in the last session,
// and apply those to the 0-RTT data it sends.
// Saving the transport parameters in the ticket gives the server the option to reject 0-RTT
// if the transport parameters changed.
// Since the session ticket is encrypted, the serialization format is defined by the server.
// For convenience, we use the same format that we also use for sending the transport parameters.
func (p *TransportParameters) MarshalForSessionTicket(b []byte) []byte {
	b = quicvarint.Append(b, transportParameterMarshalingVersion)

	// initial_max_stream_data_bidi_local
	b = p.marshalVarintParam(b, initialMaxStreamDataBidiLocalParameterID, uint64(p.InitialMaxStreamDataBidiLocal))
	// initial_max_stream_data_bidi_remote
	b = p.marshalVarintParam(b, initialMaxStreamDataBidiRemoteParameterID, uint64(p.InitialMaxStreamDataBidiRemote))
	// initial_max_stream_data_uni
	b = p.marshalVarintParam(b, initialMaxStreamDataUniParameterID, uint64(p.InitialMaxStreamDataUni))
	// initial_max_data
	b = p.marshalVarintParam(b, initialMaxDataParameterID, uint64(p.InitialMaxData))
	// initial_max_bidi_streams
	b = p.marshalVarintParam(b, initialMaxStreamsBidiParameterID, uint64(p.MaxBidiStreamNum))
	// initial_max_uni_streams
	b = p.marshalVarintParam(b, initialMaxStreamsUniParameterID, uint64(p.MaxUniStreamNum))
	// max_datagram_frame_size
	if p.MaxDatagramFrameSize != protocol.InvalidByteCount {
		b = p.marshalVarintParam(b, maxDatagramFrameSizeParameterID, uint64(p.MaxDatagramFrameSize))
	}
	// active_connection_id_limit
	return p.marshalVarintParam(b, activeConnectionIDLimitParameterID, p.ActiveConnectionIDLimit)
}

// UnmarshalFromSessionTicket unmarshals transport parameters from a session ticket.
func (p *TransportParameters) UnmarshalFromSessionTicket(r *bytes.Reader) error {
	version, err := quicvarint.Read(r)
	if err != nil {
		return err
	}
	if version != transportParameterMarshalingVersion {
		return fmt.Errorf("unknown transport parameter marshaling version: %d", version)
	}
	return p.unmarshal(r, protocol.PerspectiveServer, true)
}

// ValidFor0RTT checks if the transport parameters match those saved in the session ticket.
func (p *TransportParameters) ValidFor0RTT(saved *TransportParameters) bool {
	if saved.MaxDatagramFrameSize != protocol.InvalidByteCount && (p.MaxDatagramFrameSize == protocol.InvalidByteCount || p.MaxDatagramFrameSize < saved.MaxDatagramFrameSize) {
		return false
	}
	return p.InitialMaxStreamDataBidiLocal >= saved.InitialMaxStreamDataBidiLocal &&
		p.InitialMaxStreamDataBidiRemote >= saved.InitialMaxStreamDataBidiRemote &&
		p.InitialMaxStreamDataUni >= saved.InitialMaxStreamDataUni &&
		p.InitialMaxData >= saved.InitialMaxData &&
		p.MaxBidiStreamNum >= saved.MaxBidiStreamNum &&
		p.MaxUniStreamNum >= saved.MaxUniStreamNum &&
		p.ActiveConnectionIDLimit == saved.ActiveConnectionIDLimit
}

// ValidForUpdate checks that the new transport parameters don't reduce limits after resuming a 0-RTT connection.
// It is only used on the client side.
func (p *TransportParameters) ValidForUpdate(saved *TransportParameters) bool {
	if saved.MaxDatagramFrameSize != protocol.InvalidByteCount && (p.MaxDatagramFrameSize == protocol.InvalidByteCount || p.MaxDatagramFrameSize < saved.MaxDatagramFrameSize) {
		return false
	}
	return p.ActiveConnectionIDLimit >= saved.ActiveConnectionIDLimit &&
		p.InitialMaxData >= saved.InitialMaxData &&
		p.InitialMaxStreamDataBidiLocal >= saved.InitialMaxStreamDataBidiLocal &&
		p.InitialMaxStreamDataBidiRemote >= saved.InitialMaxStreamDataBidiRemote &&
		p.InitialMaxStreamDataUni >= saved.InitialMaxStreamDataUni &&
		p.MaxBidiStreamNum >= saved.MaxBidiStreamNum &&
		p.MaxUniStreamNum >= saved.MaxUniStreamNum
}

// String returns a string representation, intended for logging.
func (p *TransportParameters) String() string {
	logString := "&wire.TransportParameters{OriginalDestinationConnectionID: %s, InitialSourceConnectionID: %s, "
	logParams := []interface{}{p.OriginalDestinationConnectionID, p.InitialSourceConnectionID}
	if p.RetrySourceConnectionID != nil {
		logString += "RetrySourceConnectionID: %s, "
		logParams = append(logParams, p.RetrySourceConnectionID)
	}
	logString += "InitialMaxStreamDataBidiLocal: %d, InitialMaxStreamDataBidiRemote: %d, InitialMaxStreamDataUni: %d, InitialMaxData: %d, MaxBidiStreamNum: %d, MaxUniStreamNum: %d, MaxIdleTimeout: %s, AckDelayExponent: %d, MaxAckDelay: %s, ActiveConnectionIDLimit: %d"
	logParams = append(logParams, []interface{}{p.InitialMaxStreamDataBidiLocal, p.InitialMaxStreamDataBidiRemote, p.InitialMaxStreamDataUni, p.InitialMaxData, p.MaxBidiStreamNum, p.MaxUniStreamNum, p.MaxIdleTimeout, p.AckDelayExponent, p.MaxAckDelay, p.ActiveConnectionIDLimit}...)
	if p.StatelessResetToken != nil { // the client never sends a stateless reset token
		logString += ", StatelessResetToken: %#x"
		logParams = append(logParams, *p.StatelessResetToken)
	}
	if p.MaxDatagramFrameSize != protocol.InvalidByteCount {
		logString += ", MaxDatagramFrameSize: %d"
		logParams = append(logParams, p.MaxDatagramFrameSize)
	}
	logString += "}"
	return fmt.Sprintf(logString, logParams...)
}
