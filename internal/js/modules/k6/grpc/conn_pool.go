package grpc

import (
	"context"
	"strings"
	"sync"

	"go.k6.io/k6/internal/lib/netext/grpcext"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// sharedConn holds a gRPC connection that can be shared across multiple VUs.
// refCount tracks how many clients are currently using this connection.
type sharedConn struct {
	conn     *grpcext.Conn
	refCount int
}

// connectionPool manages shared gRPC connections keyed by server address.
// It is held by RootModule and shared across all VU instances.
type connectionPool struct {
	mu    sync.Mutex
	conns map[string]*sharedConn
}

// newConnectionPool creates a new connectionPool.
func newConnectionPool() *connectionPool {
	return &connectionPool{
		conns: make(map[string]*sharedConn),
	}
}

// getOrDial returns an existing shared connection for the given address if one
// exists. Otherwise, it dials a new connection and stores it in the pool.
func (p *connectionPool) getOrDial(
	ctx context.Context,
	addr string,
	types *protoregistry.Types,
	opts ...grpc.DialOption,
) (*grpcext.Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := strings.ToLower(addr)
	if sc, ok := p.conns[key]; ok {
		sc.refCount++
		return sc.conn, nil
	}

	conn, err := grpcext.Dial(ctx, addr, types, opts...)
	if err != nil {
		return nil, err
	}

	p.conns[key] = &sharedConn{
		conn:     conn,
		refCount: 1,
	}
	return conn, nil
}

// release decrements the refCount for the connection at addr.
// If no clients remain, the connection is closed and removed from the pool.
func (p *connectionPool) release(addr string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := strings.ToLower(addr)
	sc, ok := p.conns[key]
	if !ok {
		return nil
	}

	sc.refCount--
	if sc.refCount <= 0 {
		delete(p.conns, key)
		return sc.conn.Close()
	}
	return nil
}
