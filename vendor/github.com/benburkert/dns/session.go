package dns

import (
	"context"
	"io"
	"net"
)

type packetSession struct {
	session
}

func (s *packetSession) Read(b []byte) (int, error) {
	msg, err := s.recv()
	if err != nil {
		return 0, err
	}

	buf, err := msg.Pack(b[:0:len(b)], true)
	if err != nil {
		return 0, err
	}
	if len(buf) > len(b) {
		if buf, err = truncate(buf, len(b)); err != nil {
			return 0, err
		}

		copy(b, buf)
		return len(buf), nil
	}
	return len(buf), nil
}

func (s *packetSession) ReadFrom(b []byte) (int, net.Addr, error) {
	n, err := s.Read(b)
	return n, s.addr, err
}

func (s *packetSession) Write(b []byte) (int, error) {
	msg := new(Message)
	if _, err := msg.Unpack(b); err != nil {
		return 0, err
	}

	query := &Query{
		RemoteAddr: s.addr,
		Message:    msg,
	}

	go s.do(query)

	return len(b), nil
}

func (s *packetSession) WriteTo(b []byte, addr net.Addr) (int, error) {
	return s.Write(b)
}

type streamSession struct {
	session

	rbuf []byte
}

func (s *streamSession) Read(b []byte) (int, error) {
	if len(s.rbuf) > 0 {
		return s.read(b)
	}

	msg, err := s.recv()
	if err != nil {
		return 0, err
	}

	if s.rbuf, err = msg.Pack(s.rbuf[:0], true); err != nil {
		return 0, err
	}

	mlen := uint16(len(s.rbuf))
	if int(mlen) != len(s.rbuf) {
		return 0, ErrOversizedMessage
	}
	nbo.PutUint16(b, mlen)

	if len(b) == 2 {
		return 2, nil
	}

	n, err := s.read(b[2:])
	return 2 + n, err
}

func (s *streamSession) read(b []byte) (int, error) {
	if len(s.rbuf) > len(b) {
		copy(b, s.rbuf[:len(b)])
		s.rbuf = s.rbuf[len(b):]
		return len(b), nil
	}

	n := len(s.rbuf)
	copy(b, s.rbuf)
	s.rbuf = s.rbuf[:0]
	return n, nil
}

func (s streamSession) Write(b []byte) (int, error) {
	if len(b) < 2 {
		return 0, io.ErrShortWrite
	}

	mlen := nbo.Uint16(b[:2])
	buf := b[2:]

	if int(mlen) != len(buf) {
		return 0, io.ErrShortWrite
	}

	msg := new(Message)
	if _, err := msg.Unpack(buf); err != nil {
		return 0, err
	}

	query := &Query{
		RemoteAddr: s.addr,
		Message:    msg,
	}

	go s.do(query)

	return len(b), nil
}

type session struct {
	Conn

	addr net.Addr

	client *Client

	msgerrc chan msgerr
}

type msgerr struct {
	msg *Message
	err error
}

func (s session) do(query *Query) {
	msg, err := s.client.do(context.Background(), s.Conn, query)
	s.msgerrc <- msgerr{msg, err}
}

func (s session) recv() (*Message, error) {
	me, ok := <-s.msgerrc
	if !ok {
		panic("impossible")
	}
	return me.msg, me.err
}

func truncate(buf []byte, maxPacketLength int) ([]byte, error) {
	msg := new(Message)
	if _, err := msg.Unpack(buf[:maxPacketLen]); err != nil {
		if err != errResourceLen && err != errBaseLen {
			return nil, err
		}
	}
	msg.Truncated = true

	return msg.Pack(buf[:0], true)
}
