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

// connectionKey identifies a gRPC connection by every connect() parameter that
// gRPC binds to the ClientConn at dial time: the transport credentials, the
// authority and the maximum call sizes. Clients that disagree on any of them
// cannot share a connection, since the first one to dial would silently impose
// its own settings on all the later ones.
type connectionKey struct {
	addr           string
	isPlaintext    bool
	authority      string
	maxReceiveSize int64
	maxSendSize    int64

	// tls is the canonical JSON encoding of the tls parameter, so that the key
	// stays comparable and can be used as a map key.
	tls string
}

// newConnectionKey builds the pool key for a connection made to addr with the
// given parameters.
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

// normalizeAddr lower-cases the host of a plain host:port address, since host
// names are case-insensitive and clients that spell the same server differently
// should still share a connection. Any other target is kept as it is: a scheme
// such as unix:// or passthrough:/// can carry a path or a name whose casing
// identifies a different server.
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
	// key identifies this entry in the pool. A failed or released entry can
	// be replaced by a newer one under the same key, so the entry rather than
	// the key is what a client gives back.
	key connectionKey

	// ready is closed once the client that created this entry is done dialing,
	// after which conn and err are safe to read.
	ready chan struct{}
	conn  *grpcext.Conn
	err   error

	// refCount tracks how many clients are currently using this connection.
	// It is guarded by the pool's mutex.
	refCount int
}

// connectionPool manages shared gRPC connections keyed by their connection
// parameters. It is held by RootModule and shared across all VU instances.
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

// getOrDial returns the connection shared under key, dialing addr if no client
// has established it yet. The returned connection resolves protobuf types with
// the given registry, so clients keep their own types even while sharing the
// underlying connection.
//
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

		// The connection may still be in the process of being dialed by
		// another client. Wait on the entry rather than on the pool's mutex,
		// which would also block every unrelated connect() in the meantime.
		select {
		case <-sc.ready:
		case <-ctx.Done():
			// The reference taken above can only be given back once the dial
			// is over, since the connection to close isn't known before that.
			// This client's timeout must not wait for it, so it is given back
			// in the background: ready is always closed once the dialing
			// client's own timeout expires.
			go func() {
				<-sc.ready
				_ = p.release(sc)
			}()

			return nil, nil, fmt.Errorf("waiting for the shared connection to %q: %w", addr, ctx.Err())
		}

		if sc.err != nil {
			// The client that dialed has already dropped this entry from the
			// pool, so there is no connection to give back.
			return nil, nil, sc.err
		}

		return sc.conn.WithTypes(types), sc, nil
	}

	sc := &sharedConn{key: key, ready: make(chan struct{}), refCount: 1}
	p.conns[key] = sc
	p.mu.Unlock()

	// Dialing is done outside of the pool's mutex: k6 dials with
	// grpc.WithBlock(), so holding it here would serialize every VU's
	// connect() behind this handshake.
	conn, err := grpcext.Dial(ctx, addr, types, opts...)

	p.mu.Lock()
	sc.conn, sc.err = conn, err
	if err != nil && p.conns[key] == sc {
		// A failed connection must not be handed out to the clients coming
		// after this one, they get to retry the dial themselves. It is dropped
		// before ready is closed, so that no client waiting on it can find it
		// in the pool anymore.
		delete(p.conns, key)
	}
	p.mu.Unlock()
	close(sc.ready)

	if err != nil {
		return nil, nil, err
	}

	return conn, sc, nil
}

// release gives back one reference to the given entry. The connection is closed
// and dropped from the pool once no client is using it.
//
// It must only be called once the entry's dial is over.
func (p *connectionPool) release(sc *sharedConn) error {
	p.mu.Lock()
	sc.refCount--
	if sc.refCount > 0 || p.conns[sc.key] != sc {
		// Either the connection is still in use, or the entry has already
		// been dropped from the pool because its dial failed.
		p.mu.Unlock()
		return nil
	}

	delete(p.conns, sc.key)
	p.mu.Unlock()

	return sc.conn.Close()
}
