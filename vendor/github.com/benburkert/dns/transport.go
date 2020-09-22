package dns

import (
	"context"
	"crypto/tls"
	"net"
	"strings"
	"sync"
)

// Transport is an implementation of AddrDialer that manages connections to DNS
// servers. Transport may modify the sending and receiving of messages but does
// not modify messages.
type Transport struct {
	TLSConfig *tls.Config // optional TLS config, used by DialAddr

	// DialContext func creates the underlying net connection. The DialContext
	// method of a new net.Dialer is used by default.
	DialContext func(context.Context, string, string) (net.Conn, error)

	// Proxy modifies the address of the DNS server to dial.
	Proxy ProxyFunc

	// DisablePipelining disables query pipelining for stream oriented
	// connections as defined in RFC 7766, section 6.2.1.1.
	DisablePipelining bool

	plinemu sync.Mutex
	plines  map[net.Addr]*pipeline
}

// DialAddr dials a net Addr and returns a Conn.
func (t *Transport) DialAddr(ctx context.Context, addr net.Addr) (Conn, error) {
	if !t.DisablePipelining {
		if pline := t.getPipeline(addr); pline != nil && pline.alive() {
			return pline.conn(), nil
		}
	}

	conn, err := t.dialAddr(ctx, addr)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (t *Transport) dialAddr(ctx context.Context, addr net.Addr) (Conn, error) {
	conn, dnsOverTLS, err := t.dial(ctx, addr)
	if err != nil {
		return nil, err
	}
	if conn, ok := conn.(Conn); ok {
		return conn, nil
	}

	if _, ok := conn.(*tls.Conn); dnsOverTLS && !ok {
		ipaddr, _, err := net.SplitHostPort(addr.String())
		if err != nil {
			return nil, err
		}

		cfg := &tls.Config{ServerName: ipaddr}
		if t.TLSConfig != nil {
			cfg = t.TLSConfig.Clone()
		}

		conn = tls.Client(conn, cfg)
		if err := conn.(*tls.Conn).Handshake(); err != nil {
			return nil, err
		}
	}

	if _, ok := conn.(net.PacketConn); ok {
		return &PacketConn{
			Conn: conn,
		}, nil
	}

	sconn := &StreamConn{
		Conn: conn,
	}

	if !t.DisablePipelining {
		pline := t.setPipeline(addr, sconn)
		return pline.conn(), nil
	}

	return sconn, nil
}

var defaultDialer = &net.Dialer{
	Resolver: &net.Resolver{},
}

func (t *Transport) dial(ctx context.Context, addr net.Addr) (net.Conn, bool, error) {
	if t.Proxy != nil {
		var err error
		if addr, err = t.Proxy(ctx, addr); err != nil {
			return nil, false, err
		}
	}

	network, dnsOverTLS := addr.Network(), false
	if strings.HasSuffix(network, "-tls") {
		network, dnsOverTLS = network[:len(network)-4], true
	}

	dial := t.DialContext
	if dial == nil {
		dial = defaultDialer.DialContext
	}

	conn, err := dial(ctx, network, addr.String())
	if err != nil {
		return nil, false, err
	}

	return conn, dnsOverTLS, err
}

func (t *Transport) getPipeline(addr net.Addr) *pipeline {
	t.plinemu.Lock()
	defer t.plinemu.Unlock()

	if t.plines == nil {
		t.plines = make(map[net.Addr]*pipeline)
	}

	return t.plines[addr]
}

func (t *Transport) setPipeline(addr net.Addr, conn Conn) *pipeline {
	pline := &pipeline{
		Conn:     conn,
		inflight: make(map[int]pipelineTx),
	}
	go pline.run()

	t.plinemu.Lock()
	defer t.plinemu.Unlock()

	if t.plines == nil {
		t.plines = make(map[net.Addr]*pipeline)
	}

	t.plines[addr] = pline
	return pline
}
