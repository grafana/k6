package echo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"
)

// ErrInvalidTCPAddress is returned when the TCP address cannot be determined.
var ErrInvalidTCPAddress = errors.New("failed to get TCP address")

// TCPServer manages a TCP echo server instance.
type TCPServer struct {
	listener net.Listener
	done     chan struct{}
	wg       sync.WaitGroup
	port     int
}

// NewTCPServer creates a new TCP echo server instance.
func NewTCPServer() (*TCPServer, error) {
	ctx := context.Background()
	lc := &net.ListenConfig{}

	listener, err := lc.Listen(ctx, "tcp", host+":0")
	if err != nil {
		return nil, fmt.Errorf("failed to create TCP listener: %w", err)
	}

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = listener.Close()

		return nil, ErrInvalidTCPAddress
	}

	return &TCPServer{
		listener: listener,
		port:     addr.Port,
		done:     make(chan struct{}),
	}, nil
}

// Start begins accepting connections on the TCP echo server.
func (s *TCPServer) Start() {
	slog.Debug("Starting TCP echo server...", "port", s.port)

	s.wg.Add(1)

	go s.acceptConnections()
}

// Stop gracefully shuts down the TCP echo server.
func (s *TCPServer) Stop() {
	slog.Debug("Stopping TCP echo server...")

	close(s.done)

	if err := s.listener.Close(); err != nil {
		slog.Error("Error closing TCP listener", "err", err)
	}

	s.wg.Wait()

	slog.Debug("TCP echo server stopped")
}

// Address returns the TCP server's listening address.
func (s *TCPServer) Address() string {
	return s.listener.Addr().String()
}

// Port returns the port number on which the TCP server is listening.
func (s *TCPServer) Port() int {
	return s.port
}

// Host returns the hostname on which the TCP server is listening.
func (s *TCPServer) Host() string {
	return host
}

// acceptConnections handles incoming TCP connections.
func (s *TCPServer) acceptConnections() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				slog.Error("Error accepting TCP connection", "err", err)

				continue
			}
		}

		s.wg.Add(1)

		go s.handleConnection(conn)
	}
}

// handleConnection reads data from the client and writes it back (echoes it).
func (s *TCPServer) handleConnection(conn net.Conn) {
	defer func() {
		s.wg.Done()

		_ = conn.Close()
	}()

	remoteAddr := conn.RemoteAddr().String()
	slog.Debug("New connection established", "addr", remoteAddr)

	deadline := time.Now().Add(defaultConnectionTimeout)

	if err := conn.SetReadDeadline(deadline); err != nil {
		slog.Info("Failed to set read deadline", "err", err)
	}

	if err := conn.SetWriteDeadline(deadline); err != nil {
		slog.Info("Failed to set write deadline", "err", err)
	}

	if _, err := io.Copy(conn, conn); err != nil && !isConnectionClosed(err) {
		slog.Error("Error during echo operation", "err", err, "addr", remoteAddr)
	} else {
		slog.Debug("Connection closed", "addr", remoteAddr)
	}
}

// isConnectionClosed checks if the error indicates a closed connection.
func isConnectionClosed(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, io.EOF) {
		return true
	}

	errMsg := strings.ToLower(err.Error())

	return strings.Contains(errMsg, "use of closed network connection") ||
		strings.Contains(errMsg, "connection reset by peer") ||
		strings.Contains(errMsg, "broken pipe") ||
		strings.Contains(errMsg, "connection refused")
}
