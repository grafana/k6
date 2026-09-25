package grpc

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/lib/netext/grpcext"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// startTestServer starts a gRPC server exposing the health service and returns
// its address.
func startTestServer(t *testing.T) string {
	t.Helper()

	var lc net.ListenConfig
	lis, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer()
	healthpb.RegisterHealthServer(srv, health.NewServer())

	go func() {
		_ = srv.Serve(lis)
	}()
	t.Cleanup(srv.Stop)

	return lis.Addr().String()
}

// testDialOptions are the dial options used to reach the test server.
func testDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(), //nolint:staticcheck
	}
}

// getOrDialTest dials addr through the pool, with a key derived from the given
// parameters.
func getOrDialTest(t *testing.T, pool *connectionPool, addr string, p *connectParams) (*grpcext.Conn, error) {
	t.Helper()

	key, err := newConnectionKey(addr, p)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return pool.getOrDial(ctx, key, addr, new(protoregistry.Types), testDialOptions()...)
}

// requireServing asserts that the connection can still reach the server.
func requireServing(t *testing.T, conn *grpcext.Conn) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := conn.HealthCheck(ctx, "")
	require.NoError(t, err)
	require.Equal(t, healthpb.HealthCheckResponse_SERVING, res.Status)
}

// entry returns the pool entry for key, if any.
func (p *connectionPool) entry(key connectionKey) (*sharedConn, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	sc, ok := p.conns[key]
	return sc, ok
}

// refCountOf returns the number of clients using the connection under key.
func (p *connectionPool) refCountOf(key connectionKey) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	sc, ok := p.conns[key]
	if !ok {
		return 0
	}
	return sc.refCount
}

// size returns the number of connections held by the pool.
func (p *connectionPool) size() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.conns)
}

func TestConnectionPool_NewIsEmpty(t *testing.T) {
	t.Parallel()

	assert.Zero(t, newConnectionPool().size())
}

func TestConnectionPool_SameParamsShareOneConnection(t *testing.T) {
	t.Parallel()

	addr := startTestServer(t)
	pool := newConnectionPool()
	params := &connectParams{IsPlaintext: true}

	first, err := getOrDialTest(t, pool, addr, params)
	require.NoError(t, err)

	second, err := getOrDialTest(t, pool, addr, params)
	require.NoError(t, err)

	key, err := newConnectionKey(addr, params)
	require.NoError(t, err)

	assert.Equal(t, 1, pool.size())
	assert.Equal(t, 2, pool.refCountOf(key))

	// Each client gets its own view of the connection, so that they keep
	// resolving protobuf types with their own registry.
	assert.NotSame(t, first, second)

	requireServing(t, first)
	requireServing(t, second)
}

func TestConnectionPool_DifferentParamsDoNotShare(t *testing.T) {
	t.Parallel()

	addr := startTestServer(t)

	testCases := []struct {
		name   string
		params *connectParams
	}{
		{
			name:   "Authority",
			params: &connectParams{IsPlaintext: true, Authority: "test.k6.io"},
		},
		{
			name:   "MaxReceiveSize",
			params: &connectParams{IsPlaintext: true, MaxReceiveSize: 1024},
		},
		{
			name:   "MaxSendSize",
			params: &connectParams{IsPlaintext: true, MaxSendSize: 1024},
		},
		{
			name:   "TLS",
			params: &connectParams{IsPlaintext: true, TLS: map[string]any{"cacerts": "-----BEGIN CERTIFICATE-----"}},
		},
	}

	base := &connectParams{IsPlaintext: true}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pool := newConnectionPool()

			_, err := getOrDialTest(t, pool, addr, base)
			require.NoError(t, err)

			_, err = getOrDialTest(t, pool, addr, tc.params)
			require.NoError(t, err)

			assert.Equal(t, 2, pool.size(),
				"clients with different connection parameters must not share a connection")
		})
	}
}

