package quic

import (
	"fmt"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/qerr"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/internal/wire"
)

type connIDGenerator struct {
	generator  ConnectionIDGenerator
	highestSeq uint64

	activeSrcConnIDs        map[uint64]protocol.ConnectionID
	initialClientDestConnID *protocol.ConnectionID // nil for the client

	addConnectionID        func(protocol.ConnectionID)
	getStatelessResetToken func(protocol.ConnectionID) protocol.StatelessResetToken
	removeConnectionID     func(protocol.ConnectionID)
	retireConnectionID     func(protocol.ConnectionID)
	replaceWithClosed      func([]protocol.ConnectionID, protocol.Perspective, []byte)
	queueControlFrame      func(wire.Frame)
}

func newConnIDGenerator(
	initialConnectionID protocol.ConnectionID,
	initialClientDestConnID *protocol.ConnectionID, // nil for the client
	addConnectionID func(protocol.ConnectionID),
	getStatelessResetToken func(protocol.ConnectionID) protocol.StatelessResetToken,
	removeConnectionID func(protocol.ConnectionID),
	retireConnectionID func(protocol.ConnectionID),
	replaceWithClosed func([]protocol.ConnectionID, protocol.Perspective, []byte),
	queueControlFrame func(wire.Frame),
	generator ConnectionIDGenerator,
) *connIDGenerator {
	m := &connIDGenerator{
		generator:              generator,
		activeSrcConnIDs:       make(map[uint64]protocol.ConnectionID),
		addConnectionID:        addConnectionID,
		getStatelessResetToken: getStatelessResetToken,
		removeConnectionID:     removeConnectionID,
		retireConnectionID:     retireConnectionID,
		replaceWithClosed:      replaceWithClosed,
		queueControlFrame:      queueControlFrame,
	}
	m.activeSrcConnIDs[0] = initialConnectionID
	m.initialClientDestConnID = initialClientDestConnID
	return m
}

func (m *connIDGenerator) SetMaxActiveConnIDs(limit uint64) error {
	if m.generator.ConnectionIDLen() == 0 {
		return nil
	}
	// The active_connection_id_limit transport parameter is the number of
	// connection IDs the peer will store. This limit includes the connection ID
	// used during the handshake, and the one sent in the preferred_address
	// transport parameter.
	// We currently don't send the preferred_address transport parameter,
	// so we can issue (limit - 1) connection IDs.
	for i := uint64(len(m.activeSrcConnIDs)); i < utils.Min(limit, protocol.MaxIssuedConnectionIDs); i++ {
		if err := m.issueNewConnID(); err != nil {
			return err
		}
	}
	return nil
}

func (m *connIDGenerator) Retire(seq uint64, sentWithDestConnID protocol.ConnectionID) error {
	if seq > m.highestSeq {
		return &qerr.TransportError{
			ErrorCode:    qerr.ProtocolViolation,
			ErrorMessage: fmt.Sprintf("retired connection ID %d (highest issued: %d)", seq, m.highestSeq),
		}
	}
	connID, ok := m.activeSrcConnIDs[seq]
	// We might already have deleted this connection ID, if this is a duplicate frame.
	if !ok {
		return nil
	}
	if connID == sentWithDestConnID {
		return &qerr.TransportError{
			ErrorCode:    qerr.ProtocolViolation,
			ErrorMessage: fmt.Sprintf("retired connection ID %d (%s), which was used as the Destination Connection ID on this packet", seq, connID),
		}
	}
	m.retireConnectionID(connID)
	delete(m.activeSrcConnIDs, seq)
	// Don't issue a replacement for the initial connection ID.
	if seq == 0 {
		return nil
	}
	return m.issueNewConnID()
}

func (m *connIDGenerator) issueNewConnID() error {
	connID, err := m.generator.GenerateConnectionID()
	if err != nil {
		return err
	}
	m.activeSrcConnIDs[m.highestSeq+1] = connID
	m.addConnectionID(connID)
	m.queueControlFrame(&wire.NewConnectionIDFrame{
		SequenceNumber:      m.highestSeq + 1,
		ConnectionID:        connID,
		StatelessResetToken: m.getStatelessResetToken(connID),
	})
	m.highestSeq++
	return nil
}

func (m *connIDGenerator) SetHandshakeComplete() {
	if m.initialClientDestConnID != nil {
		m.retireConnectionID(*m.initialClientDestConnID)
		m.initialClientDestConnID = nil
	}
}

func (m *connIDGenerator) RemoveAll() {
	if m.initialClientDestConnID != nil {
		m.removeConnectionID(*m.initialClientDestConnID)
	}
	for _, connID := range m.activeSrcConnIDs {
		m.removeConnectionID(connID)
	}
}

func (m *connIDGenerator) ReplaceWithClosed(pers protocol.Perspective, connClose []byte) {
	connIDs := make([]protocol.ConnectionID, 0, len(m.activeSrcConnIDs)+1)
	if m.initialClientDestConnID != nil {
		connIDs = append(connIDs, *m.initialClientDestConnID)
	}
	for _, connID := range m.activeSrcConnIDs {
		connIDs = append(connIDs, connID)
	}
	m.replaceWithClosed(connIDs, pers, connClose)
}
