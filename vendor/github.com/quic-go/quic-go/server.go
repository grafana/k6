package quic

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go/internal/handshake"
	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/qerr"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/internal/wire"
	"github.com/quic-go/quic-go/logging"
)

// ErrServerClosed is returned by the Listener or EarlyListener's Accept method after a call to Close.
var ErrServerClosed = errors.New("quic: server closed")

// packetHandler handles packets
type packetHandler interface {
	handlePacket(receivedPacket)
	shutdown()
	destroy(error)
	getPerspective() protocol.Perspective
}

type packetHandlerManager interface {
	Get(protocol.ConnectionID) (packetHandler, bool)
	GetByResetToken(protocol.StatelessResetToken) (packetHandler, bool)
	AddWithConnID(protocol.ConnectionID, protocol.ConnectionID, func() (packetHandler, bool)) bool
	Close(error)
	connRunner
}

type quicConn interface {
	EarlyConnection
	earlyConnReady() <-chan struct{}
	handlePacket(receivedPacket)
	GetVersion() protocol.VersionNumber
	getPerspective() protocol.Perspective
	run() error
	destroy(error)
	shutdown()
}

type zeroRTTQueue struct {
	packets    []receivedPacket
	expiration time.Time
}

type rejectedPacket struct {
	receivedPacket
	hdr *wire.Header
}

// A Listener of QUIC
type baseServer struct {
	disableVersionNegotiation bool
	acceptEarlyConns          bool

	tlsConf *tls.Config
	config  *Config

	conn rawConn

	tokenGenerator *handshake.TokenGenerator
	maxTokenAge    time.Duration

	connIDGenerator ConnectionIDGenerator
	connHandler     packetHandlerManager
	onClose         func()

	receivedPackets chan receivedPacket

	nextZeroRTTCleanup time.Time
	zeroRTTQueues      map[protocol.ConnectionID]*zeroRTTQueue // only initialized if acceptEarlyConns == true

	// set as a member, so they can be set in the tests
	newConn func(
		sendConn,
		connRunner,
		protocol.ConnectionID, /* original dest connection ID */
		*protocol.ConnectionID, /* retry src connection ID */
		protocol.ConnectionID, /* client dest connection ID */
		protocol.ConnectionID, /* destination connection ID */
		protocol.ConnectionID, /* source connection ID */
		ConnectionIDGenerator,
		protocol.StatelessResetToken,
		*Config,
		*tls.Config,
		*handshake.TokenGenerator,
		bool, /* client address validated by an address validation token */
		*logging.ConnectionTracer,
		uint64,
		utils.Logger,
		protocol.VersionNumber,
	) quicConn

	closeOnce sync.Once
	errorChan chan struct{} // is closed when the server is closed
	closeErr  error
	running   chan struct{} // closed as soon as run() returns

	versionNegotiationQueue chan receivedPacket
	invalidTokenQueue       chan rejectedPacket
	connectionRefusedQueue  chan rejectedPacket
	retryQueue              chan rejectedPacket

	connQueue    chan quicConn
	connQueueLen int32 // to be used as an atomic

	tracer *logging.Tracer

	logger utils.Logger
}

// A Listener listens for incoming QUIC connections.
// It returns connections once the handshake has completed.
type Listener struct {
	baseServer *baseServer
}

// Accept returns new connections. It should be called in a loop.
func (l *Listener) Accept(ctx context.Context) (Connection, error) {
	return l.baseServer.Accept(ctx)
}

// Close closes the listener.
// Accept will return ErrServerClosed as soon as all connections in the accept queue have been accepted.
// QUIC handshakes that are still in flight will be rejected with a CONNECTION_REFUSED error.
// The effect of closing the listener depends on how it was created:
// * if it was created using Transport.Listen, already established connections will be unaffected
// * if it was created using the Listen convenience method, all established connection will be closed immediately
func (l *Listener) Close() error {
	return l.baseServer.Close()
}

// Addr returns the local network address that the server is listening on.
func (l *Listener) Addr() net.Addr {
	return l.baseServer.Addr()
}

