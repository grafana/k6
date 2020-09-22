package dns

import (
	"bufio"
	"context"
	"crypto/tls"
	"io"
	"log"
	"net"
	"sync"
)

// A Server defines parameters for running a DNS server. The zero value for
// Server is a valid configuration.
type Server struct {
	Addr      string      // TCP and UDP address to listen on, ":domain" if empty
	Handler   Handler     // handler to invoke
	TLSConfig *tls.Config // optional TLS config, used by ListenAndServeTLS

	// Forwarder relays a recursive query. If nil, recursive queries are
	// answered with a "Query Refused" message.
	Forwarder RoundTripper

	// ErrorLog specifies an optional logger for errors accepting connections,
	// reading data, and unpacking messages.
	// If nil, logging is done via the log package's standard logger.
	ErrorLog *log.Logger
}

// ListenAndServe listens on both the TCP and UDP network address s.Addr and
// then calls Serve or ServePacket to handle queries on incoming connections.
// If srv.Addr is blank, ":domain" is used. ListenAndServe always returns a
// non-nil error.
func (s *Server) ListenAndServe(ctx context.Context) error {
	addr := s.Addr
	if addr == "" {
		addr = ":domain"
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}

	errc := make(chan error, 1)
	go func() { errc <- s.Serve(ctx, ln) }()
	go func() { errc <- s.ServePacket(ctx, conn) }()

	return <-errc
}

// ListenAndServeTLS listens on the TCP network address s.Addr and then calls
// Serve to handle requests on incoming TLS connections.
//
// If s.Addr is blank, ":853" is used.
//
// ListenAndServeTLS always returns a non-nil error.
func (s *Server) ListenAndServeTLS(ctx context.Context) error {
	addr := s.Addr
	if addr == "" {
		addr = ":domain"
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	return s.ServeTLS(ctx, ln)
}

// Serve accepts incoming connections on the Listener ln, creating a new
// service goroutine for each. The service goroutines read TCP encoded queries
// and then call s.Handler to reply to them.
//
// See RFC 1035, section 4.2.2 "TCP usage" for transport encoding of messages.
//
// Serve always returns a non-nil error.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}

		go s.serveStream(ctx, conn)
	}
}

// ServePacket reads UDP encoded queries from the PacketConn conn, creating a
// new service goroutine for each. The service goroutines call s.Handler to
// reply.
//
// See RFC 1035, section 4.2.1 "UDP usage" for transport encoding of messages.
//
// ServePacket always returns a non-nil error.
func (s *Server) ServePacket(ctx context.Context, conn net.PacketConn) error {
	defer conn.Close()

	for {
		buf := make([]byte, maxPacketLen)
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			return err
		}

		req := &Query{
			Message:    new(Message),
			RemoteAddr: addr,
		}

		if buf, err = req.Message.Unpack(buf[:n]); err != nil {
			s.logf("dns unpack: %s", err.Error())
			continue
		}
		if len(buf) != 0 {
			s.logf("dns unpack: malformed packet, extra message bytes")
			continue
		}

		pw := &packetWriter{
			messageWriter: &messageWriter{
				msg: response(req.Message),
			},

			addr: addr,
			conn: conn,
		}

		go s.handle(ctx, pw, req)
	}
}

// ServeTLS accepts incoming connections on the Listener ln, creating a new
// service goroutine for each. The service goroutines read TCP encoded queries
// over a TLS channel and then call s.Handler to reply to them, in another
// service goroutine.
//
// See RFC 7858, section 3.3 for transport encoding of messages.
//
// ServeTLS always returns a non-nil error.
func (s *Server) ServeTLS(ctx context.Context, ln net.Listener) error {
	ln = tls.NewListener(ln, s.TLSConfig.Clone())
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}

		go func(conn net.Conn) {
			if err := conn.(*tls.Conn).Handshake(); err != nil {
				s.logf("dns handshake: %s", err.Error())
				return
			}

			s.serveStream(ctx, conn)
		}(conn)
	}
}

