package clientcert

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

// DialFunc opens the connection for a CONNECT request.
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// TunnelFunc relays bytes between the browser and the destination. It owns both connections
// and must close them before returning.
type TunnelFunc func(ctx context.Context, browser, destination net.Conn, host string, port int)

const connectHandshakeTimeout = 30 * time.Second

// ConnectServer accepts HTTP CONNECT requests from the browser on a loopback address.
type ConnectServer struct {
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

// ListenCONNECT starts a CONNECT server on 127.0.0.1 with an ephemeral port.
func ListenCONNECT(dial DialFunc, tunnel TunnelFunc, logger *log.Logger) (*ConnectServer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	var lc net.ListenConfig
	l, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("listening for CONNECT requests: %w", err)
	}
	s := &ConnectServer{
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
func (s *ConnectServer) Addr() string {
	return s.listener.Addr().String()
}

// Close stops accepting connections, closes active connections, and waits for their goroutines.
func (s *ConnectServer) Close() error {
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

func (s *ConnectServer) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				s.logger.Debugf("ConnectServer:serve", "accept err:%v", err)
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
func (s *ConnectServer) track(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.conns[conn] = struct{}{}
	return true
}

func (s *ConnectServer) untrack(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, conn)
}

func (s *ConnectServer) handle(conn net.Conn) {
	defer s.wg.Done()
	defer s.untrack(conn)

	browser, destination, host, port, err := s.connect(conn)
	if err != nil {
		s.logger.Debugf("ConnectServer:handle", "err:%v", err)
		_ = conn.Close()
		return
	}
	s.tunnel(s.ctx, browser, destination, host, port)
}

// connect reads a CONNECT request from conn, dials its target, and returns the browser side
// of the tunnel, which yields any bytes buffered after the request first.
func (s *ConnectServer) connect(conn net.Conn) (net.Conn, net.Conn, string, int, error) {
	if err := conn.SetDeadline(time.Now().Add(connectHandshakeTimeout)); err != nil {
		return nil, nil, "", 0, fmt.Errorf("setting handshake deadline: %w", err)
	}
	br := bufio.NewReader(conn)
	r, err := http.ReadRequest(br)
	if err != nil {
		return nil, nil, "", 0, fmt.Errorf("reading request: %w", err)
	}
	if r.Method != http.MethodConnect {
		_ = writeStatus(conn, http.StatusMethodNotAllowed)
		return nil, nil, "", 0, fmt.Errorf("unsupported method %q", r.Method)
	}
	host, port, err := splitTarget(r.Host)
	if err != nil {
		_ = writeStatus(conn, http.StatusBadRequest)
		return nil, nil, "", 0, err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, nil, "", 0, fmt.Errorf("clearing handshake deadline: %w", err)
	}

	s.logger.Debugf("ConnectServer:connect", "host:%s port:%d", host, port)
	destination, err := s.dial(s.ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		_ = writeStatus(conn, http.StatusBadGateway)
		return nil, nil, "", 0, fmt.Errorf("dialing host:%s port:%d: %w", host, port, err)
	}
	if _, err := io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		_ = destination.Close()
		return nil, nil, "", 0, fmt.Errorf("writing response: %w", err)
	}
	return &bufferedConn{Conn: conn, r: br}, destination, host, port, nil
}

// splitTarget parses a CONNECT request target of the form host:port.
func splitTarget(target string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return "", 0, fmt.Errorf("parsing target: %w", err)
	}
	if host == "" {
		return "", 0, errors.New("parsing target: missing host")
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil || port == 0 {
		return "", 0, fmt.Errorf("parsing target: invalid port %q", portStr)
	}
	return host, int(port), nil
}

// writeStatus writes a bodiless response that closes the connection.
func writeStatus(conn net.Conn, code int) error {
	allow := ""
	if code == http.StatusMethodNotAllowed {
		allow = "Allow: CONNECT\r\n"
	}
	_, err := fmt.Fprintf(conn, "HTTP/1.1 %d %s\r\n%sConnection: close\r\nContent-Length: 0\r\n\r\n",
		code, http.StatusText(code), allow)
	if err != nil {
		return fmt.Errorf("writing response: %w", err)
	}
	return nil
}

// bufferedConn reads through r, which may hold bytes already read from Conn.
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}

// CloseWrite half-closes the underlying connection when it supports it.
func (c *bufferedConn) CloseWrite() error {
	if cw, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return errors.ErrUnsupported
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
