package dns

import (
	"io"
	"net"
)

// Conn is a network connection to a DNS resolver.
type Conn interface {
	net.Conn

	// Recv reads a DNS message from the connection.
	Recv(msg *Message) error

	// Send writes a DNS message to the connection.
	Send(msg *Message) error
}

// PacketConn is a packet-oriented network connection to a DNS resolver that
// expects transmitted messages to adhere to RFC 1035 Section 4.2.1. "UDP
// usage".
type PacketConn struct {
	net.Conn

	rbuf, wbuf []byte
}

// Recv reads a DNS message from the underlying connection.
func (c *PacketConn) Recv(msg *Message) error {
	if len(c.rbuf) != maxPacketLen {
		c.rbuf = make([]byte, maxPacketLen)
	}

	n, err := c.Read(c.rbuf)
	if err != nil {
		return err
	}

	_, err = msg.Unpack(c.rbuf[:n])
	return err
}

// Send writes a DNS message to the underlying connection.
func (c *PacketConn) Send(msg *Message) error {
	if len(c.wbuf) != maxPacketLen {
		c.wbuf = make([]byte, maxPacketLen)
	}

	var err error
	if c.wbuf, err = msg.Pack(c.wbuf[:0], true); err != nil {
		return err
	}

	if len(c.wbuf) > maxPacketLen {
		return ErrOversizedMessage
	}

	_, err = c.Write(c.wbuf)
	return err
}

// StreamConn is a stream-oriented network connection to a DNS resolver that
// expects transmitted messages to adhere to RFC 1035 Section 4.2.2. "TCP
// usage".
type StreamConn struct {
	net.Conn

	rbuf, wbuf []byte
}

// Recv reads a DNS message from the underlying connection.
func (c *StreamConn) Recv(msg *Message) error {
	if len(c.rbuf) < 2 {
		c.rbuf = make([]byte, 1280)
	}

	if _, err := io.ReadFull(c, c.rbuf[:2]); err != nil {
		return err
	}

	mlen := nbo.Uint16(c.rbuf[:2])
	if len(c.rbuf) < int(mlen) {
		c.rbuf = make([]byte, mlen)
	}

	if _, err := io.ReadFull(c, c.rbuf[:mlen]); err != nil {
		return err
	}

	_, err := msg.Unpack(c.rbuf[:mlen])
	return err
}

// Send writes a DNS message to the underlying connection.
func (c *StreamConn) Send(msg *Message) error {
	if len(c.wbuf) < 2 {
		c.wbuf = make([]byte, 1024)
	}

	b, err := msg.Pack(c.wbuf[2:2], true)
	if err != nil {
		return err
	}

	mlen := uint16(len(b))
	if int(mlen) != len(b) {
		return ErrOversizedMessage
	}
	nbo.PutUint16(c.wbuf[:2], mlen)

	_, err = c.Write(c.wbuf[:len(b)+2])
	return err
}
