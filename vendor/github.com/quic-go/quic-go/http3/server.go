package http3

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/quicvarint"

	"github.com/quic-go/qpack"
)

// allows mocking of quic.Listen and quic.ListenAddr
var (
	quicListen = func(conn net.PacketConn, tlsConf *tls.Config, config *quic.Config) (QUICEarlyListener, error) {
		return quic.ListenEarly(conn, tlsConf, config)
	}
	quicListenAddr = func(addr string, tlsConf *tls.Config, config *quic.Config) (QUICEarlyListener, error) {
		return quic.ListenAddrEarly(addr, tlsConf, config)
	}
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

// A QUICEarlyListener listens for incoming QUIC connections.
type QUICEarlyListener interface {
	Accept(context.Context) (quic.EarlyConnection, error)
	Addr() net.Addr
	io.Closer
}

var _ QUICEarlyListener = &quic.EarlyListener{}

func versionToALPN(v protocol.VersionNumber) string {
	//nolint:exhaustive // These are all the versions we care about.
	switch v {
	case protocol.Version1, protocol.Version2:
		return NextProtoH3
	default:
		return ""
	}
}

// ConfigureTLSConfig creates a new tls.Config which can be used
// to create a quic.Listener meant for serving http3. The created
// tls.Config adds the functionality of detecting the used QUIC version
// in order to set the correct ALPN value for the http3 connection.
func ConfigureTLSConfig(tlsConf *tls.Config) *tls.Config {
	// The tls.Config used to setup the quic.Listener needs to have the GetConfigForClient callback set.
	// That way, we can get the QUIC version and set the correct ALPN value.
	return &tls.Config{
		GetConfigForClient: func(ch *tls.ClientHelloInfo) (*tls.Config, error) {
			// determine the ALPN from the QUIC version used
			proto := NextProtoH3
			val := ch.Context().Value(quic.QUICVersionContextKey)
			if v, ok := val.(quic.VersionNumber); ok {
				proto = versionToALPN(v)
			}
			config := tlsConf
			if tlsConf.GetConfigForClient != nil {
				getConfigForClient := tlsConf.GetConfigForClient
				var err error
				conf, err := getConfigForClient(ch)
				if err != nil {
					return nil, err
				}
				if conf != nil {
					config = conf
				}
			}
			if config == nil {
				return nil, nil
			}
			config = config.Clone()
			config.NextProtos = []string{proto}
			return config, nil
		},
	}
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

type requestError struct {
	err       error
	streamErr ErrCode
	connErr   ErrCode
}

func newStreamError(code ErrCode, err error) requestError {
	return requestError{err: err, streamErr: code}
}

func newConnError(code ErrCode, err error) requestError {
	return requestError{err: err, connErr: code}
}

// listenerInfo contains info about specific listener added with addListener
type listenerInfo struct {
	port int // 0 means that no info about port is available
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
	// with SetQuicHeaders.
	Addr string

	// Port is used in Alt-Svc response headers set with SetQuicHeaders. If
	// needed Port can be manually set when the Server is created.
	//
	// This is useful when a Layer 4 firewall is redirecting UDP traffic and
	// clients must use a port different from the port the Server is
	// listening on.
	Port int

	// TLSConfig provides a TLS configuration for use by server. It must be
	// set for ListenAndServe and Serve methods.
	TLSConfig *tls.Config

	// QuicConfig provides the parameters for QUIC connection created with
	// Serve. If nil, it uses reasonable default values.
	//
	// Configured versions are also used in Alt-Svc response header set with
	// SetQuicHeaders.
	QuicConfig *quic.Config

	// Handler is the HTTP request handler to use. If not set, defaults to
	// http.NotFound.
	Handler http.Handler

	// EnableDatagrams enables support for HTTP/3 datagrams.
	// If set to true, QuicConfig.EnableDatagram will be set.
	// See https://datatracker.ietf.org/doc/html/rfc9297.
	EnableDatagrams bool

	// MaxHeaderBytes controls the maximum number of bytes the server will
	// read parsing the request HEADERS frame. It does not limit the size of
	// the request body. If zero or negative, http.DefaultMaxHeaderBytes is
	// used.
	MaxHeaderBytes int

	// AdditionalSettings specifies additional HTTP/3 settings.
	// It is invalid to specify any settings defined by the HTTP/3 draft and the datagram draft.
	AdditionalSettings map[uint64]uint64

	// StreamHijacker, when set, is called for the first unknown frame parsed on a bidirectional stream.
	// It is called right after parsing the frame type.
	// If parsing the frame type fails, the error is passed to the callback.
	// In that case, the frame type will not be set.
	// Callers can either ignore the frame and return control of the stream back to HTTP/3
	// (by returning hijacked false).
	// Alternatively, callers can take over the QUIC stream (by returning hijacked true).
	StreamHijacker func(FrameType, quic.Connection, quic.Stream, error) (hijacked bool, err error)

	// UniStreamHijacker, when set, is called for unknown unidirectional stream of unknown stream type.
	// If parsing the stream type fails, the error is passed to the callback.
	// In that case, the stream type will not be set.
	UniStreamHijacker func(StreamType, quic.Connection, quic.ReceiveStream, error) (hijacked bool)

	mutex     sync.RWMutex
	listeners map[*QUICEarlyListener]listenerInfo

	closed bool

	altSvcHeader string

	logger utils.Logger
}

// ListenAndServe listens on the UDP address s.Addr and calls s.Handler to handle HTTP/3 requests on incoming connections.
//
// If s.Addr is blank, ":https" is used.
func (s *Server) ListenAndServe() error {
	return s.serveConn(s.TLSConfig, nil)
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
	config := &tls.Config{
		Certificates: certs,
	}
	return s.serveConn(config, nil)
}

// Serve an existing UDP connection.
// It is possible to reuse the same connection for outgoing connections.
// Closing the server does not close the connection.
func (s *Server) Serve(conn net.PacketConn) error {
	return s.serveConn(s.TLSConfig, conn)
}

// ServeQUICConn serves a single QUIC connection.
func (s *Server) ServeQUICConn(conn quic.Connection) error {
	s.mutex.Lock()
	if s.logger == nil {
		s.logger = utils.DefaultLogger.WithPrefix("server")
	}
	s.mutex.Unlock()

	return s.handleConn(conn)
}

// ServeListener serves an existing QUIC listener.
// Make sure you use http3.ConfigureTLSConfig to configure a tls.Config
// and use it to construct a http3-friendly QUIC listener.
// Closing the server does close the listener.
// ServeListener always returns a non-nil error. After Shutdown or Close, the returned error is http.ErrServerClosed.
func (s *Server) ServeListener(ln QUICEarlyListener) error {
	if err := s.addListener(&ln); err != nil {
		return err
	}
	defer s.removeListener(&ln)
	for {
		conn, err := ln.Accept(context.Background())
		if err == quic.ErrServerClosed {
			return http.ErrServerClosed
		}
		if err != nil {
			return err
		}
		go func() {
			if err := s.handleConn(conn); err != nil {
				s.logger.Debugf(err.Error())
			}
		}()
	}
}

var errServerWithoutTLSConfig = errors.New("use of http3.Server without TLSConfig")

func (s *Server) serveConn(tlsConf *tls.Config, conn net.PacketConn) error {
	if tlsConf == nil {
		return errServerWithoutTLSConfig
	}

	s.mutex.Lock()
	closed := s.closed
	s.mutex.Unlock()
	if closed {
		return http.ErrServerClosed
	}

	baseConf := ConfigureTLSConfig(tlsConf)
	quicConf := s.QuicConfig
	if quicConf == nil {
		quicConf = &quic.Config{Allow0RTT: true}
	} else {
		quicConf = s.QuicConfig.Clone()
	}
	if s.EnableDatagrams {
		quicConf.EnableDatagrams = true
	}

	var ln QUICEarlyListener
	var err error
	if conn == nil {
		addr := s.Addr
		if addr == "" {
			addr = ":https"
		}
		ln, err = quicListenAddr(addr, baseConf, quicConf)
	} else {
		ln, err = quicListen(conn, baseConf, quicConf)
	}
	if err != nil {
		return err
	}
	return s.ServeListener(ln)
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
	supportedVersions := protocol.SupportedVersions
	if s.QuicConfig != nil && len(s.QuicConfig.Versions) > 0 {
		supportedVersions = s.QuicConfig.Versions
	}

	// keep track of which have been seen so we don't yield duplicate values
	seen := make(map[string]struct{}, len(supportedVersions))
	var versionStrings []string
	for _, version := range supportedVersions {
		if v := versionToALPN(version); len(v) > 0 {
			if _, ok := seen[v]; !ok {
				versionStrings = append(versionStrings, v)
				seen[v] = struct{}{}
			}
		}
	}

	var altSvc []string
	addPort := func(port int) {
		for _, v := range versionStrings {
			altSvc = append(altSvc, fmt.Sprintf(`%s=":%d"; ma=2592000`, v, port))
		}
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

// We store a pointer to interface in the map set. This is safe because we only
// call trackListener via Serve and can track+defer untrack the same pointer to
// local variable there. We never need to compare a Listener from another caller.
func (s *Server) addListener(l *QUICEarlyListener) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.closed {
		return http.ErrServerClosed
	}
	if s.logger == nil {
		s.logger = utils.DefaultLogger.WithPrefix("server")
	}
	if s.listeners == nil {
		s.listeners = make(map[*QUICEarlyListener]listenerInfo)
	}

	if port, err := extractPort((*l).Addr().String()); err == nil {
		s.listeners[l] = listenerInfo{port}
	} else {
		s.logger.Errorf("Unable to extract port from listener %+v, will not be announced using SetQuicHeaders: %s", err)
		s.listeners[l] = listenerInfo{}
	}
	s.generateAltSvcHeader()
	return nil
}

func (s *Server) removeListener(l *QUICEarlyListener) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.listeners, l)
	s.generateAltSvcHeader()
}

func (s *Server) handleConn(conn quic.Connection) error {
	decoder := qpack.NewDecoder(nil)

	// send a SETTINGS frame
	str, err := conn.OpenUniStream()
	if err != nil {
		return fmt.Errorf("opening the control stream failed: %w", err)
	}
	b := make([]byte, 0, 64)
	b = quicvarint.Append(b, streamTypeControlStream) // stream type
	b = (&settingsFrame{Datagram: s.EnableDatagrams, Other: s.AdditionalSettings}).Append(b)
	str.Write(b)

	go s.handleUnidirectionalStreams(conn)

	// Process all requests immediately.
	// It's the client's responsibility to decide which requests are eligible for 0-RTT.
	for {
		str, err := conn.AcceptStream(context.Background())
		if err != nil {
			var appErr *quic.ApplicationError
			if errors.As(err, &appErr) && appErr.ErrorCode == quic.ApplicationErrorCode(ErrCodeNoError) {
				return nil
			}
			return fmt.Errorf("accepting stream failed: %w", err)
		}
		go func() {
			rerr := s.handleRequest(conn, str, decoder, func() {
				conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeFrameUnexpected), "")
			})
			if rerr.err == errHijacked {
				return
			}
			if rerr.err != nil || rerr.streamErr != 0 || rerr.connErr != 0 {
				s.logger.Debugf("Handling request failed: %s", err)
				if rerr.streamErr != 0 {
					str.CancelWrite(quic.StreamErrorCode(rerr.streamErr))
				}
				if rerr.connErr != 0 {
					var reason string
					if rerr.err != nil {
						reason = rerr.err.Error()
					}
					conn.CloseWithError(quic.ApplicationErrorCode(rerr.connErr), reason)
				}
				return
			}
			str.Close()
		}()
	}
}

