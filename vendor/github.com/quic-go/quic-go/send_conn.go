package quic

import (
	"net"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
)

// A sendConn allows sending using a simple Write() on a non-connected packet conn.
type sendConn interface {
	Write(b []byte, gsoSize uint16, ecn protocol.ECN) error
	Close() error
	LocalAddr() net.Addr
	RemoteAddr() net.Addr

	capabilities() connCapabilities
}

type sconn struct {
	rawConn

	localAddr  net.Addr
	remoteAddr net.Addr

	logger utils.Logger

	packetInfoOOB []byte
	// If GSO enabled, and we receive a GSO error for this remote address, GSO is disabled.
	gotGSOError bool
	// Used to catch the error sometimes returned by the first sendmsg call on Linux,
	// see https://github.com/golang/go/issues/63322.
	wroteFirstPacket bool
}

var _ sendConn = &sconn{}

func newSendConn(c rawConn, remote net.Addr, info packetInfo, logger utils.Logger) *sconn {
	localAddr := c.LocalAddr()
	if info.addr.IsValid() {
		if udpAddr, ok := localAddr.(*net.UDPAddr); ok {
			addrCopy := *udpAddr
			addrCopy.IP = info.addr.AsSlice()
			localAddr = &addrCopy
		}
	}

	oob := info.OOB()
	// increase oob slice capacity, so we can add the UDP_SEGMENT and ECN control messages without allocating
	l := len(oob)
	oob = append(oob, make([]byte, 64)...)[:l]
	return &sconn{
		rawConn:       c,
		localAddr:     localAddr,
		remoteAddr:    remote,
		packetInfoOOB: oob,
		logger:        logger,
	}
}

func (c *sconn) Write(p []byte, gsoSize uint16, ecn protocol.ECN) error {
	err := c.writePacket(p, c.remoteAddr, c.packetInfoOOB, gsoSize, ecn)
	if err != nil && isGSOError(err) {
		// disable GSO for future calls
		c.gotGSOError = true
		if c.logger.Debug() {
			c.logger.Debugf("GSO failed when sending to %s", c.remoteAddr)
		}
		// send out the packets one by one
		for len(p) > 0 {
			l := len(p)
			if l > int(gsoSize) {
				l = int(gsoSize)
			}
			if err := c.writePacket(p[:l], c.remoteAddr, c.packetInfoOOB, 0, ecn); err != nil {
				return err
			}
			p = p[l:]
		}
		return nil
	}
	return err
}

func (c *sconn) writePacket(p []byte, addr net.Addr, oob []byte, gsoSize uint16, ecn protocol.ECN) error {
	_, err := c.WritePacket(p, addr, oob, gsoSize, ecn)
	if err != nil && !c.wroteFirstPacket && isPermissionError(err) {
		_, err = c.WritePacket(p, addr, oob, gsoSize, ecn)
	}
	c.wroteFirstPacket = true
	return err
}

func (c *sconn) capabilities() connCapabilities {
	capabilities := c.rawConn.capabilities()
	if capabilities.GSO {
		capabilities.GSO = !c.gotGSOError
	}
	return capabilities
}

func (c *sconn) RemoteAddr() net.Addr { return c.remoteAddr }
func (c *sconn) LocalAddr() net.Addr  { return c.localAddr }
