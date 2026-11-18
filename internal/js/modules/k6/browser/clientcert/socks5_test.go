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
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

const (
	socksATYPIPv4   = 0x01
	socksATYPDomain = 0x03
	socksATYPIPv6   = 0x04
)

func TestSOCKS5Server(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		listenHost string
		atyp       byte
		host       string
		addr       []byte
	}{
		{
			name:       "domain",
			listenHost: "127.0.0.1",
			atyp:       socksATYPDomain,
			host:       "localhost",
			addr:       append([]byte{byte(len("localhost"))}, "localhost"...),
		},
		{
			name:       "ipv4",
			listenHost: "127.0.0.1",
			atyp:       socksATYPIPv4,
			host:       "127.0.0.1",
			addr:       net.IPv4(127, 0, 0, 1).To4(),
		},
		{
			name:       "ipv6",
			listenHost: "::1",
			atyp:       socksATYPIPv6,
			host:       "::1",
			addr:       net.IPv6loopback,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			echoAddr := startEchoServer(t, tt.listenHost)
			_, portStr, err := net.SplitHostPort(echoAddr)
			require.NoError(t, err)
			port, err := strconv.Atoi(portStr)
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

			srv := startSOCKS5Server(t, dial, tunnel)
			conn := dialSOCKS5(t, srv.Addr())

			assert.Equal(t, []byte{0x05, 0x00}, socksNegotiate(t, conn, 0x00))
			socksRequest(t, conn, 0x01, tt.atyp, tt.addr, uint16(port))
			reply := readN(t, conn, 10)

			call := <-dials
			assert.Equal(t, "tcp", call.network)
			assert.Equal(t, net.JoinHostPort(tt.host, portStr), call.address)
			assert.Equal(t, byte(0x00), reply[1])

			_, err = conn.Write([]byte("ping"))
			require.NoError(t, err)
			assert.Equal(t, []byte("ping"), readN(t, conn, 4))

			tc := <-tunnels
			assert.Equal(t, tt.host, tc.host)
			assert.Equal(t, port, tc.port)
		})
	}
}

func TestSOCKS5Server_Rejects(t *testing.T) {
	t.Parallel()

	connect := func(cmd byte) []byte {
		return append([]byte{0x05, 0x01, 0x00, 0x05, cmd, 0x00, socksATYPIPv4, 127, 0, 0, 1}, 0x00, 0x50)
	}
	reply := func(rep byte) []byte {
		return []byte{0x05, 0x00, 0x05, rep, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
	}

	tests := []struct {
		name    string
		send    []byte
		dialErr error
		want    []byte
	}{
		{
			name: "version_4",
			send: []byte{0x04, 0x01, 0x00, 0x50, 127, 0, 0, 1, 0x00},
		},
		{
			name: "no_acceptable_method",
			send: []byte{0x05, 0x01, 0x02},
			want: []byte{0x05, 0xFF},
		},
		{
			name: "bind_command",
			send: connect(0x02),
			want: reply(0x07),
		},
		{
			name: "udp_command",
			send: connect(0x03),
			want: reply(0x07),
		},
		{
			name: "unknown_address_type",
			send: []byte{0x05, 0x01, 0x00, 0x05, 0x01, 0x00, 0x05},
			want: reply(0x08),
		},
		{
			name:    "dial_refused",
			send:    connect(0x01),
			dialErr: fmt.Errorf("dialing: %w", syscall.ECONNREFUSED),
			want:    reply(0x05),
		},
		{
			name:    "dial_host_not_found",
			send:    connect(0x01),
			dialErr: &net.DNSError{Err: "no such host", Name: "example.invalid", IsNotFound: true},
			want:    reply(0x04),
		},
		{
			name:    "dial_other_error",
			send:    connect(0x01),
			dialErr: errors.New("dial failed"),
			want:    reply(0x01),
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

			srv := startSOCKS5Server(t, dial, tunnel)
			conn := dialSOCKS5(t, srv.Addr())

			_, err := conn.Write(tt.send)
			require.NoError(t, err)

			if len(tt.want) > 0 {
				assert.Equal(t, tt.want, readN(t, conn, len(tt.want)))
			}
			assertClosed(t, conn)
			assert.False(t, tunneled.Load())
		})
	}
}

