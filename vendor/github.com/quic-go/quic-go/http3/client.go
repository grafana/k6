package http3

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/quicvarint"

	"github.com/quic-go/qpack"
)

// MethodGet0RTT allows a GET request to be sent using 0-RTT.
// Note that 0-RTT data doesn't provide replay protection.
const MethodGet0RTT = "GET_0RTT"

const (
	defaultUserAgent              = "quic-go HTTP/3"
	defaultMaxResponseHeaderBytes = 10 * 1 << 20 // 10 MB
)

var defaultQuicConfig = &quic.Config{
	MaxIncomingStreams: -1, // don't allow the server to create bidirectional streams
	KeepAlivePeriod:    10 * time.Second,
}

type dialFunc func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlyConnection, error)

var dialAddr dialFunc = quic.DialAddrEarly

type roundTripperOpts struct {
	DisableCompression bool
	EnableDatagram     bool
	MaxHeaderBytes     int64
	AdditionalSettings map[uint64]uint64
	StreamHijacker     func(FrameType, quic.Connection, quic.Stream, error) (hijacked bool, err error)
	UniStreamHijacker  func(StreamType, quic.Connection, quic.ReceiveStream, error) (hijacked bool)
}

// client is a HTTP3 client doing requests
type client struct {
	tlsConf *tls.Config
	config  *quic.Config
	opts    *roundTripperOpts

	dialOnce     sync.Once
	dialer       dialFunc
	handshakeErr error

	requestWriter *requestWriter

	decoder *qpack.Decoder

	hostname string
	conn     atomic.Pointer[quic.EarlyConnection]

	logger utils.Logger
}

var _ roundTripCloser = &client{}

func newClient(hostname string, tlsConf *tls.Config, opts *roundTripperOpts, conf *quic.Config, dialer dialFunc) (roundTripCloser, error) {
	if conf == nil {
		conf = defaultQuicConfig.Clone()
	}
	if len(conf.Versions) == 0 {
		conf = conf.Clone()
		conf.Versions = []quic.VersionNumber{protocol.SupportedVersions[0]}
	}
	if len(conf.Versions) != 1 {
		return nil, errors.New("can only use a single QUIC version for dialing a HTTP/3 connection")
	}
	if conf.MaxIncomingStreams == 0 {
		conf.MaxIncomingStreams = -1 // don't allow any bidirectional streams
	}
	conf.EnableDatagrams = opts.EnableDatagram
	logger := utils.DefaultLogger.WithPrefix("h3 client")

	if tlsConf == nil {
		tlsConf = &tls.Config{}
	} else {
		tlsConf = tlsConf.Clone()
	}
	if tlsConf.ServerName == "" {
		sni, _, err := net.SplitHostPort(hostname)
		if err != nil {
			// It's ok if net.SplitHostPort returns an error - it could be a hostname/IP address without a port.
			sni = hostname
		}
		tlsConf.ServerName = sni
	}
	// Replace existing ALPNs by H3
	tlsConf.NextProtos = []string{versionToALPN(conf.Versions[0])}

	return &client{
		hostname:      authorityAddr("https", hostname),
		tlsConf:       tlsConf,
		requestWriter: newRequestWriter(logger),
		decoder:       qpack.NewDecoder(func(hf qpack.HeaderField) {}),
		config:        conf,
		opts:          opts,
		dialer:        dialer,
		logger:        logger,
	}, nil
}

func (c *client) dial(ctx context.Context) error {
	var err error
	var conn quic.EarlyConnection
	if c.dialer != nil {
		conn, err = c.dialer(ctx, c.hostname, c.tlsConf, c.config)
	} else {
		conn, err = dialAddr(ctx, c.hostname, c.tlsConf, c.config)
	}
	if err != nil {
		return err
	}
	c.conn.Store(&conn)

	// send the SETTINGs frame, using 0-RTT data, if possible
	go func() {
		if err := c.setupConn(conn); err != nil {
			c.logger.Debugf("Setting up connection failed: %s", err)
			conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeInternalError), "")
		}
	}()

	if c.opts.StreamHijacker != nil {
		go c.handleBidirectionalStreams(conn)
	}
	go c.handleUnidirectionalStreams(conn)
	return nil
}

func (c *client) setupConn(conn quic.EarlyConnection) error {
	// open the control stream
	str, err := conn.OpenUniStream()
	if err != nil {
		return err
	}
	b := make([]byte, 0, 64)
	b = quicvarint.Append(b, streamTypeControlStream)
	// send the SETTINGS frame
	b = (&settingsFrame{Datagram: c.opts.EnableDatagram, Other: c.opts.AdditionalSettings}).Append(b)
	_, err = str.Write(b)
	return err
}

func (c *client) handleBidirectionalStreams(conn quic.EarlyConnection) {
	for {
		str, err := conn.AcceptStream(context.Background())
		if err != nil {
			c.logger.Debugf("accepting bidirectional stream failed: %s", err)
			return
		}
		go func(str quic.Stream) {
			_, err := parseNextFrame(str, func(ft FrameType, e error) (processed bool, err error) {
				return c.opts.StreamHijacker(ft, conn, str, e)
			})
			if err == errHijacked {
				return
			}
			if err != nil {
				c.logger.Debugf("error handling stream: %s", err)
			}
			conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeFrameUnexpected), "received HTTP/3 frame on bidirectional stream")
		}(str)
	}
}

