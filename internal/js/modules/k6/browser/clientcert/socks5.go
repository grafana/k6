package clientcert

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"syscall"
	"time"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

// DialFunc opens the connection for a SOCKS CONNECT request.
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// TunnelFunc relays bytes between the browser and the destination. It owns both connections
// and must close them before returning.
type TunnelFunc func(ctx context.Context, browser, destination net.Conn, host string, port int)

const (
	socksVersion = 0x05

	socksMethodNoAuth       = 0x00
	socksMethodNoAcceptable = 0xFF

	socksCmdConnect = 0x01

	socksAddrIPv4   = 0x01
	socksAddrDomain = 0x03
	socksAddrIPv6   = 0x04

	socksRepSucceeded           = 0x00
	socksRepGeneralFailure      = 0x01
	socksRepNetworkUnreachable  = 0x03
	socksRepHostUnreachable     = 0x04
	socksRepConnectionRefused   = 0x05
	socksRepCommandUnsupported  = 0x07
	socksRepAddrTypeUnsupported = 0x08

	socksHandshakeTimeout = 30 * time.Second
)

// SOCKS5Server accepts SOCKS5 CONNECT requests from the browser on a loopback address.
type SOCKS5Server struct {
	listener net.Listener
	dial     DialFunc
	tunnel   TunnelFunc
	logger   *log.Logger

	ctx    context.Context //nolint:containedctx // cancelled by Close to stop dials and tunnels
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu     sync.Mutex
	conns  map[net.Conn]struct{}
	closed bool

	closeOnce sync.Once
	closeErr  error
}

// ListenSOCKS5 starts a SOCKS5 server on 127.0.0.1 with an ephemeral port.
func ListenSOCKS5(dial DialFunc, tunnel TunnelFunc, logger *log.Logger) (*SOCKS5Server, error) {
	ctx, cancel := context.WithCancel(context.Background())
	var lc net.ListenConfig
	l, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("listening for SOCKS5 connections: %w", err)
	}
	s := &SOCKS5Server{
		listener: l,
		dial:     dial,
		tunnel:   tunnel,
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
		conns:    make(map[net.Conn]struct{}),
	}
	s.wg.Add(1)
	go s.serve()
	return s, nil
}

// Addr returns the host:port the server listens on.
func (s *SOCKS5Server) Addr() string {
	return s.listener.Addr().String()
}

// Close stops accepting connections, closes active connections, and waits for their goroutines.
func (s *SOCKS5Server) Close() error {
	s.closeOnce.Do(func() {
		s.cancel()
		s.closeErr = s.listener.Close()
		s.mu.Lock()
		s.closed = true
		for c := range s.conns {
			_ = c.Close()
		}
		s.mu.Unlock()
		s.wg.Wait()
	})
	return s.closeErr
}

func (s *SOCKS5Server) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				s.logger.Debugf("SOCKS5Server:serve", "accept err:%v", err)
			}
			return
		}
		if !s.track(conn) {
			_ = conn.Close()
			continue
		}
		s.wg.Add(1)
		go s.handle(conn)
	}
}

// track registers conn for Close and reports false if the server is already closed.
func (s *SOCKS5Server) track(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.conns[conn] = struct{}{}
	return true
}

func (s *SOCKS5Server) untrack(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, conn)
}

func (s *SOCKS5Server) handle(conn net.Conn) {
	defer s.wg.Done()
	defer s.untrack(conn)

	destination, host, port, err := s.connect(conn)
	if err != nil {
		s.logger.Debugf("SOCKS5Server:handle", "err:%v", err)
		_ = conn.Close()
		return
	}
	s.tunnel(s.ctx, conn, destination, host, port)
}

// connect performs the SOCKS5 handshake on conn and dials the requested destination.
func (s *SOCKS5Server) connect(conn net.Conn) (net.Conn, string, int, error) {
	if err := conn.SetDeadline(time.Now().Add(socksHandshakeTimeout)); err != nil {
		return nil, "", 0, fmt.Errorf("setting handshake deadline: %w", err)
	}
	if err := negotiateMethod(conn); err != nil {
		return nil, "", 0, err
	}
	host, port, err := readRequest(conn)
	if err != nil {
		return nil, "", 0, err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, "", 0, fmt.Errorf("clearing handshake deadline: %w", err)
	}

	s.logger.Debugf("SOCKS5Server:connect", "host:%s port:%d", host, port)
	destination, err := s.dial(s.ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		_ = writeReply(conn, dialReplyCode(err))
		return nil, "", 0, fmt.Errorf("dialing host:%s port:%d: %w", host, port, err)
	}
	if err := writeReply(conn, socksRepSucceeded); err != nil {
		_ = destination.Close()
		return nil, "", 0, err
	}
	return destination, host, port, nil
}

