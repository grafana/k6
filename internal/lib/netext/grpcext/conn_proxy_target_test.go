package grpcext

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/netext"
	"go.k6.io/k6/v2/lib/types"
)

// noLookupResolver fails any lookup: the target checks before CONNECT must not use DNS.
type noLookupResolver struct{ t *testing.T }

func (r noLookupResolver) LookupIP(host string) (net.IP, error) {
	r.t.Errorf("unexpected DNS lookup of %q", host)
	return nil, errors.New("unexpected DNS lookup")
}

// startCountingProxy starts a proxy on 127.0.0.1 that accepts every CONNECT. It returns the
// proxy URL, the number of connections it accepted, and the CONNECT targets it saw.
func startCountingProxy(t *testing.T) (*url.URL, *atomic.Int64, chan string) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	accepted := &atomic.Int64{}
	targets := make(chan string, 10)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			go func() {
				defer func() { _ = conn.Close() }()
				req, err := http.ReadRequest(bufio.NewReader(conn))
				if err != nil {
					return
				}
				targets <- req.Host
				_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
			}()
		}
	}()

	proxyURL, err := url.Parse("http://" + ln.Addr().String())
	require.NoError(t, err)
	return proxyURL, accepted, targets
}

func TestDialGRPCContextChecksTargetBeforeProxy(t *testing.T) {
	t.Parallel()

	proxyURL, accepted, targets := startCountingProxy(t)
	proxy := func(*url.URL) (*url.URL, error) { return proxyURL, nil }

	dialer := netext.NewDialer(net.Dialer{}, noLookupResolver{t: t})
	blocked, err := types.NewHostnameTrie([]string{"*.blocked.example"})
	require.NoError(t, err)
	dialer.BlockedHostnames = blocked
	ipNet, err := lib.ParseCIDR("10.0.0.0/8")
	require.NoError(t, err)
	dialer.Blacklist = []*lib.IPNet{ipNet}
	hosts, err := types.NewHosts(map[string]types.Host{
		"mapped.example": {IP: net.ParseIP("10.1.2.3")},
	})
	require.NoError(t, err)
	dialer.Hosts = hosts
	state := &lib.State{Dialer: dialer}

	blockedTargets := map[string]string{
		"api.blocked.example:443": "hostname (api.blocked.example) is in a blocked pattern (*.blocked.example)",
		"10.1.2.3:443":            "IP (10.1.2.3) is in a blacklisted range (10.0.0.0/8)",
		"mapped.example:443":      "IP (10.1.2.3) is in a blacklisted range (10.0.0.0/8)",
	}
	for target, expErr := range blockedTargets {
		conn, err := dialGRPCContextWithProxy(context.Background(), state, target, proxy)
		require.EqualError(t, err, expErr, target)
		require.Nil(t, conn)
	}
	assert.Zero(t, accepted.Load(), "a blocked target must not reach the proxy")

	// A name that only the proxy can resolve still goes through, without a local DNS lookup.
	const proxyOnlyTarget = "only-the-proxy-resolves-this.internal:443"
	conn, err := dialGRPCContextWithProxy(context.Background(), state, proxyOnlyTarget, proxy)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	assert.Equal(t, proxyOnlyTarget, <-targets)
	assert.Equal(t, int64(1), accepted.Load())
}

func TestHTTPConnectHandshakeErrorNamesProxyAndTarget(t *testing.T) {
	t.Parallel()

	clientConn, serverConn := net.Pipe()
	go func() {
		defer func() { _ = serverConn.Close() }()
		if _, err := http.ReadRequest(bufio.NewReader(serverConn)); err != nil {
			return
		}
		_, _ = serverConn.Write([]byte("HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\n\r\n"))
	}()

	conn, err := doHTTPConnectHandshake(
		context.Background(), clientConn, testGRPCProxyAddr, grpcProxyTargetAddr, nil,
	)
	require.Nil(t, conn)
	require.EqualError(t, err,
		"CONNECT to "+grpcProxyTargetAddr+" through grpc proxy "+testGRPCProxyAddr+" failed: 403 Forbidden")
}
