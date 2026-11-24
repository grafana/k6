package clientcert

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

func TestConnectServer(t *testing.T) {
	t.Parallel()

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	t.Cleanup(ts.Close)
	tsURL, err := url.Parse(ts.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(tsURL.Port())
	require.NoError(t, err)

	type dialCall struct{ network, address string }
	dials := make(chan dialCall, 1)
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		dials <- dialCall{network, address}
		var d net.Dialer
		return d.DialContext(ctx, network, address)
	}
	type tunnelCall struct {
		host string
		port int
	}
	tunnels := make(chan tunnelCall, 1)
	tunnel := func(ctx context.Context, browser, destination net.Conn, host string, port int) {
		tunnels <- tunnelCall{host, port}
		Pipe(ctx, browser, destination, host, port)
	}

	srv := startConnectServer(t, dial, tunnel)

	transport, ok := ts.Client().Transport.(*http.Transport)
	require.True(t, ok)
	transport = transport.Clone()
	transport.Proxy = http.ProxyURL(&url.URL{Scheme: "http", Host: srv.Addr()})
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "hello", string(body))
	call := <-dials
	assert.Equal(t, "tcp", call.network)
	assert.Equal(t, tsURL.Host, call.address)
	tc := <-tunnels
	assert.Equal(t, tsURL.Hostname(), tc.host)
	assert.Equal(t, port, tc.port)
}

func TestConnectServer_Rejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		request    string
		dialErr    error
		wantStatus int
	}{
		{
			name:       "get_method",
			request:    "GET http://127.0.0.1/ HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "missing_port",
			request:    "CONNECT 127.0.0.1 HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid_port",
			request:    "CONNECT 127.0.0.1:65536 HTTP/1.1\r\nHost: 127.0.0.1:65536\r\n\r\n",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty_host",
			request:    "CONNECT :443 HTTP/1.1\r\nHost: :443\r\n\r\n",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "dial_error",
			request:    "CONNECT 127.0.0.1:443 HTTP/1.1\r\nHost: 127.0.0.1:443\r\n\r\n",
			dialErr:    errors.New("dial failed"),
			wantStatus: http.StatusBadGateway,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dial := func(context.Context, string, string) (net.Conn, error) {
				if tt.dialErr == nil {
					return nil, errors.New("unexpected dial")
				}
				return nil, tt.dialErr
			}
			var tunneled atomic.Bool
			tunnel := func(_ context.Context, browser, destination net.Conn, _ string, _ int) {
				tunneled.Store(true)
				_ = browser.Close()
				_ = destination.Close()
			}

			srv := startConnectServer(t, dial, tunnel)
			conn := dialConnectServer(t, srv.Addr())

			_, err := conn.Write([]byte(tt.request))
			require.NoError(t, err)
			br := bufio.NewReader(conn)
			resp, err := http.ReadResponse(br, nil)
			require.NoError(t, err)
			_ = resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			assertClosed(t, br)
			assert.False(t, tunneled.Load())
		})
	}
}

func TestConnectServer_Close(t *testing.T) {
	t.Parallel()

	echoAddr := startEchoServer(t)
	dial := func(ctx context.Context, network, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, echoAddr)
	}
	tunnelDone := make(chan struct{})
	tunnel := func(ctx context.Context, browser, destination net.Conn, host string, port int) {
		defer close(tunnelDone)
		Pipe(ctx, browser, destination, host, port)
	}

	srv, err := ListenCONNECT(dial, tunnel, log.NewNullLogger())
	require.NoError(t, err)
	conn := dialConnectServer(t, srv.Addr())
	br := connectTunnel(t, conn, "127.0.0.1:80", "")

	_, err = conn.Write([]byte("ping"))
	require.NoError(t, err)
	require.Equal(t, []byte("ping"), readN(t, br, 4))

	require.NoError(t, srv.Close())

	select {
	case <-tunnelDone:
	default:
		assert.Fail(t, "Close returned before the tunnel goroutine exited")
	}
	assertClosed(t, br)
	var d net.Dialer
	if c, err := d.DialContext(t.Context(), "tcp", srv.Addr()); err == nil {
		_ = c.Close()
		assert.Fail(t, "dial to a closed server succeeded")
	}
	assert.NoError(t, srv.Close())
}

