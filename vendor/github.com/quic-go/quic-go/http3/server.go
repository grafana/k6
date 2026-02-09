package http3

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3/qlog"
	"github.com/quic-go/quic-go/qlogwriter"
)

// NextProtoH3 is the ALPN protocol negotiated during the TLS handshake, for QUIC v1 and v2.
const NextProtoH3 = "h3"

// StreamType is the stream type of a unidirectional stream.
type StreamType uint64

const (
	streamTypeControlStream      = 0
	streamTypePushStream         = 1
	streamTypeQPACKEncoderStream = 2
	streamTypeQPACKDecoderStream = 3
)

// A QUICListener listens for incoming QUIC connections.
type QUICListener interface {
	Accept(context.Context) (*quic.Conn, error)
	Addr() net.Addr
	io.Closer
}

var _ QUICListener = &quic.EarlyListener{}

// ConfigureTLSConfig creates a new tls.Config which can be used
// to create a quic.Listener meant for serving HTTP/3.
func ConfigureTLSConfig(tlsConf *tls.Config) *tls.Config {
	// Workaround for https://github.com/golang/go/issues/60506.
	// This initializes the session tickets _before_ cloning the config.
	_, _ = tlsConf.DecryptTicket(nil, tls.ConnectionState{})
	config := tlsConf.Clone()
	config.NextProtos = []string{NextProtoH3}
	if gfc := config.GetConfigForClient; gfc != nil {
		config.GetConfigForClient = func(ch *tls.ClientHelloInfo) (*tls.Config, error) {
			conf, err := gfc(ch)
			if conf == nil || err != nil {
				return conf, err
			}
			return ConfigureTLSConfig(conf), nil
		}
	}
	return config
}

// contextKey is a value for use with context.WithValue. It's used as
// a pointer so it fits in an interface{} without allocation.
type contextKey struct {
	name string
}

func (k *contextKey) String() string { return "quic-go/http3 context value " + k.name }

// ServerContextKey is a context key. It can be used in HTTP
// handlers with Context.Value to access the server that
// started the handler. The associated value will be of
// type *http3.Server.
var ServerContextKey = &contextKey{"http3-server"}

// RemoteAddrContextKey is a context key. It can be used in
// HTTP handlers with Context.Value to access the remote
// address of the connection. The associated value will be of
// type net.Addr.
//
// Use this value instead of [http.Request.RemoteAddr] if you
// require access to the remote address of the connection rather
// than its string representation.
var RemoteAddrContextKey = &contextKey{"remote-addr"}

// listener contains info about specific listener added with addListener
type listener struct {
	ln   *QUICListener
	port int // 0 means that no info about port is available

	// if this listener was constructed by the application, it won't be closed when the server is closed
	createdLocally bool
}

// Server is a HTTP/3 server.
type Server struct {
	// Addr optionally specifies the UDP address for the server to listen on,
	// in the form "host:port".
	//
	// When used by ListenAndServe and ListenAndServeTLS methods, if empty,
	// ":https" (port 443) is used. See net.Dial for details of the address
	// format.
	//
	// Otherwise, if Port is not set and underlying QUIC listeners do not
	// have valid port numbers, the port part is used in Alt-Svc headers set
	// with SetQUICHeaders.
	Addr string

	// Port is used in Alt-Svc response headers set with SetQUICHeaders. If
	// needed Port can be manually set when the Server is created.
	//
	// This is useful when a Layer 4 firewall is redirecting UDP traffic and
	// clients must use a port different from the port the Server is
	// listening on.
	Port int

	// TLSConfig provides a TLS configuration for use by server. It must be
	// set for ListenAndServe and Serve methods.
	TLSConfig *tls.Config

	// QUICConfig provides the parameters for QUIC connection created with Serve.
	// If nil, it uses reasonable default values.
	//
	// Configured versions are also used in Alt-Svc response header set with SetQUICHeaders.
	QUICConfig *quic.Config

	// Handler is the HTTP request handler to use. If not set, defaults to
	// http.NotFound.
	Handler http.Handler

	// EnableDatagrams enables support for HTTP/3 datagrams (RFC 9297).
	// If set to true, QUICConfig.EnableDatagrams will be set.
	EnableDatagrams bool

	// MaxHeaderBytes controls the maximum number of bytes the server will
	// read parsing the request HEADERS frame. It does not limit the size of
	// the request body. If zero or negative, http.DefaultMaxHeaderBytes is
	// used.
	MaxHeaderBytes int

	// AdditionalSettings specifies additional HTTP/3 settings.
	// It is invalid to specify any settings defined by RFC 9114 (HTTP/3) and RFC 9297 (HTTP Datagrams).
	AdditionalSettings map[uint64]uint64

	// IdleTimeout specifies how long until idle clients connection should be
	// closed. Idle refers only to the HTTP/3 layer, activity at the QUIC layer
	// like PING frames are not considered.
	// If zero or negative, there is no timeout.
	IdleTimeout time.Duration

	// ConnContext optionally specifies a function that modifies the context used for a new connection c.
	// The provided ctx has a ServerContextKey value.
	ConnContext func(ctx context.Context, c *quic.Conn) context.Context

	Logger *slog.Logger

	mutex     sync.RWMutex
	listeners []listener

	closed           bool
	closeCtx         context.Context    // canceled when the server is closed
	closeCancel      context.CancelFunc // cancels the closeCtx
	graceCtx         context.Context    // canceled when the server is closed or gracefully closed
	graceCancel      context.CancelFunc // cancels the graceCtx
	connCount        atomic.Int64
	connHandlingDone chan struct{}

	altSvcHeader string
}

