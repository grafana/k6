package clientcert

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

const testDestination = "example.test:443"

func TestUpstreamDialer(t *testing.T) {
	t.Parallel()

	const (
		viaNone       = "none"
		viaHTTP       = "http"
		viaSchemeLess = "scheme_less"
		viaSOCKS5     = "socks5"
	)
	basic := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))

	tests := []struct {
		name        string
		via         string
		proxy       connectProxyOptions
		credentials [2]string
		hello       bool
		wantErr     string
		wantAuth    []string
	}{
		{
			name: "no_proxy",
			via:  viaNone,
		},
		{
			name:     "http_proxy",
			via:      viaHTTP,
			wantAuth: []string{""},
		},
		{
			name:     "scheme_less_proxy",
			via:      viaSchemeLess,
			wantAuth: []string{""},
		},
		{
			name:  "socks5_proxy",
			via:   viaSOCKS5,
			hello: true,
		},
		{
			name:        "proxy_407_with_credentials",
			via:         viaHTTP,
			proxy:       connectProxyOptions{requireAuth: basic},
			credentials: [2]string{"user", "pass"},
			wantAuth:    []string{"", basic},
		},
		{
			name:     "proxy_407_without_credentials",
			via:      viaHTTP,
			proxy:    connectProxyOptions{requireAuth: basic},
			wantErr:  "requires authentication (407)",
			wantAuth: []string{""},
		},
		{
			name:     "proxy_connect_refused_403",
			via:      viaHTTP,
			proxy:    connectProxyOptions{status: 403},
			wantErr:  "403",
			wantAuth: []string{""},
		},
		{
			name:        "proxy_407_stalled_body",
			via:         viaHTTP,
			proxy:       connectProxyOptions{requireAuth: basic, stalled407: true},
			credentials: [2]string{"user", "pass"},
			wantAuth:    []string{"", basic},
		},
		{
			name:     "connect_200_with_content_length",
			via:      viaHTTP,
			proxy:    connectProxyOptions{contentLength: true},
			hello:    true,
			wantAuth: []string{""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target := startEchoAfterEOFServer(t)
			if tt.hello {
				target = startHelloServer(t)
			}
			routes := map[string]string{testDestination: target}

			direct := &recordingDialer{routes: routes}
			cfg := UpstreamConfig{
				Credentials: func() (string, string) { return tt.credentials[0], tt.credentials[1] },
				LookupEnv:   func(string) (string, bool) { return "", false },
				GOOS:        "linux",
			}
			var (
				proxy     *connectProxy
				socksDial *recordingDialer
				wantDial  []string
			)
			switch tt.via {
			case viaNone:
				wantDial = []string{testDestination}
			case viaHTTP, viaSchemeLess:
				proxy = startConnectProxy(t, tt.proxy, routes)
				cfg.ProxyServer = proxy.addr
				if tt.via == viaHTTP {
					cfg.ProxyServer = "http://" + proxy.addr
				}
				for range tt.wantAuth {
					wantDial = append(wantDial, proxy.addr)
				}
			case viaSOCKS5:
				socksDial = &recordingDialer{routes: routes}
				srv, err := ListenSOCKS5(socksDial.dial, Pipe, log.NewNullLogger())
				require.NoError(t, err)
				t.Cleanup(func() { _ = srv.Close() })
				cfg.ProxyServer = "socks5://" + srv.Addr()
				wantDial = []string{srv.Addr()}
			}

			dial, err := NewUpstreamDialer(cfg, direct.dial)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			start := time.Now()
			conn, err := dial(ctx, "tcp", testDestination)
			elapsed := time.Since(start)
			if conn != nil {
				t.Cleanup(func() { _ = conn.Close() })
			}

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Less(t, elapsed, 2*time.Second)
				if tt.hello {
					assertReads(t, conn, "hello")
				} else {
					exchange(t, conn)
				}
			}

			assert.Equal(t, wantDial, direct.dialed())
			if socksDial != nil {
				assert.Equal(t, []string{testDestination}, socksDial.dialed())
			}
			if proxy == nil {
				return
			}
			reqs := proxy.recorded()
			require.Len(t, reqs, len(tt.wantAuth))
			for i, req := range reqs {
				assert.Equal(t, "CONNECT "+testDestination+" HTTP/1.1", req.line)
				assert.Equal(t, testDestination, req.header.Get("Host"))
				assert.Equal(t, tt.wantAuth[i], req.header.Get("Proxy-Authorization"))
			}
			open := int32(1)
			if tt.wantErr != "" {
				open = 0
			}
			assert.Eventually(t, func() bool {
				return proxy.closed.Load() == proxy.accepted.Load()-open
			}, 2*time.Second, 10*time.Millisecond, "proxy connections left open")
		})
	}
}