func (c *client) handleUnidirectionalStreams(conn quic.EarlyConnection) {
	for {
		str, err := conn.AcceptUniStream(context.Background())
		if err != nil {
			c.logger.Debugf("accepting unidirectional stream failed: %s", err)
			return
		}

		go func(str quic.ReceiveStream) {
			streamType, err := quicvarint.Read(quicvarint.NewReader(str))
			if err != nil {
				if c.opts.UniStreamHijacker != nil && c.opts.UniStreamHijacker(StreamType(streamType), conn, str, err) {
					return
				}
				c.logger.Debugf("reading stream type on stream %d failed: %s", str.StreamID(), err)
				return
			}
			// We're only interested in the control stream here.
			switch streamType {
			case streamTypeControlStream:
			case streamTypeQPACKEncoderStream, streamTypeQPACKDecoderStream:
				// Our QPACK implementation doesn't use the dynamic table yet.
				// TODO: check that only one stream of each type is opened.
				return
			case streamTypePushStream:
				// We never increased the Push ID, so we don't expect any push streams.
				conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeIDError), "")
				return
			default:
				if c.opts.UniStreamHijacker != nil && c.opts.UniStreamHijacker(StreamType(streamType), conn, str, nil) {
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
			// If datagram support was enabled on our side as well as on the server side,
			// we can expect it to have been negotiated both on the transport and on the HTTP/3 layer.
			// Note: ConnectionState() will block until the handshake is complete (relevant when using 0-RTT).
			if c.opts.EnableDatagram && !conn.ConnectionState().SupportsDatagrams {
				conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeSettingsError), "missing QUIC Datagram support")
			}
		}(str)
	}
}

func (c *client) Close() error {
	conn := c.conn.Load()
	if conn == nil {
		return nil
	}
	return (*conn).CloseWithError(quic.ApplicationErrorCode(ErrCodeNoError), "")
}

func (c *client) maxHeaderBytes() uint64 {
	if c.opts.MaxHeaderBytes <= 0 {
		return defaultMaxResponseHeaderBytes
	}
	return uint64(c.opts.MaxHeaderBytes)
}

// RoundTripOpt executes a request and returns a response
func (c *client) RoundTripOpt(req *http.Request, opt RoundTripOpt) (*http.Response, error) {
	if authorityAddr("https", hostnameFromRequest(req)) != c.hostname {
		return nil, fmt.Errorf("http3 client BUG: RoundTripOpt called for the wrong client (expected %s, got %s)", c.hostname, req.Host)
	}

	c.dialOnce.Do(func() {
		c.handshakeErr = c.dial(req.Context())
	})
	if c.handshakeErr != nil {
		return nil, c.handshakeErr
	}

	// At this point, c.conn is guaranteed to be set.
	conn := *c.conn.Load()

	// Immediately send out this request, if this is a 0-RTT request.
	if req.Method == MethodGet0RTT {
		req.Method = http.MethodGet
	} else {
		// wait for the handshake to complete
		select {
		case <-conn.HandshakeComplete():
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}

	str, err := conn.OpenStreamSync(req.Context())
	if err != nil {
		return nil, err
	}

	// Request Cancellation:
	// This go routine keeps running even after RoundTripOpt() returns.
	// It is shut down when the application is done processing the body.
	reqDone := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-req.Context().Done():
			str.CancelWrite(quic.StreamErrorCode(ErrCodeRequestCanceled))
			str.CancelRead(quic.StreamErrorCode(ErrCodeRequestCanceled))
		case <-reqDone:
		}
	}()

	doneChan := reqDone
	if opt.DontCloseRequestStream {
		doneChan = nil
	}
	rsp, rerr := c.doRequest(req, conn, str, opt, doneChan)
	if rerr.err != nil { // if any error occurred
		close(reqDone)
		<-done
		if rerr.streamErr != 0 { // if it was a stream error
			str.CancelWrite(quic.StreamErrorCode(rerr.streamErr))
		}
		if rerr.connErr != 0 { // if it was a connection error
			var reason string
			if rerr.err != nil {
				reason = rerr.err.Error()
			}
			conn.CloseWithError(quic.ApplicationErrorCode(rerr.connErr), reason)
		}
		return nil, maybeReplaceError(rerr.err)
	}
	if opt.DontCloseRequestStream {
		close(reqDone)
		<-done
	}
	return rsp, maybeReplaceError(rerr.err)
}

// cancelingReader reads from the io.Reader.
// It cancels writing on the stream if any error other than io.EOF occurs.
type cancelingReader struct {
	r   io.Reader
	str Stream
}

func (r *cancelingReader) Read(b []byte) (int, error) {
	n, err := r.r.Read(b)
	if err != nil && err != io.EOF {
		r.str.CancelWrite(quic.StreamErrorCode(ErrCodeRequestCanceled))
	}
	return n, err
}