// ListenAndServe listens on the UDP address s.Addr and calls s.Handler to handle HTTP/3 requests on incoming connections.
//
// If s.Addr is blank, ":https" is used.
func (s *Server) ListenAndServe() error {
	ln, err := s.setupListenerForConn(s.TLSConfig, nil)
	if err != nil {
		return err
	}
	defer s.removeListener(ln)

	return s.serveListener(*ln)
}

// ListenAndServeTLS listens on the UDP address s.Addr and calls s.Handler to handle HTTP/3 requests on incoming connections.
//
// If s.Addr is blank, ":https" is used.
func (s *Server) ListenAndServeTLS(certFile, keyFile string) error {
	var err error
	certs := make([]tls.Certificate, 1)
	certs[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}
	// We currently only use the cert-related stuff from tls.Config,
	// so we don't need to make a full copy.
	ln, err := s.setupListenerForConn(&tls.Config{Certificates: certs}, nil)
	if err != nil {
		return err
	}
	defer s.removeListener(ln)

	return s.serveListener(*ln)
}

// Serve an existing UDP connection.
// It is possible to reuse the same connection for outgoing connections.
// Closing the server does not close the connection.
func (s *Server) Serve(conn net.PacketConn) error {
	ln, err := s.setupListenerForConn(s.TLSConfig, conn)
	if err != nil {
		return err
	}
	defer s.removeListener(ln)

	return s.serveListener(*ln)
}

// init initializes the contexts used for shutting down the server.
// It must be called with the mutex held.
func (s *Server) init() {
	if s.closeCtx == nil {
		s.closeCtx, s.closeCancel = context.WithCancel(context.Background())
		s.graceCtx, s.graceCancel = context.WithCancel(s.closeCtx)
	}
	s.connHandlingDone = make(chan struct{}, 1)
}

func (s *Server) decreaseConnCount() {
	if s.connCount.Add(-1) == 0 && s.graceCtx.Err() != nil {
		close(s.connHandlingDone)
	}
}

// ServeQUICConn serves a single QUIC connection.
func (s *Server) ServeQUICConn(conn *quic.Conn) error {
	s.mutex.Lock()
	if s.closed {
		s.mutex.Unlock()
		return http.ErrServerClosed
	}

	s.init()
	s.mutex.Unlock()

	s.connCount.Add(1)
	defer s.decreaseConnCount()

	return s.handleConn(conn)
}

// ServeListener serves an existing QUIC listener.
// Make sure you use http3.ConfigureTLSConfig to configure a tls.Config
// and use it to construct a http3-friendly QUIC listener.
// Closing the server does not close the listener. It is the application's responsibility to close them.
// ServeListener always returns a non-nil error. After Shutdown or Close, the returned error is http.ErrServerClosed.
func (s *Server) ServeListener(ln QUICListener) error {
	s.mutex.Lock()
	if err := s.addListener(&ln, false); err != nil {
		s.mutex.Unlock()
		return err
	}
	s.mutex.Unlock()
	defer s.removeListener(&ln)

	return s.serveListener(ln)
}

func (s *Server) serveListener(ln QUICListener) error {
	for {
		conn, err := ln.Accept(s.graceCtx)
		// server closed
		if errors.Is(err, quic.ErrServerClosed) || s.graceCtx.Err() != nil {
			return http.ErrServerClosed
		}
		if err != nil {
			return err
		}
		s.connCount.Add(1)
		go func() {
			defer s.decreaseConnCount()
			if err := s.handleConn(conn); err != nil {
				if s.Logger != nil {
					s.Logger.Debug("handling connection failed", "error", err)
				}
			}
		}()
	}
}