func TestUpstreamDialer_ContextCancelled(t *testing.T) {
	t.Parallel()

	ca := newTestCA(t, "proxy CA")
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{ca.issue(t, "proxy", ecdsaSEC1, x509.ExtKeyUsageServerAuth)},
		MinVersion:   tls.VersionTLS12,
	}

	tests := []struct {
		name   string
		scheme string
		proxy  connectProxyOptions
	}{
		{
			name:   "http_proxy_never_replies",
			scheme: "http",
			proxy:  connectProxyOptions{neverReply: true},
		},
		{
			name:   "https_proxy_stalls_handshake",
			scheme: "https",
			proxy:  connectProxyOptions{stallHandshake: true},
		},
		{
			name:   "https_proxy_stalls_after_connect",
			scheme: "https",
			proxy:  connectProxyOptions{tls: tlsConfig, neverReply: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			proxy := startConnectProxy(t, tt.proxy, nil)
			direct := &recordingDialer{}
			cfg := UpstreamConfig{
				ProxyServer: tt.scheme + "://" + proxy.addr,
				Credentials: func() (string, string) { return "", "" },
				LookupEnv:   func(string) (string, bool) { return "", false },
				GOOS:        "linux",
				rootCAs:     ca.pool,
			}
			dial, err := NewUpstreamDialer(cfg, direct.dial)
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			type result struct {
				conn net.Conn
				err  error
			}
			done := make(chan result, 1)
			go func() {
				conn, err := dial(ctx, "tcp", testDestination)
				done <- result{conn, err}
			}()

			require.Eventually(t, func() bool {
				if tt.proxy.stallHandshake {
					return proxy.accepted.Load() == 1
				}
				return len(proxy.recorded()) == 1
			}, 2*time.Second, 10*time.Millisecond, "proxy did not see the connection")
			cancel()

			select {
			case r := <-done:
				if r.conn != nil {
					_ = r.conn.Close()
				}
				require.ErrorIs(t, r.err, context.Canceled)
			case <-time.After(2 * time.Second):
				require.FailNow(t, "dial did not return after cancellation")
			}
			assert.Eventually(t, func() bool {
				return proxy.closed.Load() == 1
			}, 2*time.Second, 10*time.Millisecond, "proxy connection left open")
		})
	}
}

func TestUpstreamDialer_HalfClose(t *testing.T) {
	t.Parallel()

	ca := newTestCA(t, "proxy CA")
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{ca.issue(t, "proxy", ecdsaSEC1, x509.ExtKeyUsageServerAuth)},
		MinVersion:   tls.VersionTLS12,
	}

	tests := []struct {
		name   string
		scheme string
		tls    *tls.Config
	}{
		{name: "http_proxy", scheme: "http"},
		{name: "https_proxy", scheme: "https", tls: tlsConfig},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			routes := map[string]string{testDestination: startEchoAfterEOFServer(t)}
			proxy := startConnectProxy(t, connectProxyOptions{tls: tt.tls}, routes)
			direct := &recordingDialer{}
			cfg := UpstreamConfig{
				ProxyServer: tt.scheme + "://" + proxy.addr,
				Credentials: func() (string, string) { return "", "" },
				LookupEnv:   func(string) (string, bool) { return "", false },
				GOOS:        "linux",
				rootCAs:     ca.pool,
			}
			dial, err := NewUpstreamDialer(cfg, direct.dial)
			require.NoError(t, err)

			conn, err := dial(t.Context(), "tcp", testDestination)
			require.NoError(t, err)
			t.Cleanup(func() { _ = conn.Close() })

			exchange(t, conn)
			assert.Equal(t, int32(0), proxy.closed.Load())

			require.NoError(t, conn.Close())
			assert.Eventually(t, func() bool {
				return proxy.closed.Load() == 1
			}, 2*time.Second, 10*time.Millisecond, "proxy connection left open")
		})
	}
}

