// Package echo provides TCP and HTTP echo server implementations that can be embedded
// in applications or used for testing purposes. The servers listen on random
// localhost ports and echo back data received from clients.
package echo

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"time"
)

const (
	// EnvTCPEchoHost is the environment variable used to set the TCP echo server host.
	EnvTCPEchoHost = "TCP_ECHO_HOST"

	// EnvTCPEchoPort is the environment variable used to set the TCP echo server port.
	EnvTCPEchoPort = "TCP_ECHO_PORT"

	// EnvHTTPEchoURL is the environment variable used to set the HTTP echo server URL.
	EnvHTTPEchoURL = "HTTP_ECHO_URL"

	// EnvHTTPEchoHost is the environment variable used to set the HTTP echo server host.
	EnvHTTPEchoHost = "HTTP_ECHO_HOST"

	// EnvHTTPEchoPort is the environment variable used to set the HTTP echo server port.
	EnvHTTPEchoPort = "HTTP_ECHO_PORT"

	// defaultConnectionTimeout is the default timeout for TCP connections.
	defaultConnectionTimeout = 5 * time.Second

	// defaultHTTPTimeout is the default timeout for HTTP server operations.
	defaultHTTPTimeout = 5 * time.Second

	host = "localhost"
)

// Server manages TCP and HTTP echo server instances.
type Server struct {
	tcp  *TCPServer
	http *HTTPServer
}

// Setup initializes echo server for testing purposes.
func Setup() *Server {
	slog.Debug("Setting up echo server for tests")

	server, err := New()
	if err != nil {
		panic(fmt.Errorf("failed to start embedded echo server: %w", err))
	}

	if err := server.Setenv(); err != nil {
		panic(fmt.Errorf("failed to set up embedded echo server environment: %w", err))
	}

	server.Start()

	return server
}

// New creates a new echo server instance with both TCP and HTTP servers.
func New() (*Server, error) {
	tcp, err := NewTCPServer()
	if err != nil {
		return nil, err
	}

	http, err := NewHTTPServer()
	if err != nil {
		tcp.Stop()

		return nil, err
	}

	return &Server{
		tcp:  tcp,
		http: http,
	}, nil
}

// Start begins accepting connections on both TCP and HTTP echo servers.
func (s *Server) Start() {
	slog.Debug("Starting echo servers...")
	s.tcp.Start()
	s.http.Start()
	slog.Debug("Echo servers started")
}

// Stop gracefully shuts down both TCP and HTTP echo servers.
func (s *Server) Stop() {
	slog.Debug("Stopping echo servers...")
	s.tcp.Stop()
	s.http.Stop()
	slog.Debug("Echo servers stopped")
}

// Address returns the TCP server's listening address.
func (s *Server) Address() string {
	return s.tcp.Address()
}

// Port returns the port number on which the TCP server is listening.
func (s *Server) Port() int {
	return s.tcp.Port()
}

// HTTPPort returns the port number on which the HTTP server is listening.
func (s *Server) HTTPPort() int {
	return s.http.Port()
}

// HTTPURL returns the full HTTP URL for the echo server.
func (s *Server) HTTPURL() string {
	return s.http.URL()
}

// Host returns the hostname on which the servers are listening.
func (s *Server) Host() string {
	return host
}

// Setenv sets environment variables for both TCP and HTTP echo servers.
func (s *Server) Setenv() error {
	// TCP server environment variables
	if err := setenv(EnvTCPEchoHost, host); err != nil {
		return err
	}

	if err := setenv(EnvTCPEchoPort, strconv.Itoa(s.tcp.port)); err != nil {
		return err
	}

	// HTTP server environment variables
	if err := setenv(EnvHTTPEchoHost, host); err != nil {
		return err
	}

	if err := setenv(EnvHTTPEchoPort, strconv.Itoa(s.http.port)); err != nil {
		return err
	}

	httpURL := "http://" + net.JoinHostPort(host, strconv.Itoa(s.http.port))

	return setenv(EnvHTTPEchoURL, httpURL)
}

func setenv(key, value string) error {
	if err := os.Setenv(key, value); err != nil { //nolint:forbidigo // test echo server exports connection details via env
		return fmt.Errorf("failed to set %s environment variable: %w", key, err)
	}

	return nil
}
