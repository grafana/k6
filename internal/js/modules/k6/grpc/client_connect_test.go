package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/reflect/protoregistry"

	"go.k6.io/k6/v2/internal/lib/netext/grpcext"
	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

// startHealthServer starts a gRPC server that only exposes the health service,
// and returns its address.
func startHealthServer(t *testing.T) string {
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

// closedAddr returns an address nothing is listening on.
func closedAddr(t *testing.T) string {
	t.Helper()

	var lc net.ListenConfig
	lis, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr := lis.Addr().String()
	require.NoError(t, lis.Close())

	return addr
}

// newConnectTestClient returns a client in the VU context.
func newConnectTestClient(t *testing.T) *Client {
	t.Helper()

	testRuntime := modulestest.NewRuntime(t)
	registry := metrics.NewRegistry()

	testRuntime.MoveToVUContext(&lib.State{
		Dialer:         &net.Dialer{},
		Samples:        make(chan metrics.SampleContainer, 1000),
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
		Logger:         nullLogger(),
	})

	return &Client{vu: testRuntime.VU, types: new(protoregistry.Types)}
}

// connectWith calls connect() with the given parameters.
func connectWith(c *Client, addr string, params map[string]any) (bool, error) {
	return c.Connect(addr, c.vu.Runtime().ToValue(params))
}

// healthCheck runs a health check over conn.
func healthCheck(t *testing.T, conn *grpcext.Conn) error {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := conn.HealthCheck(ctx, "")
	return err
}

func TestClientConnectAgain(t *testing.T) {
	t.Parallel()

	addr := startHealthServer(t)
	plaintext := map[string]any{"plaintext": true}

	t.Run("ClosesThePreviousConnection", func(t *testing.T) {
		t.Parallel()

		c := newConnectTestClient(t)

		_, err := connectWith(c, addr, plaintext)
		require.NoError(t, err)
		previous := c.conn

		_, err = connectWith(c, addr, plaintext)
		require.NoError(t, err)

		assert.Error(t, healthCheck(t, previous), "the previous connection must be closed")
		assert.NoError(t, healthCheck(t, c.conn))

		require.NoError(t, c.Close())
	})

	t.Run("FailedDialClosesThePreviousConnection", func(t *testing.T) {
		t.Parallel()

		c := newConnectTestClient(t)

		_, err := connectWith(c, addr, plaintext)
		require.NoError(t, err)
		previous := c.conn

		_, err = connectWith(c, closedAddr(t), map[string]any{"plaintext": true, "timeout": "2s"})
		require.Error(t, err)

		assert.Error(t, healthCheck(t, previous), "the previous connection must be closed")
		assert.Nil(t, c.conn)
		assert.NoError(t, c.Close())
	})

	t.Run("InvalidParamsKeepThePreviousConnection", func(t *testing.T) {
		t.Parallel()

		c := newConnectTestClient(t)

		_, err := connectWith(c, addr, plaintext)
		require.NoError(t, err)
		previous := c.conn

		_, err = connectWith(c, addr, map[string]any{"void": true})
		require.ErrorContains(t, err, "invalid grpc.connect() parameters")

		assert.Same(t, previous, c.conn)
		assert.NoError(t, healthCheck(t, c.conn), "the connection must still be usable")

		require.NoError(t, c.Close())
	})

	t.Run("FailedReflectionIsClosedOnTheNextConnect", func(t *testing.T) {
		t.Parallel()

		c := newConnectTestClient(t)

		// The server doesn't implement reflection, so connect() fails after
		// having dialed.
		_, err := connectWith(c, addr, map[string]any{"plaintext": true, "reflect": true})
		require.Error(t, err)

		require.NotNil(t, c.conn, "the connection dialed before the reflection failed is kept")
		previous := c.conn
		require.NoError(t, healthCheck(t, previous))

		_, err = connectWith(c, addr, plaintext)
		require.NoError(t, err)

		assert.Error(t, healthCheck(t, previous), "the previous connection must be closed")
		assert.NoError(t, healthCheck(t, c.conn))

		require.NoError(t, c.Close())
	})
}