func TestUpstreamDialer_Environment(t *testing.T) {
	t.Parallel()

	const (
		wantDirect  = "direct"
		wantEnv     = "env"
		wantContext = "context"
	)

	tests := []struct {
		name     string
		env      map[string]string
		goos     string
		context  bool
		loopback bool
		want     string
	}{
		{
			name: "linux_https_proxy",
			env:  map[string]string{"HTTPS_PROXY": "envproxy"},
			goos: "linux",
			want: wantEnv,
		},
		{
			name: "linux_lowercase",
			env:  map[string]string{"https_proxy": "envproxy"},
			goos: "linux",
			want: wantEnv,
		},
		{
			name: "linux_no_proxy_match",
			env:  map[string]string{"HTTPS_PROXY": "envproxy", "NO_PROXY": "example.test"},
			goos: "linux",
			want: wantDirect,
		},
		{
			name:     "linux_loopback_never_proxied",
			env:      map[string]string{"HTTPS_PROXY": "envproxy"},
			goos:     "linux",
			loopback: true,
			want:     wantDirect,
		},
		{
			name: "darwin_ignored",
			env:  map[string]string{"HTTPS_PROXY": "envproxy"},
			goos: "darwin",
			want: wantDirect,
		},
		{
			name: "windows_ignored",
			env:  map[string]string{"HTTPS_PROXY": "envproxy"},
			goos: "windows",
			want: wantDirect,
		},
		{
			name:    "context_proxy_wins",
			env:     map[string]string{"HTTPS_PROXY": "envproxy"},
			goos:    "linux",
			context: true,
			want:    wantContext,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target := startEchoAfterEOFServer(t)
			routes := map[string]string{testDestination: target}
			envProxy := startConnectProxy(t, connectProxyOptions{}, routes)
			contextProxy := startConnectProxy(t, connectProxyOptions{}, routes)

			env := make(map[string]string, len(tt.env))
			for k, v := range tt.env {
				env[k] = strings.ReplaceAll(v, "envproxy", "http://"+envProxy.addr)
			}
			cfg := UpstreamConfig{
				Credentials: func() (string, string) { return "", "" },
				LookupEnv: func(key string) (string, bool) {
					v, ok := env[key]
					return v, ok
				},
				GOOS: tt.goos,
			}
			if tt.context {
				cfg.ProxyServer = "http://" + contextProxy.addr
			}
			direct := &recordingDialer{routes: routes}
			dial, err := NewUpstreamDialer(cfg, direct.dial)
			require.NoError(t, err)

			address := testDestination
			if tt.loopback {
				address = target
			}
			conn, err := dial(t.Context(), "tcp", address)
			require.NoError(t, err)
			t.Cleanup(func() { _ = conn.Close() })
			exchange(t, conn)

			var wantDial string
			switch tt.want {
			case wantDirect:
				wantDial = address
			case wantEnv:
				wantDial = envProxy.addr
			case wantContext:
				wantDial = contextProxy.addr
			}
			assert.Equal(t, []string{wantDial}, direct.dialed())
			assert.Len(t, envProxy.recorded(), boolToInt(tt.want == wantEnv))
			assert.Len(t, contextProxy.recorded(), boolToInt(tt.want == wantContext))
		})
	}
}

func TestNewUpstreamDialer_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		server  string
		wantErr bool
	}{
		{name: "socks4", server: "socks4://127.0.0.1:1080", wantErr: true},
		{name: "ftp", server: "ftp://127.0.0.1:21", wantErr: true},
		{name: "unparsable", server: "http://[::1", wantErr: true},
		{name: "https", server: "https://127.0.0.1:8443"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var d net.Dialer
			_, err := NewUpstreamDialer(UpstreamConfig{ProxyServer: tt.server}, d.DialContext)
			if tt.wantErr {
				assert.ErrorContains(t, err, "unsupported proxy server")
				return
			}
			assert.NoError(t, err)
		})
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// exchange sends "ping" through conn, half-closes it, and expects the echo-after-EOF reply.
func exchange(t *testing.T, conn net.Conn) {
	t.Helper()
	_, err := conn.Write([]byte("ping"))
	require.NoError(t, err)
	cw, ok := conn.(interface{ CloseWrite() error })
	require.True(t, ok, "connection cannot be half-closed")
	require.NoError(t, cw.CloseWrite())
	assertReads(t, conn, "response:ping")
}

func assertReads(t *testing.T, conn net.Conn, want string) {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	got := make([]byte, len(want))
	_, err := io.ReadFull(conn, got)
	require.NoError(t, err)
	assert.Equal(t, want, string(got))
}

// startHelloServer listens on 127.0.0.1 and writes "hello" to each connection as soon as it
// is accepted, then reads until EOF.
func startHelloServer(t *testing.T) string {
	t.Helper()
	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	var wg sync.WaitGroup
	t.Cleanup(func() {
		_ = l.Close()
		wg.Wait()
	})
	wg.Go(func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			wg.Go(func() {
				defer func() { _ = c.Close() }()
				if _, err := c.Write([]byte("hello")); err != nil {
					return
				}
				_, _ = io.Copy(io.Discard, c)
			})
		}
	})
	return l.Addr().String()
}

// recordingDialer records the addresses it dials and connects to routes[address], or to the
// address itself when it has no route.
type recordingDialer struct {
	routes map[string]string

	mu    sync.Mutex
	addrs []string
}

func (d *recordingDialer) dial(ctx context.Context, network, address string) (net.Conn, error) {
	d.mu.Lock()
	d.addrs = append(d.addrs, address)
	d.mu.Unlock()
	if r, ok := d.routes[address]; ok {
		address = r
	}
	var nd net.Dialer
	return nd.DialContext(ctx, network, address)
}

func (d *recordingDialer) dialed() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.addrs...)
}