// An EarlyListener listens for incoming QUIC connections, and returns them before the handshake completes.
// For connections that don't use 0-RTT, this allows the server to send 0.5-RTT data.
// This data is encrypted with forward-secure keys, however, the client's identity has not yet been verified.
// For connection using 0-RTT, this allows the server to accept and respond to streams that the client opened in the
// 0-RTT data it sent. Note that at this point during the handshake, the live-ness of the
// client has not yet been confirmed, and the 0-RTT data could have been replayed by an attacker.
type EarlyListener struct {
	baseServer *baseServer
}

// Accept returns a new connections. It should be called in a loop.
func (l *EarlyListener) Accept(ctx context.Context) (EarlyConnection, error) {
	return l.baseServer.accept(ctx)
}

// Close the server. All active connections will be closed.
func (l *EarlyListener) Close() error {
	return l.baseServer.Close()
}

// Addr returns the local network addr that the server is listening on.
func (l *EarlyListener) Addr() net.Addr {
	return l.baseServer.Addr()
}

// ListenAddr creates a QUIC server listening on a given address.
// See Listen for more details.
func ListenAddr(addr string, tlsConf *tls.Config, config *Config) (*Listener, error) {
	conn, err := listenUDP(addr)
	if err != nil {
		return nil, err
	}
	return (&Transport{
		Conn:        conn,
		createdConn: true,
		isSingleUse: true,
	}).Listen(tlsConf, config)
}

// ListenAddrEarly works like ListenAddr, but it returns connections before the handshake completes.
func ListenAddrEarly(addr string, tlsConf *tls.Config, config *Config) (*EarlyListener, error) {
	conn, err := listenUDP(addr)
	if err != nil {
		return nil, err
	}
	return (&Transport{
		Conn:        conn,
		createdConn: true,
		isSingleUse: true,
	}).ListenEarly(tlsConf, config)
}

func listenUDP(addr string) (*net.UDPConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	return net.ListenUDP("udp", udpAddr)
}

// Listen listens for QUIC connections on a given net.PacketConn.
// If the PacketConn satisfies the OOBCapablePacketConn interface (as a net.UDPConn does),
// ECN and packet info support will be enabled. In this case, ReadMsgUDP and WriteMsgUDP
// will be used instead of ReadFrom and WriteTo to read/write packets.
// A single net.PacketConn can only be used for a single call to Listen.
//
// The tls.Config must not be nil and must contain a certificate configuration.
// Furthermore, it must define an application control (using NextProtos).
// The quic.Config may be nil, in that case the default values will be used.
//
// This is a convenience function. More advanced use cases should instantiate a Transport,
// which offers configuration options for a more fine-grained control of the connection establishment,
// including reusing the underlying UDP socket for outgoing QUIC connections.
// When closing a listener created with Listen, all established QUIC connections will be closed immediately.
func Listen(conn net.PacketConn, tlsConf *tls.Config, config *Config) (*Listener, error) {
	tr := &Transport{Conn: conn, isSingleUse: true}
	return tr.Listen(tlsConf, config)
}

// ListenEarly works like Listen, but it returns connections before the handshake completes.
func ListenEarly(conn net.PacketConn, tlsConf *tls.Config, config *Config) (*EarlyListener, error) {
	tr := &Transport{Conn: conn, isSingleUse: true}
	return tr.ListenEarly(tlsConf, config)
}

func newServer(
	conn rawConn,
	connHandler packetHandlerManager,
	connIDGenerator ConnectionIDGenerator,
	tlsConf *tls.Config,
	config *Config,
	tracer *logging.Tracer,
	onClose func(),
	tokenGeneratorKey TokenGeneratorKey,
	maxTokenAge time.Duration,
	disableVersionNegotiation bool,
	acceptEarly bool,
) *baseServer {
	s := &baseServer{
		conn:                      conn,
		tlsConf:                   tlsConf,
		config:                    config,
		tokenGenerator:            handshake.NewTokenGenerator(tokenGeneratorKey),
		maxTokenAge:               maxTokenAge,
		connIDGenerator:           connIDGenerator,
		connHandler:               connHandler,
		connQueue:                 make(chan quicConn),
		errorChan:                 make(chan struct{}),
		running:                   make(chan struct{}),
		receivedPackets:           make(chan receivedPacket, protocol.MaxServerUnprocessedPackets),
		versionNegotiationQueue:   make(chan receivedPacket, 4),
		invalidTokenQueue:         make(chan rejectedPacket, 4),
		connectionRefusedQueue:    make(chan rejectedPacket, 4),
		retryQueue:                make(chan rejectedPacket, 8),
		newConn:                   newConnection,
		tracer:                    tracer,
		logger:                    utils.DefaultLogger.WithPrefix("server"),
		acceptEarlyConns:          acceptEarly,
		disableVersionNegotiation: disableVersionNegotiation,
		onClose:                   onClose,
	}
	if acceptEarly {
		s.zeroRTTQueues = map[protocol.ConnectionID]*zeroRTTQueue{}
	}
	go s.run()
	go s.runSendQueue()
	s.logger.Debugf("Listening for %s connections on %s", conn.LocalAddr().Network(), conn.LocalAddr().String())
	return s
}

