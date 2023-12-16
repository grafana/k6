package quic

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/internal/wire"
	"github.com/quic-go/quic-go/logging"
)

var errListenerAlreadySet = errors.New("listener already set")

// The Transport is the central point to manage incoming and outgoing QUIC connections.
// QUIC demultiplexes connections based on their QUIC Connection IDs, not based on the 4-tuple.
// This means that a single UDP socket can be used for listening for incoming connections, as well as
// for dialing an arbitrary number of outgoing connections.
// A Transport handles a single net.PacketConn, and offers a range of configuration options
// compared to the simple helper functions like Listen and Dial that this package provides.
type Transport struct {
	// A single net.PacketConn can only be handled by one Transport.
	// Bad things will happen if passed to multiple Transports.
	//
	// A number of optimizations will be enabled if the connections implements the OOBCapablePacketConn interface,
	// as a *net.UDPConn does.
	// 1. It enables the Don't Fragment (DF) bit on the IP header.
	//    This is required to run DPLPMTUD (Path MTU Discovery, RFC 8899).
	// 2. It enables reading of the ECN bits from the IP header.
	//    This allows the remote node to speed up its loss detection and recovery.
	// 3. It uses batched syscalls (recvmmsg) to more efficiently receive packets from the socket.
	// 4. It uses Generic Segmentation Offload (GSO) to efficiently send batches of packets (on Linux).
	//
	// After passing the connection to the Transport, it's invalid to call ReadFrom or WriteTo on the connection.
	Conn net.PacketConn

	// The length of the connection ID in bytes.
	// It can be 0, or any value between 4 and 18.
	// If unset, a 4 byte connection ID will be used.
	ConnectionIDLength int

	// Use for generating new connection IDs.
	// This allows the application to control of the connection IDs used,
	// which allows routing / load balancing based on connection IDs.
	// All Connection IDs returned by the ConnectionIDGenerator MUST
	// have the same length.
	ConnectionIDGenerator ConnectionIDGenerator

	// The StatelessResetKey is used to generate stateless reset tokens.
	// If no key is configured, sending of stateless resets is disabled.
	// It is highly recommended to configure a stateless reset key, as stateless resets
	// allow the peer to quickly recover from crashes and reboots of this node.
	// See section 10.3 of RFC 9000 for details.
	StatelessResetKey *StatelessResetKey

	// The TokenGeneratorKey is used to encrypt session resumption tokens.
	// If no key is configured, a random key will be generated.
	// If multiple servers are authoritative for the same domain, they should use the same key,
	// see section 8.1.3 of RFC 9000 for details.
	TokenGeneratorKey *TokenGeneratorKey

	// MaxTokenAge is the maximum age of the resumption token presented during the handshake.
	// These tokens allow skipping address resumption when resuming a QUIC connection,
	// and are especially useful when using 0-RTT.
	// If not set, it defaults to 24 hours.
	// See section 8.1.3 of RFC 9000 for details.
	MaxTokenAge time.Duration

	// DisableVersionNegotiationPackets disables the sending of Version Negotiation packets.
	// This can be useful if version information is exchanged out-of-band.
	// It has no effect for clients.
	DisableVersionNegotiationPackets bool

	// A Tracer traces events that don't belong to a single QUIC connection.
	Tracer *logging.Tracer

	handlerMap packetHandlerManager

	mutex    sync.Mutex
	initOnce sync.Once
	initErr  error

	// Set in init.
	// If no ConnectionIDGenerator is set, this is the ConnectionIDLength.
	connIDLen int
	// Set in init.
	// If no ConnectionIDGenerator is set, this is set to a default.
	connIDGenerator ConnectionIDGenerator

	server *baseServer

	conn rawConn

	closeQueue          chan closePacket
	statelessResetQueue chan receivedPacket

	listening   chan struct{} // is closed when listen returns
	closed      bool
	createdConn bool
	isSingleUse bool // was created for a single server or client, i.e. by calling quic.Listen or quic.Dial

	readingNonQUICPackets atomic.Bool
	nonQUICPackets        chan receivedPacket

	logger utils.Logger
}

// Listen starts listening for incoming QUIC connections.
// There can only be a single listener on any net.PacketConn.
// Listen may only be called again after the current Listener was closed.
func (t *Transport) Listen(tlsConf *tls.Config, conf *Config) (*Listener, error) {
	s, err := t.createServer(tlsConf, conf, false)
	if err != nil {
		return nil, err
	}
	return &Listener{baseServer: s}, nil
}