// negotiateMethod selects the "no authentication" method.
func negotiateMethod(conn net.Conn) error {
	var hdr [2]byte
	if _, err := io.ReadFull(conn, hdr[:]); err != nil {
		return fmt.Errorf("reading method negotiation: %w", err)
	}
	if hdr[0] != socksVersion {
		return fmt.Errorf("unsupported SOCKS version %d", hdr[0])
	}
	methods := make([]byte, hdr[1])
	if _, err := io.ReadFull(conn, methods); err != nil {
		return fmt.Errorf("reading methods: %w", err)
	}
	for _, m := range methods {
		if m == socksMethodNoAuth {
			if _, err := conn.Write([]byte{socksVersion, socksMethodNoAuth}); err != nil {
				return fmt.Errorf("writing method selection: %w", err)
			}
			return nil
		}
	}
	_, _ = conn.Write([]byte{socksVersion, socksMethodNoAcceptable})
	return errors.New("no acceptable SOCKS authentication method")
}

// readRequest reads a CONNECT request and returns its destination host and port.
func readRequest(conn net.Conn) (string, int, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(conn, hdr[:]); err != nil {
		return "", 0, fmt.Errorf("reading request: %w", err)
	}
	if hdr[0] != socksVersion {
		return "", 0, fmt.Errorf("unsupported SOCKS version %d", hdr[0])
	}

	var addr []byte
	switch atyp := hdr[3]; atyp {
	case socksAddrIPv4:
		addr = make([]byte, net.IPv4len)
	case socksAddrIPv6:
		addr = make([]byte, net.IPv6len)
	case socksAddrDomain:
		var n [1]byte
		if _, err := io.ReadFull(conn, n[:]); err != nil {
			return "", 0, fmt.Errorf("reading domain length: %w", err)
		}
		addr = make([]byte, n[0])
	default:
		_ = writeReply(conn, socksRepAddrTypeUnsupported)
		return "", 0, fmt.Errorf("unsupported address type %d", atyp)
	}
	if _, err := io.ReadFull(conn, addr); err != nil {
		return "", 0, fmt.Errorf("reading address: %w", err)
	}
	var portBytes [2]byte
	if _, err := io.ReadFull(conn, portBytes[:]); err != nil {
		return "", 0, fmt.Errorf("reading port: %w", err)
	}

	if cmd := hdr[1]; cmd != socksCmdConnect {
		_ = writeReply(conn, socksRepCommandUnsupported)
		return "", 0, fmt.Errorf("unsupported command %d", cmd)
	}

	host := string(addr)
	if hdr[3] != socksAddrDomain {
		host = net.IP(addr).String()
	}
	return host, int(binary.BigEndian.Uint16(portBytes[:])), nil
}

// writeReply writes a reply with an unspecified IPv4 bound address.
func writeReply(conn net.Conn, rep byte) error {
	if _, err := conn.Write([]byte{socksVersion, rep, 0x00, socksAddrIPv4, 0, 0, 0, 0, 0, 0}); err != nil {
		return fmt.Errorf("writing reply: %w", err)
	}
	return nil
}

func dialReplyCode(err error) byte {
	var dnsErr *net.DNSError
	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return socksRepConnectionRefused
	case errors.As(err, &dnsErr) && dnsErr.IsNotFound:
		return socksRepHostUnreachable
	case errors.Is(err, syscall.ENETUNREACH):
		return socksRepNetworkUnreachable
	default:
		return socksRepGeneralFailure
	}
}

// Pipe is a TunnelFunc that copies bytes in both directions until either side closes.
func Pipe(ctx context.Context, browser, destination net.Conn, _ string, _ int) {
	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = browser.Close()
			_ = destination.Close()
		})
	}
	stop := context.AfterFunc(ctx, closeBoth)
	defer stop()

	var wg sync.WaitGroup
	wg.Go(func() { copyHalf(destination, browser, closeBoth) })
	wg.Go(func() { copyHalf(browser, destination, closeBoth) })
	wg.Wait()
	closeBoth()
}

// copyHalf copies src to dst, then half-closes dst, or closes both connections when dst
// cannot be half-closed or the copy fails.
func copyHalf(dst, src net.Conn, closeBoth func()) {
	if _, err := io.Copy(dst, src); err != nil {
		closeBoth()
		return
	}
	cw, ok := dst.(interface{ CloseWrite() error })
	if !ok || cw.CloseWrite() != nil {
		closeBoth()
	}
}