func (s *baseServer) run() {
	defer close(s.running)
	for {
		select {
		case <-s.errorChan:
			return
		default:
		}
		select {
		case <-s.errorChan:
			return
		case p := <-s.receivedPackets:
			if bufferStillInUse := s.handlePacketImpl(p); !bufferStillInUse {
				p.buffer.Release()
			}
		}
	}
}

func (s *baseServer) runSendQueue() {
	for {
		select {
		case <-s.running:
			return
		case p := <-s.versionNegotiationQueue:
			s.maybeSendVersionNegotiationPacket(p)
		case p := <-s.invalidTokenQueue:
			s.maybeSendInvalidToken(p)
		case p := <-s.connectionRefusedQueue:
			s.sendConnectionRefused(p)
		case p := <-s.retryQueue:
			s.sendRetry(p)
		}
	}
}

// Accept returns connections that already completed the handshake.
// It is only valid if acceptEarlyConns is false.
func (s *baseServer) Accept(ctx context.Context) (Connection, error) {
	return s.accept(ctx)
}

func (s *baseServer) accept(ctx context.Context) (quicConn, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case conn := <-s.connQueue:
		atomic.AddInt32(&s.connQueueLen, -1)
		return conn, nil
	case <-s.errorChan:
		return nil, s.closeErr
	}
}

func (s *baseServer) Close() error {
	s.close(ErrServerClosed, true)
	return nil
}

func (s *baseServer) close(e error, notifyOnClose bool) {
	s.closeOnce.Do(func() {
		s.closeErr = e
		close(s.errorChan)

		<-s.running
		if notifyOnClose {
			s.onClose()
		}
	})
}

// Addr returns the server's network address
func (s *baseServer) Addr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *baseServer) handlePacket(p receivedPacket) {
	select {
	case s.receivedPackets <- p:
	default:
		s.logger.Debugf("Dropping packet from %s (%d bytes). Server receive queue full.", p.remoteAddr, p.Size())
		if s.tracer != nil && s.tracer.DroppedPacket != nil {
			s.tracer.DroppedPacket(p.remoteAddr, logging.PacketTypeNotDetermined, p.Size(), logging.PacketDropDOSPrevention)
		}
	}
}