// ListenEarly starts listening for incoming QUIC connections.
// There can only be a single listener on any net.PacketConn.
// Listen may only be called again after the current Listener was closed.
func (t *Transport) ListenEarly(tlsConf *tls.Config, conf *Config) (*EarlyListener, error) {
	s, err := t.createServer(tlsConf, conf, true)
	if err != nil {
		return nil, err
	}
	return &EarlyListener{baseServer: s}, nil
}

func (t *Transport) createServer(tlsConf *tls.Config, conf *Config, allow0RTT bool) (*baseServer, error) {
	if tlsConf == nil {
		return nil, errors.New("quic: tls.Config not set")
	}
	if err := validateConfig(conf); err != nil {
		return nil, err
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.server != nil {
		return nil, errListenerAlreadySet
	}
	conf = populateServerConfig(conf)
	if err := t.init(false); err != nil {
		return nil, err
	}
	s := newServer(
		t.conn,
		t.handlerMap,
		t.connIDGenerator,
		tlsConf,
		conf,
		t.Tracer,
		t.closeServer,
		*t.TokenGeneratorKey,
		t.MaxTokenAge,
		t.DisableVersionNegotiationPackets,
		allow0RTT,
	)
	t.server = s
	return s, nil
}

// Dial dials a new connection to a remote host (not using 0-RTT).
func (t *Transport) Dial(ctx context.Context, addr net.Addr, tlsConf *tls.Config, conf *Config) (Connection, error) {
	return t.dial(ctx, addr, "", tlsConf, conf, false)
}

// DialEarly dials a new connection, attempting to use 0-RTT if possible.
func (t *Transport) DialEarly(ctx context.Context, addr net.Addr, tlsConf *tls.Config, conf *Config) (EarlyConnection, error) {
	return t.dial(ctx, addr, "", tlsConf, conf, true)
}

func (t *Transport) dial(ctx context.Context, addr net.Addr, host string, tlsConf *tls.Config, conf *Config, use0RTT bool) (EarlyConnection, error) {
	if err := validateConfig(conf); err != nil {
		return nil, err
	}
	conf = populateConfig(conf)
	if err := t.init(t.isSingleUse); err != nil {
		return nil, err
	}
	var onClose func()
	if t.isSingleUse {
		onClose = func() { t.Close() }
	}
	tlsConf = tlsConf.Clone()
	tlsConf.MinVersion = tls.VersionTLS13
	setTLSConfigServerName(tlsConf, addr, host)
	return dial(ctx, newSendConn(t.conn, addr, packetInfo{}, utils.DefaultLogger), t.connIDGenerator, t.handlerMap, tlsConf, conf, onClose, use0RTT)
}

func (t *Transport) init(allowZeroLengthConnIDs bool) error {
	t.initOnce.Do(func() {
		var conn rawConn
		if c, ok := t.Conn.(rawConn); ok {
			conn = c
		} else {
			var err error
			conn, err = wrapConn(t.Conn)
			if err != nil {
				t.initErr = err
				return
			}
		}

		t.logger = utils.DefaultLogger // TODO: make this configurable
		t.conn = conn
		t.handlerMap = newPacketHandlerMap(t.StatelessResetKey, t.enqueueClosePacket, t.logger)
		t.listening = make(chan struct{})

		t.closeQueue = make(chan closePacket, 4)
		t.statelessResetQueue = make(chan receivedPacket, 4)
		if t.TokenGeneratorKey == nil {
			var key TokenGeneratorKey
			if _, err := rand.Read(key[:]); err != nil {
				t.initErr = err
				return
			}
			t.TokenGeneratorKey = &key
		}

		if t.ConnectionIDGenerator != nil {
			t.connIDGenerator = t.ConnectionIDGenerator
			t.connIDLen = t.ConnectionIDGenerator.ConnectionIDLen()
		} else {
			connIDLen := t.ConnectionIDLength
			if t.ConnectionIDLength == 0 && !allowZeroLengthConnIDs {
				connIDLen = protocol.DefaultConnectionIDLength
			}
			t.connIDLen = connIDLen
			t.connIDGenerator = &protocol.DefaultConnectionIDGenerator{ConnLen: t.connIDLen}
		}

		getMultiplexer().AddConn(t.Conn)
		go t.listen(conn)
		go t.runSendQueue()
	})
	return t.initErr
}

// WriteTo sends a packet on the underlying connection.
func (t *Transport) WriteTo(b []byte, addr net.Addr) (int, error) {
	if err := t.init(false); err != nil {
		return 0, err
	}
	return t.conn.WritePacket(b, addr, nil, 0, protocol.ECNUnsupported)
}

func (t *Transport) enqueueClosePacket(p closePacket) {
	select {
	case t.closeQueue <- p:
	default:
		// Oops, we're backlogged.
		// Just drop the packet, sending CONNECTION_CLOSE copies is best effort anyway.
	}
}

func (t *Transport) runSendQueue() {
	for {
		select {
		case <-t.listening:
			return
		case p := <-t.closeQueue:
			t.conn.WritePacket(p.payload, p.addr, p.info.OOB(), 0, protocol.ECNUnsupported)
		case p := <-t.statelessResetQueue:
			t.sendStatelessReset(p)
		}
	}
}

// Close closes the underlying connection.
// If any listener was started, it will be closed as well.
// It is invalid to start new listeners or connections after that.
func (t *Transport) Close() error {
	t.close(errors.New("closing"))
	if t.createdConn {
		if err := t.Conn.Close(); err != nil {
			return err
		}
	} else if t.conn != nil {
		t.conn.SetReadDeadline(time.Now())
		defer func() { t.conn.SetReadDeadline(time.Time{}) }()
	}
	if t.listening != nil {
		<-t.listening // wait until listening returns
	}
	return nil
}

func (t *Transport) closeServer() {
	t.mutex.Lock()
	t.server = nil
	if t.isSingleUse {
		t.closed = true
	}
	t.mutex.Unlock()
	if t.createdConn {
		t.Conn.Close()
	}
	if t.isSingleUse {
		t.conn.SetReadDeadline(time.Now())
		defer func() { t.conn.SetReadDeadline(time.Time{}) }()
		<-t.listening // wait until listening returns
	}
}

func (t *Transport) close(e error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.closed {
		return
	}

	if t.handlerMap != nil {
		t.handlerMap.Close(e)
	}
	if t.server != nil {
		t.server.close(e, false)
	}
	t.closed = true
}

// only print warnings about the UDP receive buffer size once
var setBufferWarningOnce sync.Once

func (t *Transport) listen(conn rawConn) {
	defer close(t.listening)
	defer getMultiplexer().RemoveConn(t.Conn)

	for {
		p, err := conn.ReadPacket()
		//nolint:staticcheck // SA1019 ignore this!
		// TODO: This code is used to ignore wsa errors on Windows.
		// Since net.Error.Temporary is deprecated as of Go 1.18, we should find a better solution.
		// See https://github.com/quic-go/quic-go/issues/1737 for details.
		if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
			t.mutex.Lock()
			closed := t.closed
			t.mutex.Unlock()
			if closed {
				return
			}
			t.logger.Debugf("Temporary error reading from conn: %w", err)
			continue
		}
		if err != nil {
			// Windows returns an error when receiving a UDP datagram that doesn't fit into the provided buffer.
			if isRecvMsgSizeErr(err) {
				continue
			}
			t.close(err)
			return
		}
		t.handlePacket(p)
	}
}

