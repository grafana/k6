package dns

import (
	"context"
	"errors"
	"net"
)

var (
	// ErrConflictingID is a pipelining error due to the same message ID being
	// used for more than one inflight query.
	ErrConflictingID = errors.New("conflicting message id")

	// ErrOversizedMessage is an error returned when attempting to send a
	// message that is longer than the maximum allowed number of bytes.
	ErrOversizedMessage = errors.New("oversized message")

	// ErrTruncatedMessage indicates the response message has been truncated.
	ErrTruncatedMessage = errors.New("truncated message")

	// ErrUnsupportedNetwork is returned when DialAddr is called with an
	// unknown network.
	ErrUnsupportedNetwork = errors.New("unsupported network")

	// ErrUnsupportedOp indicates the operation is not supported by callee.
	ErrUnsupportedOp = errors.New("unsupported operation")
)

// AddrDialer dials a net Addr.
type AddrDialer interface {
	DialAddr(context.Context, net.Addr) (Conn, error)
}

// Query is a DNS request message bound for a DNS resolver.
type Query struct {
	*Message

	// RemoteAddr is the address of a DNS resolver.
	RemoteAddr net.Addr
}

// OverTLSAddr indicates the remote DNS service implements DNS-over-TLS as
// defined in RFC 7858.
type OverTLSAddr struct {
	net.Addr
}

// Network returns the address's network name with a "-tls" suffix.
func (a OverTLSAddr) Network() string {
	return a.Addr.Network() + "-tls"
}

// ProxyFunc modifies the address of a DNS server.
type ProxyFunc func(context.Context, net.Addr) (net.Addr, error)

// RoundTripper is an interface representing the ability to execute a single
// DNS transaction, obtaining a response Message for a given Query.
type RoundTripper interface {
	Do(context.Context, *Query) (*Message, error)
}
