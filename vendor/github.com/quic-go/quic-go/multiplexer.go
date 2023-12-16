package quic

import (
	"fmt"
	"net"
	"sync"

	"github.com/quic-go/quic-go/internal/utils"
)

var (
	connMuxerOnce sync.Once
	connMuxer     multiplexer
)

type indexableConn interface{ LocalAddr() net.Addr }

type multiplexer interface {
	AddConn(conn indexableConn)
	RemoveConn(indexableConn) error
}

// The connMultiplexer listens on multiple net.PacketConns and dispatches
// incoming packets to the connection handler.
type connMultiplexer struct {
	mutex sync.Mutex

	conns  map[string] /* LocalAddr().String() */ indexableConn
	logger utils.Logger
}

var _ multiplexer = &connMultiplexer{}

func getMultiplexer() multiplexer {
	connMuxerOnce.Do(func() {
		connMuxer = &connMultiplexer{
			conns:  make(map[string]indexableConn),
			logger: utils.DefaultLogger.WithPrefix("muxer"),
		}
	})
	return connMuxer
}

func (m *connMultiplexer) index(addr net.Addr) string {
	return addr.Network() + " " + addr.String()
}

func (m *connMultiplexer) AddConn(c indexableConn) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	connIndex := m.index(c.LocalAddr())
	p, ok := m.conns[connIndex]
	if ok {
		// Panics if we're already listening on this connection.
		// This is a safeguard because we're introducing a breaking API change, see
		// https://github.com/quic-go/quic-go/issues/3727 for details.
		// We'll remove this at a later time, when most users of the library have made the switch.
		panic("connection already exists") // TODO: write a nice message
	}
	m.conns[connIndex] = p
}

func (m *connMultiplexer) RemoveConn(c indexableConn) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	connIndex := m.index(c.LocalAddr())
	if _, ok := m.conns[connIndex]; !ok {
		return fmt.Errorf("cannote remove connection, connection is unknown")
	}

	delete(m.conns, connIndex)
	return nil
}