func (s *baseServer) handlePacketImpl(p receivedPacket) bool /* is the buffer still in use? */ {
	if !s.nextZeroRTTCleanup.IsZero() && p.rcvTime.After(s.nextZeroRTTCleanup) {
		defer s.cleanupZeroRTTQueues(p.rcvTime)
	}

	if wire.IsVersionNegotiationPacket(p.data) {
		s.logger.Debugf("Dropping Version Negotiation packet.")
		if s.tracer != nil && s.tracer.DroppedPacket != nil {
			s.tracer.DroppedPacket(p.remoteAddr, logging.PacketTypeVersionNegotiation, p.Size(), logging.PacketDropUnexpectedPacket)
		}
		return false
	}
	// Short header packets should never end up here in the first place
	if !wire.IsLongHeaderPacket(p.data[0]) {
		panic(fmt.Sprintf("misrouted packet: %#v", p.data))
	}
	v, err := wire.ParseVersion(p.data)
	// drop the packet if we failed to parse the protocol version
	if err != nil {
		s.logger.Debugf("Dropping a packet with an unknown version")
		if s.tracer != nil && s.tracer.DroppedPacket != nil {
			s.tracer.DroppedPacket(p.remoteAddr, logging.PacketTypeNotDetermined, p.Size(), logging.PacketDropUnexpectedPacket)
		}
		return false
	}
	// send a Version Negotiation Packet if the client is speaking a different protocol version
	if !protocol.IsSupportedVersion(s.config.Versions, v) {
		if s.disableVersionNegotiation {
			return false
		}

		if p.Size() < protocol.MinUnknownVersionPacketSize {
			s.logger.Debugf("Dropping a packet with an unsupported version number %d that is too small (%d bytes)", v, p.Size())
			if s.tracer != nil && s.tracer.DroppedPacket != nil {
				s.tracer.DroppedPacket(p.remoteAddr, logging.PacketTypeNotDetermined, p.Size(), logging.PacketDropUnexpectedPacket)
			}
			return false
		}
		return s.enqueueVersionNegotiationPacket(p)
	}

	if wire.Is0RTTPacket(p.data) {
		if !s.acceptEarlyConns {
			if s.tracer != nil && s.tracer.DroppedPacket != nil {
				s.tracer.DroppedPacket(p.remoteAddr, logging.PacketType0RTT, p.Size(), logging.PacketDropUnexpectedPacket)
			}
			return false
		}
		return s.handle0RTTPacket(p)
	}

	// If we're creating a new connection, the packet will be passed to the connection.
	// The header will then be parsed again.
	hdr, _, _, err := wire.ParsePacket(p.data)
	if err != nil {
		if s.tracer != nil && s.tracer.DroppedPacket != nil {
			s.tracer.DroppedPacket(p.remoteAddr, logging.PacketTypeNotDetermined, p.Size(), logging.PacketDropHeaderParseError)
		}
		s.logger.Debugf("Error parsing packet: %s", err)
		return false
	}
	if hdr.Type == protocol.PacketTypeInitial && p.Size() < protocol.MinInitialPacketSize {
		s.logger.Debugf("Dropping a packet that is too small to be a valid Initial (%d bytes)", p.Size())
		if s.tracer != nil && s.tracer.DroppedPacket != nil {
			s.tracer.DroppedPacket(p.remoteAddr, logging.PacketTypeInitial, p.Size(), logging.PacketDropUnexpectedPacket)
		}
		return false
	}

	if hdr.Type != protocol.PacketTypeInitial {
		// Drop long header packets.
		// There's little point in sending a Stateless Reset, since the client
		// might not have received the token yet.
		s.logger.Debugf("Dropping long header packet of type %s (%d bytes)", hdr.Type, len(p.data))
		if s.tracer != nil && s.tracer.DroppedPacket != nil {
			s.tracer.DroppedPacket(p.remoteAddr, logging.PacketTypeFromHeader(hdr), p.Size(), logging.PacketDropUnexpectedPacket)
		}
		return false
	}

	s.logger.Debugf("<- Received Initial packet.")

	if err := s.handleInitialImpl(p, hdr); err != nil {
		s.logger.Errorf("Error occurred handling initial packet: %s", err)
	}
	// Don't put the packet buffer back.
	// handleInitialImpl deals with the buffer.
	return true
}

func (s *baseServer) handle0RTTPacket(p receivedPacket) bool {
	connID, err := wire.ParseConnectionID(p.data, 0)
	if err != nil {
		if s.tracer != nil && s.tracer.DroppedPacket != nil {
			s.tracer.DroppedPacket(p.remoteAddr, logging.PacketType0RTT, p.Size(), logging.PacketDropHeaderParseError)
		}
		return false
	}

	// check again if we might have a connection now
	if handler, ok := s.connHandler.Get(connID); ok {
		handler.handlePacket(p)
		return true
	}

	if q, ok := s.zeroRTTQueues[connID]; ok {
		if len(q.packets) >= protocol.Max0RTTQueueLen {
			if s.tracer != nil && s.tracer.DroppedPacket != nil {
				s.tracer.DroppedPacket(p.remoteAddr, logging.PacketType0RTT, p.Size(), logging.PacketDropDOSPrevention)
			}
			return false
		}
		q.packets = append(q.packets, p)
		return true
	}

	if len(s.zeroRTTQueues) >= protocol.Max0RTTQueues {
		if s.tracer != nil && s.tracer.DroppedPacket != nil {
			s.tracer.DroppedPacket(p.remoteAddr, logging.PacketType0RTT, p.Size(), logging.PacketDropDOSPrevention)
		}
		return false
	}
	queue := &zeroRTTQueue{packets: make([]receivedPacket, 1, 8)}
	queue.packets[0] = p
	expiration := p.rcvTime.Add(protocol.Max0RTTQueueingDuration)
	queue.expiration = expiration
	if s.nextZeroRTTCleanup.IsZero() || s.nextZeroRTTCleanup.After(expiration) {
		s.nextZeroRTTCleanup = expiration
	}
	s.zeroRTTQueues[connID] = queue
	return true
}