func (s *Server) serveStream(ctx context.Context, conn net.Conn) {
	var (
		rbuf = bufio.NewReader(conn)

		lbuf [2]byte
		mu   sync.Mutex
	)

	for {
		if _, err := rbuf.Read(lbuf[:]); err != nil {
			if err != io.EOF {
				s.logf("dns read: %s", err.Error())
			}
			return
		}

		buf := make([]byte, int(nbo.Uint16(lbuf[:])))
		if _, err := io.ReadFull(rbuf, buf); err != nil {
			s.logf("dns read: %s", err.Error())
			return
		}

		req := &Query{
			Message:    new(Message),
			RemoteAddr: conn.RemoteAddr(),
		}

		var err error
		if buf, err = req.Message.Unpack(buf); err != nil {
			s.logf("dns unpack: %s", err.Error())
			continue
		}
		if len(buf) != 0 {
			s.logf("dns unpack: malformed packet, extra message bytes")
			continue
		}

		sw := streamWriter{
			messageWriter: &messageWriter{
				msg: response(req.Message),
			},

			mu:   &mu,
			conn: conn,
		}

		go s.handle(ctx, sw, req)
	}
}

func (s *Server) handle(ctx context.Context, w MessageWriter, r *Query) {
	sw := &serverWriter{
		MessageWriter: w,
		forwarder:     s.Forwarder,
		query:         r,
	}

	s.Handler.ServeDNS(ctx, sw, r)

	if !sw.replied {
		if err := sw.Reply(ctx); err != nil {
			s.logf("dns: %s", err.Error())
		}
	}
}

func (s *Server) logf(format string, args ...interface{}) {
	printf := log.Printf
	if s.ErrorLog != nil {
		printf = s.ErrorLog.Printf
	}

	printf(format, args...)
}

type packetWriter struct {
	*messageWriter

	addr net.Addr
	conn net.PacketConn
}

func (w packetWriter) Recur(ctx context.Context) (*Message, error) {
	return nil, ErrUnsupportedOp
}

func (w packetWriter) Reply(ctx context.Context) error {
	buf, err := w.msg.Pack(nil, true)
	if err != nil {
		return err
	}

	if len(buf) > maxPacketLen {
		return w.truncate(buf)
	}

	_, err = w.conn.WriteTo(buf, w.addr)
	return err
}

func (w packetWriter) truncate(buf []byte) error {
	var err error
	if buf, err = truncate(buf, maxPacketLen); err != nil {
		return err
	}

	if _, err := w.conn.WriteTo(buf, w.addr); err != nil {
		return err
	}
	return ErrTruncatedMessage
}

type streamWriter struct {
	*messageWriter

	mu   *sync.Mutex
	conn net.Conn
}

func (w streamWriter) Recur(ctx context.Context) (*Message, error) {
	return nil, ErrUnsupportedOp
}

func (w streamWriter) Reply(ctx context.Context) error {
	buf, err := w.msg.Pack(make([]byte, 2), true)
	if err != nil {
		return err
	}

	blen := uint16(len(buf) - 2)
	if int(blen) != len(buf)-2 {
		return ErrOversizedMessage
	}
	nbo.PutUint16(buf[:2], blen)

	w.mu.Lock()
	defer w.mu.Unlock()

	_, err = w.conn.Write(buf)
	return err
}

type serverWriter struct {
	MessageWriter

	forwarder RoundTripper
	query     *Query

	replied bool
}

func (w serverWriter) Recur(ctx context.Context) (*Message, error) {
	query := &Query{
		Message:    request(w.query.Message),
		RemoteAddr: w.query.RemoteAddr,
	}

	qs := make([]Question, 0, len(w.query.Questions))
	for _, q := range w.query.Questions {
		if !questionMatched(q, query.Message) {
			qs = append(qs, q)
		}
	}
	query.Questions = qs

	return w.forward(ctx, query)
}

func (w serverWriter) Reply(ctx context.Context) error {
	w.replied = true

	return w.MessageWriter.Reply(ctx)
}

func response(msg *Message) *Message {
	res := new(Message)
	*res = *msg // shallow copy

	res.Response = true

	return res
}

var refuser = &Client{
	Transport: nopDialer{},
	Resolver:  HandlerFunc(Refuse),
}

func (w serverWriter) forward(ctx context.Context, query *Query) (*Message, error) {
	if w.forwarder != nil {
		return w.forwarder.Do(ctx, query)
	}

	return refuser.Do(ctx, query)
}

type nopDialer struct{}

func (nopDialer) DialAddr(ctx context.Context, addr net.Addr) (Conn, error) {
	return nil, nil
}