func TestSOCKS5Server_Close(t *testing.T) {
	t.Parallel()

	echoAddr := startEchoServer(t, "127.0.0.1")
	dial := func(ctx context.Context, network, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, echoAddr)
	}
	tunnelDone := make(chan struct{})
	tunnel := func(ctx context.Context, browser, destination net.Conn, host string, port int) {
		defer close(tunnelDone)
		Pipe(ctx, browser, destination, host, port)
	}

	srv, err := ListenSOCKS5(dial, tunnel, log.NewNullLogger())
	require.NoError(t, err)
	conn := dialSOCKS5(t, srv.Addr())

	require.Equal(t, []byte{0x05, 0x00}, socksNegotiate(t, conn, 0x00))
	socksRequest(t, conn, 0x01, socksATYPIPv4, net.IPv4(127, 0, 0, 1).To4(), 80)
	require.Equal(t, byte(0x00), readN(t, conn, 10)[1])
	_, err = conn.Write([]byte("ping"))
	require.NoError(t, err)
	require.Equal(t, []byte("ping"), readN(t, conn, 4))

	require.NoError(t, srv.Close())

	select {
	case <-tunnelDone:
	default:
		assert.Fail(t, "Close returned before the tunnel goroutine exited")
	}
	assertClosed(t, conn)
	var d net.Dialer
	if c, err := d.DialContext(t.Context(), "tcp", srv.Addr()); err == nil {
		_ = c.Close()
		assert.Fail(t, "dial to a closed server succeeded")
	}
	assert.NoError(t, srv.Close())
}

func TestPipe_HalfClose(t *testing.T) {
	t.Parallel()

	client, browser := tcpPair(t)
	destination, server := tcpPair(t)

	serverErr := make(chan error, 1)
	go func() {
		defer func() { _ = server.Close() }()
		req, err := io.ReadAll(server)
		if err != nil {
			serverErr <- err
			return
		}
		_, err = server.Write(append([]byte("response:"), req...))
		serverErr <- err
	}()

	pipeDone := make(chan struct{})
	go func() {
		defer close(pipeDone)
		Pipe(t.Context(), browser, destination, "127.0.0.1", 0)
	}()

	_, err := client.Write([]byte("request"))
	require.NoError(t, err)
	require.NoError(t, client.CloseWrite())
	require.NoError(t, client.SetReadDeadline(time.Now().Add(5*time.Second)))

	got, err := io.ReadAll(client)
	require.NoError(t, err)
	assert.Equal(t, "response:request", string(got))
	require.NoError(t, <-serverErr)

	select {
	case <-pipeDone:
	case <-time.After(5 * time.Second):
		assert.Fail(t, "Pipe did not return after both sides closed")
	}
}

func startSOCKS5Server(t *testing.T, dial DialFunc, tunnel TunnelFunc) *SOCKS5Server {
	t.Helper()
	srv, err := ListenSOCKS5(dial, tunnel, log.NewNullLogger())
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Close() })
	return srv
}

func dialSOCKS5(t *testing.T, addr string) net.Conn {
	t.Helper()
	var d net.Dialer
	conn, err := d.DialContext(t.Context(), "tcp", addr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
	return conn
}

// socksNegotiate offers methods and returns the server's method selection reply.
func socksNegotiate(t *testing.T, conn net.Conn, methods ...byte) []byte {
	t.Helper()
	_, err := conn.Write(append([]byte{0x05, byte(len(methods))}, methods...))
	require.NoError(t, err)
	return readN(t, conn, 2)
}

func socksRequest(t *testing.T, conn net.Conn, cmd, atyp byte, addr []byte, port uint16) {
	t.Helper()
	req := append([]byte{0x05, cmd, 0x00, atyp}, addr...)
	req = binary.BigEndian.AppendUint16(req, port)
	_, err := conn.Write(req)
	require.NoError(t, err)
}

func readN(t *testing.T, conn net.Conn, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	_, err := io.ReadFull(conn, b)
	require.NoError(t, err)
	return b
}

// assertClosed asserts the peer closed conn without sending further bytes.
func assertClosed(t *testing.T, conn net.Conn) {
	t.Helper()
	b := make([]byte, 1)
	n, err := conn.Read(b)
	assert.Zero(t, n)
	assert.Error(t, err)
	if netErr, ok := errors.AsType[net.Error](err); ok {
		assert.False(t, netErr.Timeout(), "connection was not closed")
	}
}

// startEchoServer listens on host and echoes every connection back to itself.
func startEchoServer(t *testing.T, host string) string {
	t.Helper()
	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", net.JoinHostPort(host, "0"))
	if err != nil && host == "::1" {
		t.Skipf("IPv6 loopback unavailable: %v", err)
	}
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