func (s *baseServer) cleanupZeroRTTQueues(now time.Time) {
	// Iterate over all queues to find those that are expired.
	// This is ok since we're placing a pretty low limit on the number of queues.
	var nextCleanup time.Time
	for connID, q := range s.zeroRTTQueues {
		if q.expiration.After(now) {
			if nextCleanup.IsZero() || nextCleanup.After(q.expiration) {
				nextCleanup = q.expiration
			}
			continue
		}
		for _, p := range q.packets {
			if s.tracer != nil && s.tracer.DroppedPacket != nil {
				s.tracer.DroppedPacket(p.remoteAddr, logging.PacketType0RTT, p.Size(), logging.PacketDropDOSPrevention)
			}
			p.buffer.Release()
		}
		delete(s.zeroRTTQueues, connID)
		if s.logger.Debug() {
			s.logger.Debugf("Removing 0-RTT queue for %s.", connID)
		}
	}
	s.nextZeroRTTCleanup = nextCleanup
}

// validateToken returns false if:
//   - address is invalid
//   - token is expired
//   - token is null
func (s *baseServer) validateToken(token *handshake.Token, addr net.Addr) bool {
	if token == nil {
		return false
	}
	if !token.ValidateRemoteAddr(addr) {
		return false
	}
	if !token.IsRetryToken && time.Since(token.SentTime) > s.maxTokenAge {
		return false
	}
	if token.IsRetryToken && time.Since(token.SentTime) > s.config.maxRetryTokenAge() {
		return false
	}
	return true
}

