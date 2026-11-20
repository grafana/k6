package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"

	"go.k6.io/k6/v2/internal/lib/netext/grpcext"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// connectionKey holds the connect() parameters that are fixed at dial time,
// so that only clients agreeing on all of them share a connection.
type connectionKey struct {
	addr           string
	isPlaintext    bool
	authority      string
	maxReceiveSize int64
	maxSendSize    int64
	tls            string // JSON encoded, so that the key stays comparable
}

// newConnectionKey builds the pool key for addr and the given parameters.
func newConnectionKey(addr string, p *connectParams) (connectionKey, error) {
	tls, err := json.Marshal(p.TLS)
	if err != nil {
		return connectionKey{}, fmt.Errorf("unable to identify the connection to %q for sharing: %w", addr, err)
	}

	return connectionKey{
		addr:           normalizeAddr(addr),
		isPlaintext:    p.IsPlaintext,
		authority:      p.Authority,
		maxReceiveSize: p.MaxReceiveSize,
		maxSendSize:    p.MaxSendSize,
		tls:            string(tls),
	}, nil
}

// normalizeAddr lower-cases the host of a host:port address. Targets with a
// scheme are kept as they are, since their casing can matter.
func normalizeAddr(addr string) string {
	if strings.Contains(addr, ":/") || strings.HasPrefix(addr, "unix:") {
		return addr
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}

	return net.JoinHostPort(strings.ToLower(host), port)
}

// sharedConn holds a gRPC connection that can be shared across multiple VUs.
type sharedConn struct {
	key connectionKey

	ready chan struct{} // closed once the dial is over
	conn  *grpcext.Conn
	err   error

	refCount int // guarded by connectionPool.mu
}

// connectionPool holds the gRPC connections shared across VUs.
type connectionPool struct {
	mu    sync.Mutex
	conns map[connectionKey]*sharedConn
}

// newConnectionPool creates a new connectionPool.
func newConnectionPool() *connectionPool {
	return &connectionPool{
		conns: make(map[connectionKey]*sharedConn),
	}
}

// getOrDial returns the connection shared under key, dialing addr if needed.
// Every successful call must be paired with a release of the returned entry.
func (p *connectionPool) getOrDial(
	ctx context.Context,
	key connectionKey,
	addr string,
	types *protoregistry.Types,
	opts ...grpc.DialOption,
) (*grpcext.Conn, *sharedConn, error) {
	p.mu.Lock()
	if sc, ok := p.conns[key]; ok {
		sc.refCount++
		p.mu.Unlock()

		// Another client may still be dialing it.
		select {
		case <-sc.ready:
		case <-ctx.Done():
			// The reference can only be given back once the dial is over.
			go func() {
				<-sc.ready
				_ = p.release(sc)
			}()

			return nil, nil, fmt.Errorf("waiting for the shared connection to %q: %w", addr, ctx.Err())
		}

		if sc.err != nil {
			return nil, nil, sc.err
		}

		return sc.conn.WithTypes(types), sc, nil
	}

	sc := &sharedConn{key: key, ready: make(chan struct{}), refCount: 1}
	p.conns[key] = sc
	p.mu.Unlock()

	// Dial outside of the lock, since grpc.WithBlock() waits for the handshake.
	conn, err := grpcext.Dial(ctx, addr, types, opts...)

	p.mu.Lock()
	sc.conn, sc.err = conn, err
	if err != nil && p.conns[key] == sc {
		// Let the next clients dial again instead of getting this error.
		delete(p.conns, key)
	}
	p.mu.Unlock()
	close(sc.ready)

	if err != nil {
		return nil, nil, err
	}

	return conn, sc, nil
}

// release gives back one reference to sc, and closes the connection once
// nobody uses it anymore. It must only be called once the dial is over.
func (p *connectionPool) release(sc *sharedConn) error {
	p.mu.Lock()
	sc.refCount--
	if sc.refCount > 0 || p.conns[sc.key] != sc {
		p.mu.Unlock()
		return nil
	}

	delete(p.conns, sc.key)
	p.mu.Unlock()

	return sc.conn.Close()
}