func TestPipe_HalfClose(t *testing.T) {
	t.Parallel()

	destination, server := tcpPair(t)
	dial := func(context.Context, string, string) (net.Conn, error) {
		return destination, nil
	}
	pipeDone := make(chan struct{})
	tunnel := func(ctx context.Context, browser, destination net.Conn, host string, port int) {
		defer close(pipeDone)
		Pipe(ctx, browser, destination, host, port)
	}

	// The destination half-closes first, so the browser must keep writing after reading EOF.
	serverErr := make(chan error, 1)
	serverGot := make(chan string, 1)
	go func() {
		defer func() { _ = server.Close() }()
		if _, err := server.Write([]byte("response")); err != nil {
			serverErr <- err
			return
		}
		if err := server.CloseWrite(); err != nil {
			serverErr <- err
			return
		}
		req, err := io.ReadAll(server)
		serverGot <- string(req)
		serverErr <- err
	}()

	srv := startConnectServer(t, dial, tunnel)
	conn := dialConnectServer(t, srv.Addr())
	client, ok := conn.(*net.TCPConn)
	require.True(t, ok)

	// The payload's first part follows the CONNECT request in one write, so the server buffers it.
	br := connectTunnel(t, client, "127.0.0.1:443", "req")
	got, err := io.ReadAll(br)
	require.NoError(t, err)
	assert.Equal(t, "response", string(got))

	_, err = client.Write([]byte("uest"))
	require.NoError(t, err)
	require.NoError(t, client.CloseWrite())
	require.NoError(t, <-serverErr)
	assert.Equal(t, "request", <-serverGot)

	select {
	case <-pipeDone:
	case <-time.After(5 * time.Second):
		assert.Fail(t, "Pipe did not return after both sides closed")
	}
}

func startConnectServer(t *testing.T, dial DialFunc, tunnel TunnelFunc) *ConnectServer {
	t.Helper()
	srv, err := ListenCONNECT(dial, tunnel, log.NewNullLogger())
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Close() })
	return srv
}

func dialConnectServer(t *testing.T, addr string) net.Conn {
	t.Helper()
	var d net.Dialer
	conn, err := d.DialContext(t.Context(), "tcp", addr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
	return conn
}

// connectTunnel sends a CONNECT request for target followed by payload, requires the 200 reply,
// and returns the reader positioned after it.
func connectTunnel(t *testing.T, conn net.Conn, target, payload string) *bufio.Reader {
	t.Helper()
	_, err := conn.Write([]byte("CONNECT " + target + " HTTP/1.1\r\nHost: " + target + "\r\n\r\n" + payload))
	require.NoError(t, err)
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	require.NoError(t, err)
	// Closing the unbounded body leaves the tunnel bytes in br unread.
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return br
}

func readN(t *testing.T, r io.Reader, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	require.NoError(t, err)
	return b
}

// assertClosed asserts the peer closed the connection without sending further bytes.
func assertClosed(t *testing.T, r io.Reader) {
	t.Helper()
	b := make([]byte, 1)
	n, err := r.Read(b)
	assert.Zero(t, n)
	assert.Error(t, err)
	if netErr, ok := errors.AsType[net.Error](err); ok {
		assert.False(t, netErr.Timeout(), "connection was not closed")
	}
}

// startEchoServer listens on loopback and echoes every connection back to itself.
func startEchoServer(t *testing.T) string {
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
				_, _ = io.Copy(c, c)
			})
		}
	})
	return l.Addr().String()
}

// tcpPair returns both ends of a loopback TCP connection.
func tcpPair(t *testing.T) (*net.TCPConn, *net.TCPConn) {
	t.Helper()
	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = l.Close() }()

	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := l.Accept()
		if err != nil {
			accepted <- nil
			return
		}
		accepted <- c
	}()

	var d net.Dialer
	dialed, err := d.DialContext(t.Context(), "tcp", l.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = dialed.Close() })
	peer := <-accepted
	require.NotNil(t, peer)
	t.Cleanup(func() { _ = peer.Close() })

	dc, ok := dialed.(*net.TCPConn)
	require.True(t, ok)
	pc, ok := peer.(*net.TCPConn)
	require.True(t, ok)
	return dc, pc
}