func (s *baseServer) handleInitialImpl(p receivedPacket, hdr *wire.Header) error {
	if len(hdr.Token) == 0 && hdr.DestConnectionID.Len() < protocol.MinConnectionIDLenInitial {
		p.buffer.Release()
		if s.tracer != nil && s.tracer.DroppedPacket != nil {
			s.tracer.DroppedPacket(p.remoteAddr, logging.PacketTypeInitial, p.Size(), logging.PacketDropUnexpectedPacket)
		}
		return errors.New("too short connection ID")
	}

	// The server queues packets for a while, and we might already have established a connection by now.
	// This results in a second check in the connection map.
	// That's ok since it's not the hot path (it's only taken by some Initial and 0-RTT packets).
	if handler, ok := s.connHandler.Get(hdr.DestConnectionID); ok {
		handler.handlePacket(p)
		return nil
	}

	var (
		token          *handshake.Token
		retrySrcConnID *protocol.ConnectionID
	)
	origDestConnID := hdr.DestConnectionID
	if len(hdr.Token) > 0 {
		tok, err := s.tokenGenerator.DecodeToken(hdr.Token)
		if err == nil {
			if tok.IsRetryToken {
				origDestConnID = tok.OriginalDestConnectionID
				retrySrcConnID = &tok.RetrySrcConnectionID
			}
			token = tok
		}
	}

	clientAddrIsValid := s.validateToken(token, p.remoteAddr)
	if token != nil && !clientAddrIsValid {
		// For invalid and expired non-retry tokens, we don't send an INVALID_TOKEN error.
		// We just ignore them, and act as if there was no token on this packet at all.
		// This also means we might send a Retry later.
		if !token.IsRetryToken {
			token = nil
		} else {
			// For Retry tokens, we send an INVALID_ERROR if
			// * the token is too old, or
			// * the token is invalid, in case of a retry token.
			select {
			case s.invalidTokenQueue <- rejectedPacket{receivedPacket: p, hdr: hdr}:
			default:
				// drop packet if we can't send out the  INVALID_TOKEN packets fast enough
				p.buffer.Release()
			}
			return nil
		}
	}
	if token == nil && s.config.RequireAddressValidation(p.remoteAddr) {
		// Retry invalidates all 0-RTT packets sent.
		delete(s.zeroRTTQueues, hdr.DestConnectionID)
		select {
		case s.retryQueue <- rejectedPacket{receivedPacket: p, hdr: hdr}:
		default:
			// drop packet if we can't send out Retry packets fast enough
			p.buffer.Release()
		}
		return nil
	}

	if queueLen := atomic.LoadInt32(&s.connQueueLen); queueLen >= protocol.MaxAcceptQueueSize {
		s.logger.Debugf("Rejecting new connection. Server currently busy. Accept queue length: %d (max %d)", queueLen, protocol.MaxAcceptQueueSize)
		select {
		case s.connectionRefusedQueue <- rejectedPacket{receivedPacket: p, hdr: hdr}:
		default:
			// drop packet if we can't send out the CONNECTION_REFUSED fast enough
			p.buffer.Release()
		}
		return nil
	}

	connID, err := s.connIDGenerator.GenerateConnectionID()
	if err != nil {
		return err
	}
	s.logger.Debugf("Changing connection ID to %s.", connID)
	var conn quicConn
	tracingID := nextConnTracingID()
	if added := s.connHandler.AddWithConnID(hdr.DestConnectionID, connID, func() (packetHandler, bool) {
		config := s.config
		if s.config.GetConfigForClient != nil {
			conf, err := s.config.GetConfigForClient(&ClientHelloInfo{RemoteAddr: p.remoteAddr})
			if err != nil {
				s.logger.Debugf("Rejecting new connection due to GetConfigForClient callback")
				return nil, false
			}
			config = populateConfig(conf)
		}
		var tracer *logging.ConnectionTracer
		if config.Tracer != nil {
			// Use the same connection ID that is passed to the client's GetLogWriter callback.
			connID := hdr.DestConnectionID
			if origDestConnID.Len() > 0 {
				connID = origDestConnID
			}
			tracer = config.Tracer(context.WithValue(context.Background(), ConnectionTracingKey, tracingID), protocol.PerspectiveServer, connID)
		}
		conn = s.newConn(
			newSendConn(s.conn, p.remoteAddr, p.info, s.logger),
			s.connHandler,
			origDestConnID,
			retrySrcConnID,
			hdr.DestConnectionID,
			hdr.SrcConnectionID,
			connID,
			s.connIDGenerator,
			s.connHandler.GetStatelessResetToken(connID),
			config,
			s.tlsConf,
			s.tokenGenerator,
			clientAddrIsValid,
			tracer,
			tracingID,
			s.logger,
			hdr.Version,
		)
		conn.handlePacket(p)

		if q, ok := s.zeroRTTQueues[hdr.DestConnectionID]; ok {
			for _, p := range q.packets {
				conn.handlePacket(p)
			}
			delete(s.zeroRTTQueues, hdr.DestConnectionID)
		}

		return conn, true
	}); !added {
		select {
		case s.connectionRefusedQueue <- rejectedPacket{receivedPacket: p, hdr: hdr}:
		default:
			// drop packet if we can't send out the CONNECTION_REFUSED fast enough
			p.buffer.Release()
		}
		return nil
	}
	go conn.run()
	go s.handleNewConn(conn)
	if conn == nil {
		p.buffer.Release()
		return nil
	}
	return nil
}

func (s *baseServer) handleNewConn(conn quicConn) {
	connCtx := conn.Context()
	if s.acceptEarlyConns {
		// wait until the early connection is ready, the handshake fails, or the server is closed
		select {
		case <-s.errorChan:
			conn.destroy(&qerr.TransportError{ErrorCode: ConnectionRefused})
			return
		case <-conn.earlyConnReady():
		case <-connCtx.Done():
			return
		}
	} else {
		// wait until the handshake is complete (or fails)
		select {
		case <-s.errorChan:
			conn.destroy(&qerr.TransportError{ErrorCode: ConnectionRefused})
			return
		case <-conn.HandshakeComplete():
		case <-connCtx.Done():
			return
		}
	}

	atomic.AddInt32(&s.connQueueLen, 1)
	select {
	case s.connQueue <- conn:
		// blocks until the connection is accepted
	case <-connCtx.Done():
		atomic.AddInt32(&s.connQueueLen, -1)
		// don't pass connections that were already closed to Accept()
	}
}