func TestConnectionPool_AddressIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	params := &connectParams{IsPlaintext: true}

	lower, err := newConnectionKey("localhost:50051", params)
	require.NoError(t, err)

	upper, err := newConnectionKey("LOCALHOST:50051", params)
	require.NoError(t, err)

	assert.Equal(t, lower, upper)

	// The same server spelled with a different casing shares one connection.
	addr := startTestServer(t)
	_, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)

	pool := newConnectionPool()

	_, err = getOrDialTest(t, pool, "localhost:"+port, params)
	require.NoError(t, err)

	_, err = getOrDialTest(t, pool, "LOCALHOST:"+port, params)
	require.NoError(t, err)

	assert.Equal(t, 1, pool.size())
}

func TestConnectionPool_ReleaseDecrementsRefCount(t *testing.T) {
	t.Parallel()

	addr := startTestServer(t)
	pool := newConnectionPool()
	params := &connectParams{IsPlaintext: true}

	first, err := getOrDialTest(t, pool, addr, params)
	require.NoError(t, err)

	_, err = getOrDialTest(t, pool, addr, params)
	require.NoError(t, err)

	key, err := newConnectionKey(addr, params)
	require.NoError(t, err)

	require.NoError(t, pool.release(key))

	assert.Equal(t, 1, pool.refCountOf(key), "the connection is still in use by another client")

	// The connection must stay open while a client is still using it.
	requireServing(t, first)
}

func TestConnectionPool_ReleaseClosesTheConnectionAtZero(t *testing.T) {
	t.Parallel()

	addr := startTestServer(t)
	pool := newConnectionPool()
	params := &connectParams{IsPlaintext: true}

	first, err := getOrDialTest(t, pool, addr, params)
	require.NoError(t, err)

	second, err := getOrDialTest(t, pool, addr, params)
	require.NoError(t, err)

	key, err := newConnectionKey(addr, params)
	require.NoError(t, err)

	require.NoError(t, pool.release(key))
	require.NoError(t, pool.release(key))

	_, ok := pool.entry(key)
	assert.False(t, ok, "the connection must be dropped from the pool once nobody uses it")
	assert.Zero(t, pool.size())

	// Both views of the connection are backed by the closed one.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = first.HealthCheck(ctx, "")
	assert.Error(t, err, "the connection must be closed once nobody uses it")

	_, err = second.HealthCheck(ctx, "")
	assert.Error(t, err, "the connection must be closed once nobody uses it")
}

func TestConnectionPool_ReleaseUnknownKey(t *testing.T) {
	t.Parallel()

	pool := newConnectionPool()

	key, err := newConnectionKey("nonexistent:50051", &connectParams{IsPlaintext: true})
	require.NoError(t, err)

	assert.NoError(t, pool.release(key))
}

func TestConnectionPool_ConcurrentDialSharesOneConnection(t *testing.T) {
	t.Parallel()

	const clients = 20

	addr := startTestServer(t)
	pool := newConnectionPool()
	params := &connectParams{IsPlaintext: true}

	conns := make([]*grpcext.Conn, clients)
	errs := make([]error, clients)

	var wg sync.WaitGroup
	wg.Add(clients)
	for i := range clients {
		go func() {
			defer wg.Done()
			conns[i], errs[i] = getOrDialTest(t, pool, addr, params)
		}()
	}
	wg.Wait()

	key, err := newConnectionKey(addr, params)
	require.NoError(t, err)

	for i := range clients {
		require.NoError(t, errs[i])
		requireServing(t, conns[i])
	}

	assert.Equal(t, 1, pool.size())
	assert.Equal(t, clients, pool.refCountOf(key))
}

func TestConnectionPool_FailedDialIsNotPooled(t *testing.T) {
	t.Parallel()

	pool := newConnectionPool()
	params := &connectParams{IsPlaintext: true}

	// Nothing is listening on this address, so the dial gets refused.
	addr := "127.0.0.1:1"
	key, err := newConnectionKey(addr, params)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := append(testDialOptions(),
		grpc.FailOnNonTempDialError(true), //nolint:staticcheck
		grpc.WithReturnConnectionError(),  //nolint:staticcheck
	)

	_, err = pool.getOrDial(ctx, key, addr, new(protoregistry.Types), opts...)
	require.Error(t, err)

	assert.Zero(t, pool.size(), "a failed connection must not be handed out to the next client")
}