func (s *Server) handleUnidirectionalStreams(conn quic.Connection) {
	for {
		str, err := conn.AcceptUniStream(context.Background())
		if err != nil {
			s.logger.Debugf("accepting unidirectional stream failed: %s", err)
			return
		}

		go func(str quic.ReceiveStream) {
			streamType, err := quicvarint.Read(quicvarint.NewReader(str))
			if err != nil {
				if s.UniStreamHijacker != nil && s.UniStreamHijacker(StreamType(streamType), conn, str, err) {
					return
				}
				s.logger.Debugf("reading stream type on stream %d failed: %s", str.StreamID(), err)
				return
			}
			// We're only interested in the control stream here.
			switch streamType {
			case streamTypeControlStream:
			case streamTypeQPACKEncoderStream, streamTypeQPACKDecoderStream:
				// Our QPACK implementation doesn't use the dynamic table yet.
				// TODO: check that only one stream of each type is opened.
				return
			case streamTypePushStream: // only the server can push
				conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeStreamCreationError), "")
				return
			default:
				if s.UniStreamHijacker != nil && s.UniStreamHijacker(StreamType(streamType), conn, str, nil) {
					return
				}
				str.CancelRead(quic.StreamErrorCode(ErrCodeStreamCreationError))
				return
			}
			f, err := parseNextFrame(str, nil)
			if err != nil {
				conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeFrameError), "")
				return
			}
			sf, ok := f.(*settingsFrame)
			if !ok {
				conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeMissingSettings), "")
				return
			}
			if !sf.Datagram {
				return
			}
			// If datagram support was enabled on our side as well as on the client side,
			// we can expect it to have been negotiated both on the transport and on the HTTP/3 layer.
			// Note: ConnectionState() will block until the handshake is complete (relevant when using 0-RTT).
			if s.EnableDatagrams && !conn.ConnectionState().SupportsDatagrams {
				conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeSettingsError), "missing QUIC Datagram support")
			}
		}(str)
	}
}