var errServerWithoutTLSConfig = errors.New("use of http3.Server without TLSConfig")

func (s *Server) setupListenerForConn(tlsConf *tls.Config, conn net.PacketConn) (*QUICListener, error) {
	if tlsConf == nil {
		return nil, errServerWithoutTLSConfig
	}

	baseConf := ConfigureTLSConfig(tlsConf)
	quicConf := s.QUICConfig
	if quicConf == nil {
		quicConf = &quic.Config{Allow0RTT: true}
	} else {
		quicConf = s.QUICConfig.Clone()
	}
	if s.EnableDatagrams {
		quicConf.EnableDatagrams = true
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	closed := s.closed
	if closed {
		return nil, http.ErrServerClosed
	}

	var ln QUICListener
	var err error
	if conn == nil {
		addr := s.Addr
		if addr == "" {
			addr = ":https"
		}
		ln, err = quic.ListenAddrEarly(addr, baseConf, quicConf)
	} else {
		ln, err = quic.ListenEarly(conn, baseConf, quicConf)
	}
	if err != nil {
		return nil, err
	}
	if err := s.addListener(&ln, true); err != nil {
		return nil, err
	}
	return &ln, nil
}

func extractPort(addr string) (int, error) {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}

	portInt, err := net.LookupPort("tcp", portStr)
	if err != nil {
		return 0, err
	}
	return portInt, nil
}

func (s *Server) generateAltSvcHeader() {
	if len(s.listeners) == 0 {
		// Don't announce any ports since no one is listening for connections
		s.altSvcHeader = ""
		return
	}

	// This code assumes that we will use protocol.SupportedVersions if no quic.Config is passed.

	var altSvc []string
	addPort := func(port int) {
		altSvc = append(altSvc, fmt.Sprintf(`%s=":%d"; ma=2592000`, NextProtoH3, port))
	}

	if s.Port != 0 {
		// if Port is specified, we must use it instead of the
		// listener addresses since there's a reason it's specified.
		addPort(s.Port)
	} else {
		// if we have some listeners assigned, try to find ports
		// which we can announce, otherwise nothing should be announced
		validPortsFound := false
		for _, info := range s.listeners {
			if info.port != 0 {
				addPort(info.port)
				validPortsFound = true
			}
		}
		if !validPortsFound {
			if port, err := extractPort(s.Addr); err == nil {
				addPort(port)
			}
		}
	}

	s.altSvcHeader = strings.Join(altSvc, ",")
}

func (s *Server) addListener(l *QUICListener, createdLocally bool) error {
	if s.closed {
		return http.ErrServerClosed
	}
	s.init()

	laddr := (*l).Addr()
	if port, err := extractPort(laddr.String()); err == nil {
		s.listeners = append(s.listeners, listener{ln: l, port: port, createdLocally: createdLocally})
	} else {
		logger := s.Logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Error("Unable to extract port from listener, will not be announced using SetQUICHeaders", "local addr", laddr, "error", err)
		s.listeners = append(s.listeners, listener{ln: l, port: 0, createdLocally: createdLocally})
	}
	s.generateAltSvcHeader()
	return nil
}

func (s *Server) removeListener(l *QUICListener) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.listeners = slices.DeleteFunc(s.listeners, func(info listener) bool {
		return info.ln == l
	})
	s.generateAltSvcHeader()
}

func (s *Server) NewRawServerConn(conn *quic.Conn) (*RawServerConn, error) {
	hconn, _, _, err := s.newRawServerConn(conn)
	if err != nil {
		return nil, err
	}
	return hconn, nil
}

