package grpc

import (
	"context"
	"encoding/json"
	"fmt"
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
		// Host names are case-insensitive, so clients that spell the same
		// server differently still share a connection.
		addr:           strings.ToLower(addr),
		isPlaintext:    p.IsPlaintext,
		authority:      p.Authority,
		maxReceiveSize: p.MaxReceiveSize,
		maxSendSize:    p.MaxSendSize,
		tls:            string(tls),
	}, nil
}

// sharedConn holds a gRPC connection that can be shared across multiple VUs.
type sharedConn struct {
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
// Every successful call must be paired with a release of the same key.
func (p *connectionPool) getOrDial(
	ctx context.Context,
	key connectionKey,
	addr string,
	types *protoregistry.Types,
	opts ...grpc.DialOption,
) (*grpcext.Conn, error) {
	p.mu.Lock()
	if sc, ok := p.conns[key]; ok {
		sc.refCount++
		p.mu.Unlock()

		// The connection may still be in the process of being dialed by
		// another client. Wait on the entry rather than on the pool's mutex,
		// which would also block every unrelated connect() in the meantime.
		<-sc.ready
		if sc.err != nil {
			// The client that dialed has already dropped this entry from the
			// pool, so give back the reference taken above instead of calling
			// release, which by now may refer to a newer entry for this key.
			p.mu.Lock()
			sc.refCount--
			p.mu.Unlock()

			return nil, sc.err
		}

		return sc.conn.WithTypes(types), nil
	}

	sc := &sharedConn{ready: make(chan struct{}), refCount: 1}
	p.conns[key] = sc
	p.mu.Unlock()

	// Dialing is done outside of the pool's mutex: k6 dials with
	// grpc.WithBlock(), so holding it here would serialize every VU's
	// connect() behind this handshake.
	sc.conn, sc.err = grpcext.Dial(ctx, addr, types, opts...)
	close(sc.ready)

	if sc.err != nil {
		// A failed connection must not be handed out to the clients coming
		// after this one, they get to retry the dial themselves.
		p.mu.Lock()
		if p.conns[key] == sc {
			delete(p.conns, key)
		}
		p.mu.Unlock()

		return nil, sc.err
	}

	return sc.conn, nil
}

// release gives back one reference to the connection shared under key. The
// connection is closed and dropped from the pool once no client is using it.
func (p *connectionPool) release(key connectionKey) error {
	p.mu.Lock()
	sc, ok := p.conns[key]
	if !ok {
		p.mu.Unlock()
		return nil
	}

	sc.refCount--
	if sc.refCount > 0 {
		p.mu.Unlock()
		return nil
	}

	delete(p.conns, key)
	p.mu.Unlock()

	if sc.conn == nil {
		return nil
	}

	return sc.conn.Close()
}
