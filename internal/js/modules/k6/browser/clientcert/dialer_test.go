package clientcert

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/lib/testutils/connectproxy"
)

func TestUpstreamDialer(t *testing.T) {
	t.Parallel()

	const (
		viaNone       = "none"
		viaHTTP       = "http"
		viaSchemeLess = "scheme_less"
	)

	tests := []struct {
		name         string
		via          string
		proxyAuth    bool
		credentials  [2]string
		closedTarget bool
		wantErr      string
		wantDials    int
		wantTargets  int
	}{
		{
			name: "no_proxy",
			via:  viaNone,
		},
		{
			name:        "http_proxy",
			via:         viaHTTP,
			wantDials:   1,
			wantTargets: 1,
		},
		{
			name:        "scheme_less_proxy",
			via:         viaSchemeLess,
			wantDials:   1,
			wantTargets: 1,
		},
		{
			name:        "proxy_407_with_credentials",
			via:         viaHTTP,
			proxyAuth:   true,
			credentials: [2]string{"user", "pass"},
			wantDials:   2,
			wantTargets: 1,
		},
		{
			name:      "proxy_407_without_credentials",
			via:       viaHTTP,
			proxyAuth: true,
			wantErr:   "requires authentication (407)",
			wantDials: 1,
		},
		{
			name:        "proxy_407_with_wrong_credentials",
			via:         viaHTTP,
			proxyAuth:   true,
			credentials: [2]string{"user", "wrong"},
			wantErr:     "407",
			wantDials:   2,
		},
		{
			name:         "proxy_502_unreachable_target",
			via:          viaHTTP,
			closedTarget: true,
			wantErr:      "502",
			wantDials:    1,
			wantTargets:  1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target := startEchoAfterEOFServer(t)
			if tt.closedTarget {
				target = closedAddr(t)
			}
			_, port, err := net.SplitHostPort(target)
			require.NoError(t, err)
			// The hostname reaches the proxy unresolved.
			destination := net.JoinHostPort("localhost", port)

			direct := &recordingDialer{}
			cfg := UpstreamConfig{
				Credentials: func() (string, string) { return tt.credentials[0], tt.credentials[1] },
			}
			var (
				proxy     *connectproxy.Server
				proxyAddr string
				wantDial  = []string{destination}
			)
			if tt.via != viaNone {
				opts := connectproxy.Options{}
				if tt.proxyAuth {
					opts = connectproxy.Options{Username: "user", Password: "pass"}
				}
				proxy = connectproxy.New(t, opts)
				proxyAddr = strings.TrimPrefix(proxy.URL(), "http://")
				cfg.ProxyServer = proxy.URL()
				if tt.via == viaSchemeLess {
					cfg.ProxyServer = proxyAddr
				}
				wantDial = slices.Repeat([]string{proxyAddr}, tt.wantDials)
			}

			dial, err := NewUpstreamDialer(cfg, direct.dial)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			conn, err := dial(ctx, "tcp", destination)
			if conn != nil {
				t.Cleanup(func() { _ = conn.Close() })
			}

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.NotContains(t, err.Error(), proxyAddr)
				assert.NotContains(t, err.Error(), "pass")
			} else {
				require.NoError(t, err)
				exchange(t, conn)
			}
			assert.Equal(t, wantDial, direct.dialed())
			if proxy != nil {
				targets := proxy.Targets()
				assert.Len(t, targets, tt.wantTargets)
				for _, target := range targets {
					assert.Equal(t, destination, target)
				}
			}
		})
	}
}

