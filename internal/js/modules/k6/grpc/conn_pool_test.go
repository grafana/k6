package grpc

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/lib/netext/grpcext"
	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
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
func getOrDialTest(
	t *testing.T, pool *connectionPool, addr string, p *connectParams,
) (*grpcext.Conn, *sharedConn, error) {
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

	first, _, err := getOrDialTest(t, pool, addr, params)
	require.NoError(t, err)

	second, _, err := getOrDialTest(t, pool, addr, params)
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

			_, _, err := getOrDialTest(t, pool, addr, base)
			require.NoError(t, err)

			_, _, err = getOrDialTest(t, pool, addr, tc.params)
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

	_, _, err = getOrDialTest(t, pool, "localhost:"+port, params)
	require.NoError(t, err)

	_, _, err = getOrDialTest(t, pool, "LOCALHOST:"+port, params)
	require.NoError(t, err)

	assert.Equal(t, 1, pool.size())
}

func TestConnectionPool_ReleaseDecrementsRefCount(t *testing.T) {
	t.Parallel()

	addr := startTestServer(t)
	pool := newConnectionPool()
	params := &connectParams{IsPlaintext: true}

	first, sc, err := getOrDialTest(t, pool, addr, params)
	require.NoError(t, err)

	_, _, err = getOrDialTest(t, pool, addr, params)
	require.NoError(t, err)

	key, err := newConnectionKey(addr, params)
	require.NoError(t, err)

	require.NoError(t, pool.release(sc))

	assert.Equal(t, 1, pool.refCountOf(key), "the connection is still in use by another client")

	// The connection must stay open while a client is still using it.
	requireServing(t, first)
}

func TestConnectionPool_ReleaseClosesTheConnectionAtZero(t *testing.T) {
	t.Parallel()

	addr := startTestServer(t)
	pool := newConnectionPool()
	params := &connectParams{IsPlaintext: true}

	first, firstEntry, err := getOrDialTest(t, pool, addr, params)
	require.NoError(t, err)

	second, secondEntry, err := getOrDialTest(t, pool, addr, params)
	require.NoError(t, err)

	key, err := newConnectionKey(addr, params)
	require.NoError(t, err)

	require.NoError(t, pool.release(firstEntry))
	require.NoError(t, pool.release(secondEntry))

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
			conns[i], _, errs[i] = getOrDialTest(t, pool, addr, params)
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

	_, _, err = pool.getOrDial(ctx, key, addr, new(protoregistry.Types), opts...)
	require.Error(t, err)

	assert.Zero(t, pool.size(), "a failed connection must not be handed out to the next client")
}

// gatedDialer holds every dial until the test opens its gate, so that a test
// can keep a dial in flight while other clients join it.
type gatedDialer struct {
	dialing chan struct{} // closed once the first dial has started
	gate    chan struct{}
	once    sync.Once
}

func newGatedDialer() *gatedDialer {
	return &gatedDialer{
		dialing: make(chan struct{}),
		gate:    make(chan struct{}),
	}
}

func (d *gatedDialer) option() grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		d.once.Do(func() { close(d.dialing) })

		select {
		case <-d.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		var nd net.Dialer
		return nd.DialContext(ctx, "tcp", addr)
	})
}

type getOrDialResult struct {
	conn *grpcext.Conn
	sc   *sharedConn
	err  error
}

// getOrDialAsync dials addr through the pool in the background.
func getOrDialAsync(
	pool *connectionPool, key connectionKey, addr string, opts ...grpc.DialOption,
) <-chan getOrDialResult {
	done := make(chan getOrDialResult, 1)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		conn, sc, err := pool.getOrDial(ctx, key, addr, new(protoregistry.Types), opts...)
		done <- getOrDialResult{conn: conn, sc: sc, err: err}
	}()

	return done
}