func (s *baseServer) sendRetry(p rejectedPacket) {
	if err := s.sendRetryPacket(p); err != nil {
		s.logger.Debugf("Error sending Retry packet: %s", err)
	}
}

func (s *baseServer) sendRetryPacket(p rejectedPacket) error {
	hdr := p.hdr
	// Log the Initial packet now.
	// If no Retry is sent, the packet will be logged by the connection.
	(&wire.ExtendedHeader{Header: *hdr}).Log(s.logger)
	srcConnID, err := s.connIDGenerator.GenerateConnectionID()
	if err != nil {
		return err
	}
	token, err := s.tokenGenerator.NewRetryToken(p.remoteAddr, hdr.DestConnectionID, srcConnID)
	if err != nil {
		return err
	}
	replyHdr := &wire.ExtendedHeader{}
	replyHdr.Type = protocol.PacketTypeRetry
	replyHdr.Version = hdr.Version
	replyHdr.SrcConnectionID = srcConnID
	replyHdr.DestConnectionID = hdr.SrcConnectionID
	replyHdr.Token = token
	if s.logger.Debug() {
		s.logger.Debugf("Changing connection ID to %s.", srcConnID)
		s.logger.Debugf("-> Sending Retry")
		replyHdr.Log(s.logger)
	}

	buf := getPacketBuffer()
	defer buf.Release()
	buf.Data, err = replyHdr.Append(buf.Data, hdr.Version)
	if err != nil {
		return err
	}
	// append the Retry integrity tag
	tag := handshake.GetRetryIntegrityTag(buf.Data, hdr.DestConnectionID, hdr.Version)
	buf.Data = append(buf.Data, tag[:]...)
	if s.tracer != nil && s.tracer.SentPacket != nil {
		s.tracer.SentPacket(p.remoteAddr, &replyHdr.Header, protocol.ByteCount(len(buf.Data)), nil)
	}
	_, err = s.conn.WritePacket(buf.Data, p.remoteAddr, p.info.OOB(), 0, protocol.ECNUnsupported)
	return err
}

func (s *baseServer) maybeSendInvalidToken(p rejectedPacket) {
	defer p.buffer.Release()

	// Only send INVALID_TOKEN if we can unprotect the packet.
	// This makes sure that we won't send it for packets that were corrupted.
	hdr := p.hdr
	sealer, opener := handshake.NewInitialAEAD(hdr.DestConnectionID, protocol.PerspectiveServer, hdr.Version)
	data := p.data[:hdr.ParsedLen()+hdr.Length]
	extHdr, err := unpackLongHeader(opener, hdr, data, hdr.Version)
	// Only send INVALID_TOKEN if we can unprotect the packet.
	// This makes sure that we won't send it for packets that were corrupted.
	if err != nil {
		if s.tracer != nil && s.tracer.DroppedPacket != nil {
			s.tracer.DroppedPacket(p.remoteAddr, logging.PacketTypeInitial, p.Size(), logging.PacketDropHeaderParseError)
		}
		return
	}
	hdrLen := extHdr.ParsedLen()
	if _, err := opener.Open(data[hdrLen:hdrLen], data[hdrLen:], extHdr.PacketNumber, data[:hdrLen]); err != nil {
		if s.tracer != nil && s.tracer.DroppedPacket != nil {
			s.tracer.DroppedPacket(p.remoteAddr, logging.PacketTypeInitial, p.Size(), logging.PacketDropPayloadDecryptError)
		}
		return
	}
	if s.logger.Debug() {
		s.logger.Debugf("Client sent an invalid retry token. Sending INVALID_TOKEN to %s.", p.remoteAddr)
	}
	if err := s.sendError(p.remoteAddr, hdr, sealer, qerr.InvalidToken, p.info); err != nil {
		s.logger.Debugf("Error sending INVALID_TOKEN error: %s", err)
	}
}

