package quic

import (
	"math/bits"
	"net"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
)

// A closedLocalConn is a connection that we closed locally.
// When receiving packets for such a connection, we need to retransmit the packet containing the CONNECTION_CLOSE frame,
// with an exponential backoff.
type closedLocalConn struct {
	counter     uint32
	perspective protocol.Perspective
	logger      utils.Logger

	sendPacket func(net.Addr, packetInfo)
}

var _ packetHandler = &closedLocalConn{}

// newClosedLocalConn creates a new closedLocalConn and runs it.
func newClosedLocalConn(sendPacket func(net.Addr, packetInfo), pers protocol.Perspective, logger utils.Logger) packetHandler {
	return &closedLocalConn{
		sendPacket:  sendPacket,
		perspective: pers,
		logger:      logger,
	}
}

func (c *closedLocalConn) handlePacket(p receivedPacket) {
	c.counter++
	// exponential backoff
	// only send a CONNECTION_CLOSE for the 1st, 2nd, 4th, 8th, 16th, ... packet arriving
	if bits.OnesCount32(c.counter) != 1 {
		return
	}
	c.logger.Debugf("Received %d packets after sending CONNECTION_CLOSE. Retransmitting.", c.counter)
	c.sendPacket(p.remoteAddr, p.info)
}

func (c *closedLocalConn) shutdown()                            {}
func (c *closedLocalConn) destroy(error)                        {}
func (c *closedLocalConn) getPerspective() protocol.Perspective { return c.perspective }

// A closedRemoteConn is a connection that was closed remotely.
// For such a connection, we might receive reordered packets that were sent before the CONNECTION_CLOSE.
// We can just ignore those packets.
type closedRemoteConn struct {
	perspective protocol.Perspective
}

var _ packetHandler = &closedRemoteConn{}

func newClosedRemoteConn(pers protocol.Perspective) packetHandler {
	return &closedRemoteConn{perspective: pers}
}

func (s *closedRemoteConn) handlePacket(receivedPacket)          {}
func (s *closedRemoteConn) shutdown()                            {}
func (s *closedRemoteConn) destroy(error)                        {}
func (s *closedRemoteConn) getPerspective() protocol.Perspective { return s.perspective }