func (s *Server) maxHeaderBytes() uint64 {
	if s.MaxHeaderBytes <= 0 {
		return http.DefaultMaxHeaderBytes
	}
	return uint64(s.MaxHeaderBytes)
}

func (s *Server) handleRequest(conn quic.Connection, str quic.Stream, decoder *qpack.Decoder, onFrameError func()) requestError {
	var ufh unknownFrameHandlerFunc
	if s.StreamHijacker != nil {
		ufh = func(ft FrameType, e error) (processed bool, err error) { return s.StreamHijacker(ft, conn, str, e) }
	}
	frame, err := parseNextFrame(str, ufh)
	if err != nil {
		if err == errHijacked {
			return requestError{err: errHijacked}
		}
		return newStreamError(ErrCodeRequestIncomplete, err)
	}
	hf, ok := frame.(*headersFrame)
	if !ok {
		return newConnError(ErrCodeFrameUnexpected, errors.New("expected first frame to be a HEADERS frame"))
	}
	if hf.Length > s.maxHeaderBytes() {
		return newStreamError(ErrCodeFrameError, fmt.Errorf("HEADERS frame too large: %d bytes (max: %d)", hf.Length, s.maxHeaderBytes()))
	}
	headerBlock := make([]byte, hf.Length)
	if _, err := io.ReadFull(str, headerBlock); err != nil {
		return newStreamError(ErrCodeRequestIncomplete, err)
	}
	hfs, err := decoder.DecodeFull(headerBlock)
	if err != nil {
		// TODO: use the right error code
		return newConnError(ErrCodeGeneralProtocolError, err)
	}
	req, err := requestFromHeaders(hfs)
	if err != nil {
		return newStreamError(ErrCodeMessageError, err)
	}

	connState := conn.ConnectionState().TLS
	req.TLS = &connState
	req.RemoteAddr = conn.RemoteAddr().String()

	// Check that the client doesn't send more data in DATA frames than indicated by the Content-Length header (if set).
	// See section 4.1.2 of RFC 9114.
	var httpStr Stream
	if _, ok := req.Header["Content-Length"]; ok && req.ContentLength >= 0 {
		httpStr = newLengthLimitedStream(newStream(str, onFrameError), req.ContentLength)
	} else {
		httpStr = newStream(str, onFrameError)
	}
	body := newRequestBody(httpStr)
	req.Body = body

	if s.logger.Debug() {
		s.logger.Infof("%s %s%s, on stream %d", req.Method, req.Host, req.RequestURI, str.StreamID())
	} else {
		s.logger.Infof("%s %s%s", req.Method, req.Host, req.RequestURI)
	}

	ctx := str.Context()
	ctx = context.WithValue(ctx, ServerContextKey, s)
	ctx = context.WithValue(ctx, http.LocalAddrContextKey, conn.LocalAddr())
	req = req.WithContext(ctx)
	r := newResponseWriter(str, conn, s.logger)
	if req.Method == http.MethodHead {
		r.isHead = true
	}
	handler := s.Handler
	if handler == nil {
		handler = http.DefaultServeMux
	}

	var panicked bool
	func() {
		defer func() {
			if p := recover(); p != nil {
				panicked = true
				if p == http.ErrAbortHandler {
					return
				}
				// Copied from net/http/server.go
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				s.logger.Errorf("http: panic serving: %v\n%s", p, buf)
			}
		}()
		handler.ServeHTTP(r, req)
	}()

	if body.wasStreamHijacked() {
		return requestError{err: errHijacked}
	}

	// only write response when there is no panic
	if !panicked {
		// response not written to the client yet, set Content-Length
		if !r.written {
			if _, haveCL := r.header["Content-Length"]; !haveCL {
				r.header.Set("Content-Length", strconv.FormatInt(r.numWritten, 10))
			}
		}
		r.Flush()
	}
	// If the EOF was read by the handler, CancelRead() is a no-op.
	str.CancelRead(quic.StreamErrorCode(ErrCodeNoError))
	return requestError{}
}

