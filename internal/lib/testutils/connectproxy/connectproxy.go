// Package connectproxy provides an HTTP CONNECT proxy server for tests.
// Test-only, do not import in production code.
package connectproxy

import (
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// Options configures a Server.
type Options struct {
	// Username and Password are the Basic credentials required in Proxy-Authorization.
	// An empty Username disables authentication.
	Username, Password string
}

// Server is an HTTP CONNECT proxy for tests. It records each CONNECT target and can require Basic
// proxy authentication.
type Server struct {
	srv      *httptest.Server
	wantAuth string

	mu      sync.Mutex
	targets []string
	closed  bool
	conns   map[net.Conn]struct{}
	tunnels sync.WaitGroup
}

// New starts a Server. It is closed, together with its open tunnels, when t finishes.
func New(t testing.TB, opts Options) *Server {
	t.Helper()

	s := &Server{conns: make(map[net.Conn]struct{})}
	if opts.Username != "" {
		s.wantAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(opts.Username+":"+opts.Password))
	}
	s.srv = httptest.NewServer(http.HandlerFunc(s.serveHTTP))
	t.Cleanup(s.close)

	return s
}

// URL returns the proxy URL, http://127.0.0.1:N.
func (s *Server) URL() string {
	return s.srv.URL
}

// Targets returns the targets of the CONNECT requests that passed authentication, in order.
func (s *Server) Targets() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]string(nil), s.targets...)
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodConnect {
		http.Error(w, "CONNECT only", http.StatusMethodNotAllowed)
		return
	}
	if s.wantAuth != "" && r.Header.Get("Proxy-Authorization") != s.wantAuth {
		w.Header().Set("Proxy-Authenticate", `Basic realm="test"`)
		w.WriteHeader(http.StatusProxyAuthRequired)
		return
	}
	s.mu.Lock()
	s.targets = append(s.targets, r.Host)
	s.mu.Unlock()

	target, err := (&net.Dialer{}).DialContext(r.Context(), "tcp", r.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	client, buf, err := http.NewResponseController(w).Hijack()
	if err != nil {
		_ = target.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection established\r\n\r\n"); err != nil {
		_ = client.Close()
		_ = target.Close()
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		_ = client.Close()
		_ = target.Close()
		return
	}
	s.conns[client] = struct{}{}
	s.conns[target] = struct{}{}
	s.tunnels.Go(func() {
		tunnel(client, buf.Reader, target)
		s.mu.Lock()
		delete(s.conns, client)
		delete(s.conns, target)
		s.mu.Unlock()
	})
}

// close stops the server and closes the tunnels still open.
func (s *Server) close() {
	s.srv.Close()
	s.mu.Lock()
	s.closed = true
	for c := range s.conns {
		_ = c.Close()
	}
	s.mu.Unlock()
	s.tunnels.Wait()
}

// tunnel copies between client and target in both directions, forwarding half-closes, and
// closes both once both directions finish. clientReader reads from client, including bytes
// buffered before the hijack.
func tunnel(client net.Conn, clientReader io.Reader, target net.Conn) {
	var wg sync.WaitGroup
	copyHalf := func(dst net.Conn, src io.Reader) {
		_, _ = io.Copy(dst, src)
		if cw, ok := dst.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
		}
	}
	wg.Go(func() { copyHalf(target, clientReader) })
	wg.Go(func() { copyHalf(client, target) })
	wg.Wait()
	_ = client.Close()
	_ = target.Close()
}