func (s *Server) newRawServerConn(conn *quic.Conn) (*RawServerConn, *quic.SendStream, qlogwriter.Recorder, error) {
	var qlogger qlogwriter.Recorder
	if qlogTrace := conn.QlogTrace(); qlogTrace != nil && qlogTrace.SupportsSchemas(qlog.EventSchema) {
		qlogger = qlogTrace.AddProducer()
	}
	connCtx := conn.Context()
	connCtx = context.WithValue(connCtx, ServerContextKey, s)
	connCtx = context.WithValue(connCtx, http.LocalAddrContextKey, conn.LocalAddr())
	connCtx = context.WithValue(connCtx, RemoteAddrContextKey, conn.RemoteAddr())
	if s.ConnContext != nil {
		connCtx = s.ConnContext(connCtx, conn)
		if connCtx == nil {
			panic("http3: ConnContext returned nil")
		}
	}
	hconn := newRawServerConn(
		conn,
		s.EnableDatagrams,
		s.IdleTimeout,
		qlogger,
		s.Logger,
		connCtx,
		s.Handler,
		s.maxHeaderBytes(),
	)

	// open the control stream and send a SETTINGS frame, it's also used to send a GOAWAY frame later
	// when the server is gracefully closed
	ctrlStr, err := hconn.openControlStream(&settingsFrame{
		MaxFieldSectionSize: int64(s.maxHeaderBytes()),
		Datagram:            s.EnableDatagrams,
		ExtendedConnect:     true,
		Other:               s.AdditionalSettings,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("opening the control stream failed: %w", err)
	}
	return hconn, ctrlStr, qlogger, nil
}

// handleConn handles the HTTP/3 exchange on a QUIC connection.
// It blocks until all HTTP handlers for all streams have returned.
func (s *Server) handleConn(conn *quic.Conn) error {
	hconn, ctrlStr, qlogger, err := s.newRawServerConn(conn)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for {
			str, err := conn.AcceptUniStream(context.Background())
			if err != nil {
				return
			}
			go hconn.HandleUnidirectionalStream(str)
		}
	}()

	var nextStreamID quic.StreamID
	var handleErr error
	var inGracefulShutdown bool
	// Process all requests immediately.
	// It's the client's responsibility to decide which requests are eligible for 0-RTT.
	ctx := s.graceCtx
	for {
		// The context used here is:
		// * before graceful shutdown: s.graceCtx
		// * after graceful shutdown: s.closeCtx
		// This allows us to keep accepting (and resetting) streams after graceful shutdown has started.
		str, err := conn.AcceptStream(ctx)
		if err != nil {
			// the underlying connection was closed (by either side)
			if conn.Context().Err() != nil {
				var appErr *quic.ApplicationError
				if !errors.As(err, &appErr) || appErr.ErrorCode != quic.ApplicationErrorCode(ErrCodeNoError) {
					handleErr = fmt.Errorf("accepting stream failed: %w", err)
				}
				break
			}
			// server (not gracefully) closed, close the connection immediately
			if s.closeCtx.Err() != nil {
				hconn.CloseWithError(quic.ApplicationErrorCode(ErrCodeNoError), "")
				handleErr = http.ErrServerClosed
				break
			}
			inGracefulShutdown = s.graceCtx.Err() != nil
			if !inGracefulShutdown {
				var appErr *quic.ApplicationError
				if !errors.As(err, &appErr) || appErr.ErrorCode != quic.ApplicationErrorCode(ErrCodeNoError) {
					handleErr = fmt.Errorf("accepting stream failed: %w", err)
				}
				break
			}

			// gracefully closed, send GOAWAY frame and wait for requests to complete or grace period to end
			// new requests will be rejected and shouldn't be sent
			if qlogger != nil {
				qlogger.RecordEvent(qlog.FrameCreated{
					StreamID: ctrlStr.StreamID(),
					Frame:    qlog.Frame{Frame: qlog.GoAwayFrame{StreamID: nextStreamID}},
				})
			}
			wg.Add(1)
			// Send the GOAWAY frame in a separate Goroutine.
			// Sending might block if the peer didn't grant enough flow control credit.
			// Write is guaranteed to return once the connection is closed.
			go func() {
				defer wg.Done()
				_, _ = ctrlStr.Write((&goAwayFrame{StreamID: nextStreamID}).Append(nil))
			}()
			ctx = s.closeCtx
			continue
		}
		if inGracefulShutdown {
			str.CancelRead(quic.StreamErrorCode(ErrCodeRequestRejected))
			str.CancelWrite(quic.StreamErrorCode(ErrCodeRequestRejected))
			continue
		}

		nextStreamID = str.StreamID() + 4
		wg.Add(1)
		go func() {
			// HandleRequestStream will return once the request has been handled,
			// or the underlying connection is closed.
			defer wg.Done()
			hconn.HandleRequestStream(str)
		}()
	}
	wg.Wait()
	return handleErr
}

func (s *Server) maxHeaderBytes() int {
	if s.MaxHeaderBytes <= 0 {
		return http.DefaultMaxHeaderBytes
	}
	return s.MaxHeaderBytes
}

// Close the server immediately, aborting requests and sending CONNECTION_CLOSE frames to connected clients.
// Close in combination with ListenAndServe() (instead of Serve()) may race if it is called before a UDP socket is established.
// It is the caller's responsibility to close any connection passed to ServeQUICConn.
func (s *Server) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.closed = true
	// server is never used
	if s.closeCtx == nil {
		return nil
	}
	s.closeCancel()

	var err error
	for _, l := range s.listeners {
		if l.createdLocally {
			if cerr := (*l.ln).Close(); cerr != nil && err == nil {
				err = cerr
			}
		}
	}
	if s.connCount.Load() == 0 {
		return err
	}
	// wait for all connections to be closed
	<-s.connHandlingDone
	return err
}