func TestConnectionPool_WaiterTimesOutOnInFlightDial(t *testing.T) {
	t.Parallel()

	addr := startTestServer(t)
	pool := newConnectionPool()

	key, err := newConnectionKey(addr, &connectParams{IsPlaintext: true})
	require.NoError(t, err)

	dialer := newGatedDialer()
	dialed := getOrDialAsync(pool, key, addr, append(testDialOptions(), dialer.option())...)
	<-dialer.dialing

	// The dial is held by the gate, so this client can only return through its
	// own timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, _, err = pool.getOrDial(ctx, key, addr, new(protoregistry.Types), testDialOptions()...)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// The reference taken by the client that gave up is only given back once
	// the dial is over.
	assert.Equal(t, 2, pool.refCountOf(key))

	close(dialer.gate)
	res := <-dialed
	require.NoError(t, res.err)

	require.Eventually(t, func() bool { return pool.refCountOf(key) == 1 }, 10*time.Second, 10*time.Millisecond,
		"the client that gave up must give back its reference once the dial is over")
	requireServing(t, res.conn)

	// The last user closes the connection, the client that gave up doesn't keep
	// it open.
	require.NoError(t, pool.release(res.sc))
	assert.Zero(t, pool.size())
}

func TestConnectionPool_WaiterGetsTheFailedDialError(t *testing.T) {
	t.Parallel()

	pool := newConnectionPool()

	// Nothing is listening on this address, so the dial gets refused.
	addr := "127.0.0.1:1"
	key, err := newConnectionKey(addr, &connectParams{IsPlaintext: true})
	require.NoError(t, err)

	dialer := newGatedDialer()
	dialed := getOrDialAsync(pool, key, addr, append(testDialOptions(),
		grpc.FailOnNonTempDialError(true), //nolint:staticcheck
		grpc.WithReturnConnectionError(),  //nolint:staticcheck
		dialer.option(),
	)...)
	<-dialer.dialing

	waited := getOrDialAsync(pool, key, addr, testDialOptions()...)
	require.Eventually(t, func() bool { return pool.refCountOf(key) == 2 }, 10*time.Second, 10*time.Millisecond,
		"the second client must join the dial in flight")

	close(dialer.gate)

	dialRes := <-dialed
	require.Error(t, dialRes.err)

	waitRes := <-waited
	require.Error(t, waitRes.err)
	assert.Equal(t, dialRes.err, waitRes.err, "the joining client must get the error of the dial it waited on")
	assert.Nil(t, waitRes.sc)

	assert.Zero(t, pool.size(), "a failed connection must not be handed out to the next client")
}

// newSharingTestClient returns a client, in the VU context, that shares its
// connections through pool.
func newSharingTestClient(t *testing.T, pool *connectionPool) *Client {
	t.Helper()

	testRuntime := modulestest.NewRuntime(t)
	registry := metrics.NewRegistry()

	logger := logrus.New()
	logger.Out = io.Discard

	testRuntime.MoveToVUContext(&lib.State{
		Dialer:         &net.Dialer{},
		Samples:        make(chan metrics.SampleContainer, 1000),
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
		Logger:         logger,
	})

	return &Client{vu: testRuntime.VU, types: new(protoregistry.Types), connPool: pool}
}

func connectTest(t *testing.T, c *Client, addr string, sharing bool) {
	t.Helper()

	params := c.vu.Runtime().ToValue(map[string]any{"plaintext": true, "connectionSharing": sharing})
	ok, err := c.Connect(addr, params)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestClient_ReconnectGivesBackTheSharedConnection(t *testing.T) {
	t.Parallel()

	addr := startTestServer(t)

	t.Run("ToDedicated", func(t *testing.T) {
		t.Parallel()

		pool := newConnectionPool()
		c := newSharingTestClient(t, pool)

		connectTest(t, c, addr, true)
		connectTest(t, c, addr, false)

		assert.Zero(t, pool.size(), "the shared connection must be given back when connecting again")

		dedicated := c.conn
		require.NoError(t, c.Close())

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := dedicated.HealthCheck(ctx, "")
		assert.Error(t, err, "closing the client must close its dedicated connection")
	})

	t.Run("ToShared", func(t *testing.T) {
		t.Parallel()

		pool := newConnectionPool()
		c := newSharingTestClient(t, pool)

		connectTest(t, c, addr, true)
		connectTest(t, c, addr, true)

		key, err := newConnectionKey(addr, &connectParams{IsPlaintext: true})
		require.NoError(t, err)
		assert.Equal(t, 1, pool.refCountOf(key), "the previous reference must be given back when connecting again")

		require.NoError(t, c.Close())
		assert.Zero(t, pool.size(), "the shared connection must be closed once its last user is closed")
	})
}