// Close the server immediately, aborting requests and sending CONNECTION_CLOSE frames to connected clients.
// Close in combination with ListenAndServe() (instead of Serve()) may race if it is called before a UDP socket is established.
func (s *Server) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.closed = true

	var err error
	for ln := range s.listeners {
		if cerr := (*ln).Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return err
}

// CloseGracefully shuts down the server gracefully. The server sends a GOAWAY frame first, then waits for either timeout to trigger, or for all running requests to complete.
// CloseGracefully in combination with ListenAndServe() (instead of Serve()) may race if it is called before a UDP socket is established.
func (s *Server) CloseGracefully(timeout time.Duration) error {
	// TODO: implement
	return nil
}

// ErrNoAltSvcPort is the error returned by SetQuicHeaders when no port was found
// for Alt-Svc to announce. This can happen if listening on a PacketConn without a port
// (UNIX socket, for example) and no port is specified in Server.Port or Server.Addr.
var ErrNoAltSvcPort = errors.New("no port can be announced, specify it explicitly using Server.Port or Server.Addr")

// SetQuicHeaders can be used to set the proper headers that announce that this server supports HTTP/3.
// The values set by default advertise all of the ports the server is listening on, but can be
// changed to a specific port by setting Server.Port before launching the serverr.
// If no listener's Addr().String() returns an address with a valid port, Server.Addr will be used
// to extract the port, if specified.
// For example, a server launched using ListenAndServe on an address with port 443 would set:
//
//	Alt-Svc: h3=":443"; ma=2592000,h3-29=":443"; ma=2592000
func (s *Server) SetQuicHeaders(hdr http.Header) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.altSvcHeader == "" {
		return ErrNoAltSvcPort
	}
	// use the map directly to avoid constant canonicalization
	// since the key is already canonicalized
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

// ListenAndServe listens on the given network address for both, TLS and QUIC
// connections in parallel. It returns if one of the two returns an error.
// http.DefaultServeMux is used when handler is nil.
// The correct Alt-Svc headers for QUIC are set.
func ListenAndServe(addr, certFile, keyFile string, handler http.Handler) error {
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

	hErr := make(chan error)
	qErr := make(chan error)
	go func() {
		hErr <- http.ListenAndServeTLS(addr, certFile, keyFile, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			quicServer.SetQuicHeaders(w.Header())
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