func (t *Transport) handlePacket(p receivedPacket) {
	if len(p.data) == 0 {
		return
	}
	if !wire.IsPotentialQUICPacket(p.data[0]) && !wire.IsLongHeaderPacket(p.data[0]) {
		t.handleNonQUICPacket(p)
		return
	}
	connID, err := wire.ParseConnectionID(p.data, t.connIDLen)
	if err != nil {
		t.logger.Debugf("error parsing connection ID on packet from %s: %s", p.remoteAddr, err)
		if t.Tracer != nil && t.Tracer.DroppedPacket != nil {
			t.Tracer.DroppedPacket(p.remoteAddr, logging.PacketTypeNotDetermined, p.Size(), logging.PacketDropHeaderParseError)
		}
		p.buffer.MaybeRelease()
		return
	}

	if isStatelessReset := t.maybeHandleStatelessReset(p.data); isStatelessReset {
		return
	}
	if handler, ok := t.handlerMap.Get(connID); ok {
		handler.handlePacket(p)
		return
	}
	if !wire.IsLongHeaderPacket(p.data[0]) {
		t.maybeSendStatelessReset(p)
		return
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.server == nil { // no server set
		t.logger.Debugf("received a packet with an unexpected connection ID %s", connID)
		return
	}
	t.server.handlePacket(p)
}

func (t *Transport) maybeSendStatelessReset(p receivedPacket) {
	if t.StatelessResetKey == nil {
		p.buffer.Release()
		return
	}

	// Don't send a stateless reset in response to very small packets.
	// This includes packets that could be stateless resets.
	if len(p.data) <= protocol.MinStatelessResetSize {
		p.buffer.Release()
		return
	}

	select {
	case t.statelessResetQueue <- p:
	default:
		// it's fine to not send a stateless reset when we're busy
		p.buffer.Release()
	}
}

func (t *Transport) sendStatelessReset(p receivedPacket) {
	defer p.buffer.Release()

	connID, err := wire.ParseConnectionID(p.data, t.connIDLen)
	if err != nil {
		t.logger.Errorf("error parsing connection ID on packet from %s: %s", p.remoteAddr, err)
		return
	}
	token := t.handlerMap.GetStatelessResetToken(connID)
	t.logger.Debugf("Sending stateless reset to %s (connection ID: %s). Token: %#x", p.remoteAddr, connID, token)
	data := make([]byte, protocol.MinStatelessResetSize-16, protocol.MinStatelessResetSize)
	rand.Read(data)
	data[0] = (data[0] & 0x7f) | 0x40
	data = append(data, token[:]...)
	if _, err := t.conn.WritePacket(data, p.remoteAddr, p.info.OOB(), 0, protocol.ECNUnsupported); err != nil {
		t.logger.Debugf("Error sending Stateless Reset to %s: %s", p.remoteAddr, err)
	}
}

func (t *Transport) maybeHandleStatelessReset(data []byte) bool {
	// stateless resets are always short header packets
	if wire.IsLongHeaderPacket(data[0]) {
		return false
	}
	if len(data) < 17 /* type byte + 16 bytes for the reset token */ {
		return false
	}

	token := *(*protocol.StatelessResetToken)(data[len(data)-16:])
	if conn, ok := t.handlerMap.GetByResetToken(token); ok {
		t.logger.Debugf("Received a stateless reset with token %#x. Closing connection.", token)
		go conn.destroy(&StatelessResetError{Token: token})
		return true
	}
	return false
}

func (t *Transport) handleNonQUICPacket(p receivedPacket) {
	// Strictly speaking, this is racy,
	// but we only care about receiving packets at some point after ReadNonQUICPacket has been called.
	if !t.readingNonQUICPackets.Load() {
		return
	}
	select {
	case t.nonQUICPackets <- p:
	default:
		if t.Tracer != nil && t.Tracer.DroppedPacket != nil {
			t.Tracer.DroppedPacket(p.remoteAddr, logging.PacketTypeNotDetermined, p.Size(), logging.PacketDropDOSPrevention)
		}
	}
}

const maxQueuedNonQUICPackets = 32

// ReadNonQUICPacket reads non-QUIC packets received on the underlying connection.
// The detection logic is very simple: Any packet that has the first and second bit of the packet set to 0.
// Note that this is stricter than the detection logic defined in RFC 9443.
func (t *Transport) ReadNonQUICPacket(ctx context.Context, b []byte) (int, net.Addr, error) {
	if err := t.init(false); err != nil {
		return 0, nil, err
	}
	if !t.readingNonQUICPackets.Load() {
		t.nonQUICPackets = make(chan receivedPacket, maxQueuedNonQUICPackets)
		t.readingNonQUICPackets.Store(true)
	}
	select {
	case <-ctx.Done():
		return 0, nil, ctx.Err()
	case p := <-t.nonQUICPackets:
		n := copy(b, p.data)
		return n, p.remoteAddr, nil
	case <-t.listening:
		return 0, nil, errors.New("closed")
	}
}

func setTLSConfigServerName(tlsConf *tls.Config, addr net.Addr, host string) {
	// If no ServerName is set, infer the ServerName from the host we're connecting to.
	if tlsConf.ServerName != "" {
		return
	}
	if host == "" {
		if udpAddr, ok := addr.(*net.UDPAddr); ok {
			tlsConf.ServerName = udpAddr.IP.String()
			return
		}
	}
	h, _, err := net.SplitHostPort(host)
	if err != nil { // This happens if the host doesn't contain a port number.
		tlsConf.ServerName = host
		return
	}
	tlsConf.ServerName = h
}