type connectProxyOptions struct {
	// tls makes the proxy accept TLS connections.
	tls *tls.Config
	// status answers every CONNECT with this status instead of 200.
	status int
	// requireAuth answers 407 unless Proxy-Authorization equals it.
	requireAuth string
	// stalled407 makes the 407 declare a 1000-byte body and send only 10 bytes of it.
	stalled407 bool
	// contentLength adds Content-Length: 5 to the 200 and sends the destination's first
	// bytes in the same write.
	contentLength bool
	// neverReply reads the CONNECT and never answers.
	neverReply bool
	// stallHandshake accepts connections and never completes the TLS handshake.
	stallHandshake bool
}

type connectRequest struct {
	line   string
	header textproto.MIMEHeader
}

// connectProxy is an HTTP CONNECT proxy that reads one request per connection. A connection
// counts as closed once the proxy observes the client closing it.
type connectProxy struct {
	addr     string
	opts     connectProxyOptions
	dest     *recordingDialer
	accepted atomic.Int32
	closed   atomic.Int32

	mu       sync.Mutex
	requests []connectRequest
	conns    map[net.Conn]struct{}
}

func startConnectProxy(t *testing.T, opts connectProxyOptions, routes map[string]string) *connectProxy {
	t.Helper()
	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	p := &connectProxy{
		addr:  l.Addr().String(),
		opts:  opts,
		dest:  &recordingDialer{routes: routes},
		conns: make(map[net.Conn]struct{}),
	}
	var wg sync.WaitGroup
	t.Cleanup(func() {
		_ = l.Close()
		p.mu.Lock()
		for c := range p.conns {
			_ = c.Close()
		}
		p.mu.Unlock()
		wg.Wait()
	})
	wg.Go(func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			p.accepted.Add(1)
			p.track(c)
			wg.Go(func() {
				defer func() { _ = c.Close() }()
				p.handle(c)
				p.closed.Add(1)
			})
		}
	})
	return p
}

func (p *connectProxy) track(c net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.conns[c] = struct{}{}
}

func (p *connectProxy) recorded() []connectRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]connectRequest(nil), p.requests...)
}

// handle serves one connection and returns once the client has closed it.
func (p *connectProxy) handle(raw net.Conn) {
	if p.opts.stallHandshake {
		_, _ = io.Copy(io.Discard, raw)
		return
	}
	conn := raw
	if p.opts.tls != nil {
		tc := tls.Server(raw, p.opts.tls)
		if err := tc.HandshakeContext(context.Background()); err != nil {
			return
		}
		conn = tc
	}
	br := bufio.NewReader(conn)
	tp := textproto.NewReader(br)
	line, err := tp.ReadLine()
	if err != nil {
		return
	}
	header, err := tp.ReadMIMEHeader()
	if err != nil {
		return
	}
	p.mu.Lock()
	p.requests = append(p.requests, connectRequest{line: line, header: header})
	p.mu.Unlock()

	reject := func(response string) {
		if response != "" {
			_, _ = io.WriteString(conn, response)
		}
		_, _ = io.Copy(io.Discard, br)
	}
	switch {
	case p.opts.neverReply:
		reject("")
		return
	case p.opts.status != 0:
		reject(fmt.Sprintf("HTTP/1.1 %d Forbidden\r\nContent-Length: 0\r\n\r\n", p.opts.status))
		return
	case p.opts.requireAuth != "" && header.Get("Proxy-Authorization") != p.opts.requireAuth:
		const head = "HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic realm=\"k6\"\r\n"
		if p.opts.stalled407 {
			reject(head + "Content-Length: 1000\r\n\r\n" + strings.Repeat("x", 10))
		} else {
			reject(head + "Content-Length: 0\r\n\r\n")
		}
		return
	}

	fields := strings.Fields(line)
	if len(fields) != 3 {
		return
	}
	dest, err := p.dest.dial(context.Background(), "tcp", fields[1])
	if err != nil {
		return
	}
	defer func() { _ = dest.Close() }()
	p.track(dest)

	response := "HTTP/1.1 200 Connection established\r\n\r\n"
	if p.opts.contentLength {
		first := make([]byte, 5)
		if _, err := io.ReadFull(dest, first); err != nil {
			return
		}
		response = "HTTP/1.1 200 Connection established\r\nContent-Length: 5\r\n\r\n" + string(first)
	}
	if _, err := io.WriteString(conn, response); err != nil {
		return
	}

	var wg sync.WaitGroup
	wg.Go(func() {
		_, _ = io.Copy(dest, br)
		_ = dest.(*net.TCPConn).CloseWrite()
	})
	_, _ = io.Copy(conn, dest)
	// Probe until a write fails, which happens once the client has closed the connection.
	for {
		if _, err := conn.Write([]byte{0}); err != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = raw.Close()
	wg.Wait()
}