func TestUpstreamDialer_ContextCancelled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		scheme string
		proxy  fakeProxyOptions
	}{
		{
			name:   "http_proxy_never_replies",
			scheme: "http",
			proxy:  fakeProxyOptions{neverReply: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			proxy := startFakeProxy(t, tt.proxy)
			cfg := UpstreamConfig{
				ProxyServer: tt.scheme + "://" + proxy.addr,
			}
			var d net.Dialer
			dial, err := NewUpstreamDialer(cfg, d.DialContext)
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			type result struct {
				conn net.Conn
				err  error
			}
			done := make(chan result, 1)
			go func() {
				conn, err := dial(ctx, "tcp", "example.test:443")
				done <- result{conn, err}
			}()

			require.Eventually(t, func() bool {
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

func TestNewUpstreamDialer_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		server  string
		wantErr string
	}{
		{
			name:    "https",
			server:  "https://user:secret@127.0.0.1:443",
			wantErr: "unsupported proxy server scheme",
		},
		{
			name:    "socks5",
			server:  "socks5://user:secret@127.0.0.1:1080",
			wantErr: "unsupported proxy server scheme",
		},
		{
			name:    "socks5h",
			server:  "socks5h://user:secret@127.0.0.1:1080",
			wantErr: "unsupported proxy server scheme",
		},
		{
			name:    "ftp",
			server:  "ftp://user:secret@127.0.0.1:21",
			wantErr: "unsupported proxy server scheme",
		},
		{
			name:    "unparsable",
			server:  "http://user:secret@[::1",
			wantErr: "unsupported proxy server",
		},
		{
			name:    "missing_host",
			server:  "http://user:secret@:8080",
			wantErr: "missing host",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var d net.Dialer
			_, err := NewUpstreamDialer(UpstreamConfig{ProxyServer: tt.server}, d.DialContext)
			require.ErrorContains(t, err, tt.wantErr)
			assert.NotContains(t, err.Error(), "secret")
			assert.NotContains(t, err.Error(), "127.0.0.1")
		})
	}
}

// exchange sends "ping" through conn, half-closes it, and expects the echo-after-EOF reply.
func exchange(t *testing.T, conn net.Conn) {
	t.Helper()
	_, err := conn.Write([]byte("ping"))
	require.NoError(t, err)
	cw, ok := conn.(interface{ CloseWrite() error })
	require.True(t, ok, "connection cannot be half-closed")
	require.NoError(t, cw.CloseWrite())
	assert.Equal(t, "response:ping", string(readWithin(t, conn, len("response:ping"))))
}

// readWithin reads n bytes from conn, failing after 5 seconds.
func readWithin(t *testing.T, conn net.Conn, n int) []byte {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	return readN(t, conn, n)
}

// closedAddr returns a loopback address nothing listens on.
func closedAddr(t *testing.T) string {
	t.Helper()
	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return addr
}

// startEchoAfterEOFServer listens on 127.0.0.1 and answers each connection with "response:"
// followed by everything it read, once the client has half-closed.
func startEchoAfterEOFServer(t *testing.T) string {
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
				req, err := io.ReadAll(c)
				if err != nil {
					return
				}
				_, _ = c.Write(append([]byte("response:"), req...))
			})
		}
	})
	return l.Addr().String()
}

// recordingDialer records the addresses it dials.
type recordingDialer struct {
	mu    sync.Mutex
	addrs []string
}

func (d *recordingDialer) dial(ctx context.Context, network, address string) (net.Conn, error) {
	d.mu.Lock()
	d.addrs = append(d.addrs, address)
	d.mu.Unlock()
	var nd net.Dialer
	return nd.DialContext(ctx, network, address)
}

func (d *recordingDialer) dialed() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.addrs...)
}

type fakeProxyOptions struct {
	// neverReply reads the CONNECT and never answers.
	neverReply bool
}

// fakeProxy is an HTTP CONNECT proxy that reads one request per connection. A connection
// counts as closed once the proxy observes the client closing it.
type fakeProxy struct {
	addr     string
	opts     fakeProxyOptions
	accepted atomic.Int32
	closed   atomic.Int32

	mu       sync.Mutex
	requests []*http.Request
	conns    map[net.Conn]struct{}
}

func startFakeProxy(t *testing.T, opts fakeProxyOptions) *fakeProxy {
	t.Helper()
	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	p := &fakeProxy{addr: l.Addr().String(), opts: opts, conns: make(map[net.Conn]struct{})}
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

func (p *fakeProxy) track(c net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.conns[c] = struct{}{}
}

func (p *fakeProxy) recorded() []*http.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]*http.Request(nil), p.requests...)
}

// handle serves one connection and returns once the client has closed it.
func (p *fakeProxy) handle(raw net.Conn) {
	conn := raw
	br := bufio.NewReader(conn)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	p.mu.Lock()
	p.requests = append(p.requests, req)
	p.mu.Unlock()

	reject := func(response string) {
		_, _ = io.WriteString(conn, response)
		_, _ = io.Copy(io.Discard, br)
	}
	if p.opts.neverReply {
		reject("")
		return
	}

	var d net.Dialer
	dest, err := d.DialContext(context.Background(), "tcp", req.Host)
	if err != nil {
		return
	}
	defer func() { _ = dest.Close() }()
	p.track(dest)

	response := "HTTP/1.1 200 Connection established\r\n\r\n"
	if _, err := io.WriteString(conn, response); err != nil {
		return
	}

	var wg sync.WaitGroup
	wg.Go(func() {
		_, _ = io.Copy(dest, br)
		if cw, ok := dest.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
		}
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