func (c *client) sendRequestBody(str Stream, body io.ReadCloser, contentLength int64) error {
	defer body.Close()
	buf := make([]byte, bodyCopyBufferSize)
	sr := &cancelingReader{str: str, r: body}
	if contentLength == -1 {
		_, err := io.CopyBuffer(str, sr, buf)
		return err
	}

	// make sure we don't send more bytes than the content length
	n, err := io.CopyBuffer(str, io.LimitReader(sr, contentLength), buf)
	if err != nil {
		return err
	}
	var extra int64
	extra, err = io.CopyBuffer(io.Discard, sr, buf)
	n += extra
	if n > contentLength {
		str.CancelWrite(quic.StreamErrorCode(ErrCodeRequestCanceled))
		return fmt.Errorf("http: ContentLength=%d with Body length %d", contentLength, n)
	}
	return err
}

func (c *client) doRequest(req *http.Request, conn quic.EarlyConnection, str quic.Stream, opt RoundTripOpt, reqDone chan<- struct{}) (*http.Response, requestError) {
	var requestGzip bool
	if !c.opts.DisableCompression && req.Method != "HEAD" && req.Header.Get("Accept-Encoding") == "" && req.Header.Get("Range") == "" {
		requestGzip = true
	}
	if err := c.requestWriter.WriteRequestHeader(str, req, requestGzip); err != nil {
		return nil, newStreamError(ErrCodeInternalError, err)
	}

	if req.Body == nil && !opt.DontCloseRequestStream {
		str.Close()
	}

	hstr := newStream(str, func() { conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeFrameUnexpected), "") })
	if req.Body != nil {
		// send the request body asynchronously
		go func() {
			contentLength := int64(-1)
			// According to the documentation for http.Request.ContentLength,
			// a value of 0 with a non-nil Body is also treated as unknown content length.
			if req.ContentLength > 0 {
				contentLength = req.ContentLength
			}
			if err := c.sendRequestBody(hstr, req.Body, contentLength); err != nil {
				c.logger.Errorf("Error writing request: %s", err)
			}
			if !opt.DontCloseRequestStream {
				hstr.Close()
			}
		}()
	}

	frame, err := parseNextFrame(str, nil)
	if err != nil {
		return nil, newStreamError(ErrCodeFrameError, err)
	}
	hf, ok := frame.(*headersFrame)
	if !ok {
		return nil, newConnError(ErrCodeFrameUnexpected, errors.New("expected first frame to be a HEADERS frame"))
	}
	if hf.Length > c.maxHeaderBytes() {
		return nil, newStreamError(ErrCodeFrameError, fmt.Errorf("HEADERS frame too large: %d bytes (max: %d)", hf.Length, c.maxHeaderBytes()))
	}
	headerBlock := make([]byte, hf.Length)
	if _, err := io.ReadFull(str, headerBlock); err != nil {
		return nil, newStreamError(ErrCodeRequestIncomplete, err)
	}
	hfs, err := c.decoder.DecodeFull(headerBlock)
	if err != nil {
		// TODO: use the right error code
		return nil, newConnError(ErrCodeGeneralProtocolError, err)
	}

	res, err := responseFromHeaders(hfs)
	if err != nil {
		return nil, newStreamError(ErrCodeMessageError, err)
	}
	connState := conn.ConnectionState().TLS
	res.TLS = &connState
	res.Request = req
	// Check that the server doesn't send more data in DATA frames than indicated by the Content-Length header (if set).
	// See section 4.1.2 of RFC 9114.
	var httpStr Stream
	if _, ok := res.Header["Content-Length"]; ok && res.ContentLength >= 0 {
		httpStr = newLengthLimitedStream(hstr, res.ContentLength)
	} else {
		httpStr = hstr
	}
	respBody := newResponseBody(httpStr, conn, reqDone)

	// Rules for when to set Content-Length are defined in https://tools.ietf.org/html/rfc7230#section-3.3.2.
	_, hasTransferEncoding := res.Header["Transfer-Encoding"]
	isInformational := res.StatusCode >= 100 && res.StatusCode < 200
	isNoContent := res.StatusCode == http.StatusNoContent
	isSuccessfulConnect := req.Method == http.MethodConnect && res.StatusCode >= 200 && res.StatusCode < 300
	if !hasTransferEncoding && !isInformational && !isNoContent && !isSuccessfulConnect {
		res.ContentLength = -1
		if clens, ok := res.Header["Content-Length"]; ok && len(clens) == 1 {
			if clen64, err := strconv.ParseInt(clens[0], 10, 64); err == nil {
				res.ContentLength = clen64
			}
		}
	}

	if requestGzip && res.Header.Get("Content-Encoding") == "gzip" {
		res.Header.Del("Content-Encoding")
		res.Header.Del("Content-Length")
		res.ContentLength = -1
		res.Body = newGzipReader(respBody)
		res.Uncompressed = true
	} else {
		res.Body = respBody
	}

	return res, requestError{}
}

func (c *client) HandshakeComplete() bool {
	conn := c.conn.Load()
	if conn == nil {
		return false
	}
	select {
	case <-(*conn).HandshakeComplete():
		return true
	default:
		return false
	}
}