// Shutdown gracefully shuts down the server without interrupting any active connections.
// The server sends a GOAWAY frame first, then or for all running requests to complete.
// Shutdown in combination with ListenAndServe may race if it is called before a UDP socket is established.
// It is recommended to use Serve instead.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mutex.Lock()
	s.closed = true
	// server was never used
	if s.closeCtx == nil {
		s.mutex.Unlock()
		return nil
	}
	s.graceCancel()

	// close all listeners
	var closeErrs []error
	for _, l := range s.listeners {
		if l.createdLocally {
			if err := (*l.ln).Close(); err != nil {
				closeErrs = append(closeErrs, err)
			}
		}
	}
	s.mutex.Unlock()
	if len(closeErrs) > 0 {
		return errors.Join(closeErrs...)
	}

	if s.connCount.Load() == 0 {
		return s.Close()
	}
	select {
	case <-s.connHandlingDone: // all connections were closed
		// When receiving a GOAWAY frame, HTTP/3 clients are expected to close the connection
		// once all requests were successfully handled...
		return s.Close()
	case <-ctx.Done():
		// ... however, clients handling long-lived requests (and misbehaving clients),
		// might not do so before the context is cancelled.
		// In this case, we close the server, which closes all existing connections
		// (expect those passed to ServeQUICConn).
		_ = s.Close()
		return ctx.Err()
	}
}

// ErrNoAltSvcPort is the error returned by SetQUICHeaders when no port was found
// for Alt-Svc to announce. This can happen if listening on a PacketConn without a port
// (UNIX socket, for example) and no port is specified in Server.Port or Server.Addr.
var ErrNoAltSvcPort = errors.New("no port can be announced, specify it explicitly using Server.Port or Server.Addr")

// SetQUICHeaders can be used to set the proper headers that announce that this server supports HTTP/3.
// The values set by default advertise all the ports the server is listening on, but can be
// changed to a specific port by setting Server.Port before launching the server.
// If no listener's Addr().String() returns an address with a valid port, Server.Addr will be used
// to extract the port, if specified.
// For example, a server launched using ListenAndServe on an address with port 443 would set:
//
//	Alt-Svc: h3=":443"; ma=2592000
func (s *Server) SetQUICHeaders(hdr http.Header) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.altSvcHeader == "" {
		return ErrNoAltSvcPort
	}
	// use the map directly to avoid constant canonicalization since the key is already canonicalized
	hdr["Alt-Svc"] = append(hdr["Alt-Svc"], s.altSvcHeader)
	return nil
}

// ListenAndServeQUIC listens on the UDP network address addr and calls the
// handler for HTTP/3 requests on incoming connections. http.DefaultServeMux is
// used when handler is nil.
func ListenAndServeQUIC(addr, certFile, keyFile string, handler http.Handler) error {
	server := &Server{
		Addr:    addr,
		Handler: handler,
	}
	return server.ListenAndServeTLS(certFile, keyFile)
}

// ListenAndServeTLS listens on the given network address for both TLS/TCP and QUIC
// connections in parallel. It returns if one of the two returns an error.
// http.DefaultServeMux is used when handler is nil.
// The correct Alt-Svc headers for QUIC are set.
func ListenAndServeTLS(addr, certFile, keyFile string, handler http.Handler) error {
	// Load certs
	var err error
	certs := make([]tls.Certificate, 1)
	certs[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}
	// We currently only use the cert-related stuff from tls.Config,
	// so we don't need to make a full copy.
	config := &tls.Config{
		Certificates: certs,
	}

	if addr == "" {
		addr = ":https"
	}

	// Open the listeners
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	defer udpConn.Close()

	if handler == nil {
		handler = http.DefaultServeMux
	}
	// Start the servers
	quicServer := &Server{
		TLSConfig: config,
		Handler:   handler,
	}

	hErr := make(chan error, 1)
	qErr := make(chan error, 1)
	go func() {
		hErr <- http.ListenAndServeTLS(addr, certFile, keyFile, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			quicServer.SetQUICHeaders(w.Header())
			handler.ServeHTTP(w, r)
		}))
	}()
	go func() {
		qErr <- quicServer.Serve(udpConn)
	}()

	select {
	case err := <-hErr:
		quicServer.Close()
		return err
	case err := <-qErr:
		// Cannot close the HTTP server or wait for requests to complete properly :/
		return err
	}
}