func (s *baseServer) sendConnectionRefused(p rejectedPacket) {
	defer p.buffer.Release()
	sealer, _ := handshake.NewInitialAEAD(p.hdr.DestConnectionID, protocol.PerspectiveServer, p.hdr.Version)
	if err := s.sendError(p.remoteAddr, p.hdr, sealer, qerr.ConnectionRefused, p.info); err != nil {
		s.logger.Debugf("Error sending CONNECTION_REFUSED error: %s", err)
	}
}

// sendError sends the error as a response to the packet received with header hdr
func (s *baseServer) sendError(remoteAddr net.Addr, hdr *wire.Header, sealer handshake.LongHeaderSealer, errorCode qerr.TransportErrorCode, info packetInfo) error {
	b := getPacketBuffer()
	defer b.Release()

	ccf := &wire.ConnectionCloseFrame{ErrorCode: uint64(errorCode)}

	replyHdr := &wire.ExtendedHeader{}
	replyHdr.Type = protocol.PacketTypeInitial
	replyHdr.Version = hdr.Version
	replyHdr.SrcConnectionID = hdr.DestConnectionID
	replyHdr.DestConnectionID = hdr.SrcConnectionID
	replyHdr.PacketNumberLen = protocol.PacketNumberLen4
	replyHdr.Length = 4 /* packet number len */ + ccf.Length(hdr.Version) + protocol.ByteCount(sealer.Overhead())
	var err error
	b.Data, err = replyHdr.Append(b.Data, hdr.Version)
	if err != nil {
		return err
	}
	payloadOffset := len(b.Data)

	b.Data, err = ccf.Append(b.Data, hdr.Version)
	if err != nil {
		return err
	}

	_ = sealer.Seal(b.Data[payloadOffset:payloadOffset], b.Data[payloadOffset:], replyHdr.PacketNumber, b.Data[:payloadOffset])
	b.Data = b.Data[0 : len(b.Data)+sealer.Overhead()]

	pnOffset := payloadOffset - int(replyHdr.PacketNumberLen)
	sealer.EncryptHeader(
		b.Data[pnOffset+4:pnOffset+4+16],
		&b.Data[0],
		b.Data[pnOffset:payloadOffset],
	)

	replyHdr.Log(s.logger)
	wire.LogFrame(s.logger, ccf, true)
	if s.tracer != nil && s.tracer.SentPacket != nil {
		s.tracer.SentPacket(remoteAddr, &replyHdr.Header, protocol.ByteCount(len(b.Data)), []logging.Frame{ccf})
	}
	_, err = s.conn.WritePacket(b.Data, remoteAddr, info.OOB(), 0, protocol.ECNUnsupported)
	return err
}

func (s *baseServer) enqueueVersionNegotiationPacket(p receivedPacket) (bufferInUse bool) {
	select {
	case s.versionNegotiationQueue <- p:
		return true
	default:
		// it's fine to not send version negotiation packets when we are busy
	}
	return false
}

func (s *baseServer) maybeSendVersionNegotiationPacket(p receivedPacket) {
	defer p.buffer.Release()

	v, err := wire.ParseVersion(p.data)
	if err != nil {
		s.logger.Debugf("failed to parse version for sending version negotiation packet: %s", err)
		return
	}

	_, src, dest, err := wire.ParseArbitraryLenConnectionIDs(p.data)
	if err != nil { // should never happen
		s.logger.Debugf("Dropping a packet with an unknown version for which we failed to parse connection IDs")
		if s.tracer != nil && s.tracer.DroppedPacket != nil {
			s.tracer.DroppedPacket(p.remoteAddr, logging.PacketTypeNotDetermined, p.Size(), logging.PacketDropUnexpectedPacket)
		}
		return
	}

	s.logger.Debugf("Client offered version %s, sending Version Negotiation", v)

	data := wire.ComposeVersionNegotiation(dest, src, s.config.Versions)
	if s.tracer != nil && s.tracer.SentVersionNegotiationPacket != nil {
		s.tracer.SentVersionNegotiationPacket(p.remoteAddr, src, dest, s.config.Versions)
	}
	if _, err := s.conn.WritePacket(data, p.remoteAddr, p.info.OOB(), 0, protocol.ECNUnsupported); err != nil {
		s.logger.Debugf("Error sending Version Negotiation: %s", err)
	}
}
