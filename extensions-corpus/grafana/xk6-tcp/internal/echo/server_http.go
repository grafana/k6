package echo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
)

// ErrInvalidHTTPAddress is returned when the HTTP address cannot be determined.
var ErrInvalidHTTPAddress = errors.New("failed to get HTTP address")

// HTTPServer manages an HTTP echo server instance.
type HTTPServer struct {
	listener net.Listener
	server   *http.Server
	port     int
}

// NewHTTPServer creates a new HTTP echo server instance.
func NewHTTPServer() (*HTTPServer, error) {
	ctx := context.Background()
	lc := &net.ListenConfig{}

	listener, err := lc.Listen(ctx, "tcp", host+":0")
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP listener: %w", err)
	}

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = listener.Close()

		return nil, ErrInvalidHTTPAddress
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", httpEchoHandler)

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: defaultHTTPTimeout,
	}

	return &HTTPServer{
		listener: listener,
		server:   server,
		port:     addr.Port,
	}, nil
}

// Start begins accepting connections on the HTTP echo server.
func (s *HTTPServer) Start() {
	slog.Debug("Starting HTTP echo server...", "port", s.port)

	go func() {
		if err := s.server.Serve(s.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP echo server error", "err", err)
		}
	}()
}

// Stop gracefully shuts down the HTTP echo server.
func (s *HTTPServer) Stop() {
	slog.Debug("Stopping HTTP echo server...")

	ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		slog.Error("Error shutting down HTTP server", "err", err)
	}

	slog.Debug("HTTP echo server stopped")
}

// Address returns the HTTP server's listening address.
func (s *HTTPServer) Address() string {
	return s.listener.Addr().String()
}

// Port returns the port number on which the HTTP server is listening.
func (s *HTTPServer) Port() int {
	return s.port
}

// URL returns the full HTTP URL for the echo server.
func (s *HTTPServer) URL() string {
	return "http://" + net.JoinHostPort(host, strconv.Itoa(s.port))
}

// Host returns the hostname on which the HTTP server is listening.
func (s *HTTPServer) Host() string {
	return host
}

// httpEchoHandler handles HTTP GET requests and returns request information as JSON.
func httpEchoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	query := make(map[string]string)

	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			query[k] = v[0]
		}
	}

	headers := make(map[string]string)

	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	response := map[string]any{
		"method":  r.Method,
		"url":     r.URL.String(),
		"path":    r.URL.Path,
		"query":   query,
		"headers": headers,
		"host":    r.Host,
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("Failed to encode JSON response", "err", err)
	}
}
