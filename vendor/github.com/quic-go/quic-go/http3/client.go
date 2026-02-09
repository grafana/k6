package http3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptrace"
	"net/textproto"
	"sync"
	"time"

	"github.com/quic-go/qpack"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3/qlog"
	"github.com/quic-go/quic-go/qlogwriter"
)

const (
	// MethodGet0RTT allows a GET request to be sent using 0-RTT.
	// Note that 0-RTT doesn't provide replay protection and should only be used for idempotent requests.
	MethodGet0RTT = "GET_0RTT"
	// MethodHead0RTT allows a HEAD request to be sent using 0-RTT.
	// Note that 0-RTT doesn't provide replay protection and should only be used for idempotent requests.
	MethodHead0RTT = "HEAD_0RTT"
)

const (
	defaultUserAgent              = "quic-go HTTP/3"
	defaultMaxResponseHeaderBytes = 10 * 1 << 20 // 10 MB
)

var errGoAway = errors.New("connection in graceful shutdown")

type errConnUnusable struct{ e error }

func (e *errConnUnusable) Unwrap() error { return e.e }
func (e *errConnUnusable) Error() string { return fmt.Sprintf("http3: conn unusable: %s", e.e.Error()) }

const max1xxResponses = 5 // arbitrary bound on number of informational responses

var defaultQuicConfig = &quic.Config{
	MaxIncomingStreams: -1, // don't allow the server to create bidirectional streams
	KeepAlivePeriod:    10 * time.Second,
}

// ClientConn is an HTTP/3 client doing requests to a single remote server.
type ClientConn struct {
	conn    *quic.Conn
	rawConn *rawConn

	decoder *qpack.Decoder

	// Additional HTTP/3 settings.
	// It is invalid to specify any settings defined by RFC 9114 (HTTP/3) and RFC 9297 (HTTP Datagrams).
	additionalSettings map[uint64]uint64

	// maxResponseHeaderBytes specifies a limit on how many response bytes are
	// allowed in the server's response header.
	maxResponseHeaderBytes int

	// disableCompression, if true, prevents the Transport from requesting compression with an
	// "Accept-Encoding: gzip" request header when the Request contains no existing Accept-Encoding value.
	// If the Transport requests gzip on its own and gets a gzipped response, it's transparently
	// decoded in the Response.Body.
	// However, if the user explicitly requested gzip it is not automatically uncompressed.
	disableCompression bool

	streamMx     sync.Mutex
	maxStreamID  quic.StreamID // set once a GOAWAY frame is received
	lastStreamID quic.StreamID // the highest stream ID that was opened

	qlogger qlogwriter.Recorder
	logger  *slog.Logger

	requestWriter *requestWriter
}

var _ http.RoundTripper = &ClientConn{}

func newClientConn(
	conn *quic.Conn,
	enableDatagrams bool,
	additionalSettings map[uint64]uint64,
	maxResponseHeaderBytes int,
	disableCompression bool,
	logger *slog.Logger,
) *ClientConn {
	var qlogger qlogwriter.Recorder
	if qlogTrace := conn.QlogTrace(); qlogTrace != nil && qlogTrace.SupportsSchemas(qlog.EventSchema) {
		qlogger = qlogTrace.AddProducer()
	}
	c := &ClientConn{
		conn:               conn,
		additionalSettings: additionalSettings,
		disableCompression: disableCompression,
		maxStreamID:        invalidStreamID,
		lastStreamID:       invalidStreamID,
		logger:             logger,
		qlogger:            qlogger,
		decoder:            qpack.NewDecoder(),
	}
	if maxResponseHeaderBytes <= 0 {
		c.maxResponseHeaderBytes = defaultMaxResponseHeaderBytes
	} else {
		c.maxResponseHeaderBytes = maxResponseHeaderBytes
	}
	c.requestWriter = newRequestWriter()
	c.rawConn = newRawConn(
		conn,
		enableDatagrams,
		c.onStreamsEmpty,
		c.handleControlStream,
		qlogger,
		c.logger,
	)
	// send the SETTINGs frame, using 0-RTT data, if possible
	go func() {
		_, err := c.rawConn.openControlStream(&settingsFrame{
			Datagram:            enableDatagrams,
			Other:               additionalSettings,
			MaxFieldSectionSize: int64(c.maxResponseHeaderBytes),
		})
		if err != nil {
			if c.logger != nil {
				c.logger.Debug("setting up connection failed", "error", err)
			}
			c.conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeInternalError), "")
			return
		}
	}()
	return c
}

// OpenRequestStream opens a new request stream on the HTTP/3 connection.
func (c *ClientConn) OpenRequestStream(ctx context.Context) (*RequestStream, error) {
	return c.openRequestStream(ctx, c.requestWriter, nil, c.disableCompression, c.maxResponseHeaderBytes)
}

func (c *ClientConn) openRequestStream(
	ctx context.Context,
	requestWriter *requestWriter,
	reqDone chan<- struct{},
	disableCompression bool,
	maxHeaderBytes int,
) (*RequestStream, error) {
	c.streamMx.Lock()
	maxStreamID := c.maxStreamID
	var nextStreamID quic.StreamID
	if c.lastStreamID == invalidStreamID {
		nextStreamID = 0
	} else {
		nextStreamID = c.lastStreamID + 4
	}
	c.streamMx.Unlock()
	// Streams with stream ID equal to or greater than the stream ID carried in the GOAWAY frame
	// will be rejected, see section 5.2 of RFC 9114.
	if maxStreamID != invalidStreamID && nextStreamID >= maxStreamID {
		return nil, errGoAway
	}

	str, err := c.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}

	c.streamMx.Lock()
	// take the maximum here, as multiple OpenStreamSync calls might have returned concurrently
	if c.lastStreamID == invalidStreamID {
		c.lastStreamID = str.StreamID()
	} else {
		c.lastStreamID = max(c.lastStreamID, str.StreamID())
	}
	// check again, in case a (or another) GOAWAY frame was received
	maxStreamID = c.maxStreamID
	c.streamMx.Unlock()

	if maxStreamID != invalidStreamID && str.StreamID() >= maxStreamID {
		str.CancelRead(quic.StreamErrorCode(ErrCodeRequestCanceled))
		str.CancelWrite(quic.StreamErrorCode(ErrCodeRequestCanceled))
		return nil, errGoAway
	}

	hstr := c.rawConn.TrackStream(str)
	rsp := &http.Response{}
	trace := httptrace.ContextClientTrace(ctx)
	return newRequestStream(
		newStream(hstr, c.rawConn, trace, func(r io.Reader, hf *headersFrame) error {
			hdr, err := decodeTrailers(r, hf, maxHeaderBytes, c.decoder, c.qlogger, str.StreamID())
			if err != nil {
				return err
			}
			rsp.Trailer = hdr
			return nil
		}, c.qlogger),
		requestWriter,
		reqDone,
		c.decoder,
		disableCompression,
		maxHeaderBytes,
		rsp,
	), nil
}

func (c *ClientConn) handleUnidirectionalStream(str *quic.ReceiveStream) {
	c.rawConn.handleUnidirectionalStream(str, false)
}

func (c *ClientConn) handleControlStream(str *quic.ReceiveStream, fp *frameParser) {
	for {
		f, err := fp.ParseNext(c.qlogger)
		if err != nil {
			var serr *quic.StreamError
			if err == io.EOF || errors.As(err, &serr) {
				c.conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeClosedCriticalStream), "")
				return
			}
			c.conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeFrameError), "")
			return
		}
		// GOAWAY is the only frame allowed at this point:
		// * unexpected frames are ignored by the frame parser
		// * we don't support any extension that might add support for more frames
		goaway, ok := f.(*goAwayFrame)
		if !ok {
			c.conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeFrameUnexpected), "")
			return
		}
		if goaway.StreamID%4 != 0 { // client-initiated, bidirectional streams
			c.conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeIDError), "")
			return
		}
		c.streamMx.Lock()
		// the server is not allowed to increase the Stream ID in subsequent GOAWAY frames
		if c.maxStreamID != invalidStreamID && goaway.StreamID > c.maxStreamID {
			c.streamMx.Unlock()
			c.conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeIDError), "")
			return
		}
		c.maxStreamID = goaway.StreamID
		c.streamMx.Unlock()

		hasActiveStreams := c.rawConn.hasActiveStreams()
		// immediately close the connection if there are currently no active requests
		if !hasActiveStreams {
			c.CloseWithError(quic.ApplicationErrorCode(ErrCodeNoError), "")
			return
		}
	}
}

func (c *ClientConn) onStreamsEmpty() {
	c.streamMx.Lock()
	defer c.streamMx.Unlock()

	// The server is performing a graceful shutdown.
	if c.maxStreamID != invalidStreamID {
		c.conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeNoError), "")
	}
}

// RoundTrip executes a request and returns a response
func (c *ClientConn) RoundTrip(req *http.Request) (*http.Response, error) {
	rsp, err := c.roundTrip(req)
	if err != nil && req.Context().Err() != nil {
		// if the context was canceled, return the context cancellation error
		err = req.Context().Err()
	}
	return rsp, err
}

func (c *ClientConn) roundTrip(req *http.Request) (*http.Response, error) {
	// Immediately send out this request, if this is a 0-RTT request.
	switch req.Method {
	case MethodGet0RTT:
		// don't modify the original request
		reqCopy := *req
		req = &reqCopy
		req.Method = http.MethodGet
	case MethodHead0RTT:
		// don't modify the original request
		reqCopy := *req
		req = &reqCopy
		req.Method = http.MethodHead
	default:
		// wait for the handshake to complete
		select {
		case <-c.conn.HandshakeComplete():
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}

	// It is only possible to send an Extended CONNECT request once the SETTINGS were received.
	// See section 3 of RFC 8441.
	if isExtendedConnectRequest(req) {
		connCtx := c.conn.Context()
		// wait for the server's SETTINGS frame to arrive
		select {
		case <-c.rawConn.ReceivedSettings():
		case <-connCtx.Done():
			return nil, context.Cause(connCtx)
		}
		if !c.rawConn.Settings().EnableExtendedConnect {
			return nil, errors.New("http3: server didn't enable Extended CONNECT")
		}
	}

	reqDone := make(chan struct{})
	str, err := c.openRequestStream(
		req.Context(),
		c.requestWriter,
		reqDone,
		c.disableCompression,
		c.maxResponseHeaderBytes,
	)
	if err != nil {
		return nil, &errConnUnusable{e: err}
	}

	// Request Cancellation:
	// This go routine keeps running even after RoundTripOpt() returns.
	// It is shut down when the application is done processing the body.
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

	rsp, err := c.doRequest(req, str)
	if err != nil { // if any error occurred
		close(reqDone)
		<-done
		return nil, maybeReplaceError(err)
	}
	return rsp, maybeReplaceError(err)
}

// ReceivedSettings returns a channel that is closed once the server's HTTP/3 settings were received.
// Settings can be obtained from the Settings method after the channel was closed.
func (c *ClientConn) ReceivedSettings() <-chan struct{} {
	return c.rawConn.ReceivedSettings()
}

// Settings returns the HTTP/3 settings for this connection.
// It is only valid to call this function after the channel returned by ReceivedSettings was closed.
func (c *ClientConn) Settings() *Settings {
	return c.rawConn.Settings()
}

// CloseWithError closes the connection with the given error code and message.
// It is invalid to call this function after the connection was closed.
func (c *ClientConn) CloseWithError(code quic.ApplicationErrorCode, msg string) error {
	return c.conn.CloseWithError(code, msg)
}

// Context returns a context that is cancelled when the connection is closed.
func (c *ClientConn) Context() context.Context {
	return c.conn.Context()
}

// cancelingReader reads from the io.Reader.
// It cancels writing on the stream if any error other than io.EOF occurs.
type cancelingReader struct {
	r   io.Reader
	str *RequestStream
}

func (r *cancelingReader) Read(b []byte) (int, error) {
	n, err := r.r.Read(b)
	if err != nil && err != io.EOF {
		r.str.CancelWrite(quic.StreamErrorCode(ErrCodeRequestCanceled))
	}
	return n, err
}

func (c *ClientConn) sendRequestBody(str *RequestStream, body io.ReadCloser, contentLength int64) error {
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

func (c *ClientConn) doRequest(req *http.Request, str *RequestStream) (*http.Response, error) {
	trace := httptrace.ContextClientTrace(req.Context())
	var sendingReqFailed bool
	if err := str.sendRequestHeader(req); err != nil {
		traceWroteRequest(trace, err)
		if c.logger != nil {
			c.logger.Debug("error writing request", "error", err)
		}
		sendingReqFailed = true
	}
	if !sendingReqFailed {
		if req.Body == nil {
			traceWroteRequest(trace, nil)
			str.Close()
		} else {
			// send the request body asynchronously
			go func() {
				defer str.Close()
				contentLength := int64(-1)
				// According to the documentation for http.Request.ContentLength,
				// a value of 0 with a non-nil Body is also treated as unknown content length.
				if req.ContentLength > 0 {
					contentLength = req.ContentLength
				}
				err := c.sendRequestBody(str, req.Body, contentLength)
				traceWroteRequest(trace, err)
				if err != nil {
					if c.logger != nil {
						c.logger.Debug("error writing request", "error", err)
					}
					return
				}

				if len(req.Trailer) > 0 {
					if err := str.sendRequestTrailer(req); err != nil {
						if c.logger != nil {
							c.logger.Debug("error writing trailers", "error", err)
						}
					}
				}
			}()
		}
	}

	// copy from net/http: support 1xx responses
	var num1xx int // number of informational 1xx headers received
	var res *http.Response
	for {
		var err error
		res, err = str.ReadResponse()
		if err != nil {
			return nil, err
		}
		resCode := res.StatusCode
		is1xx := 100 <= resCode && resCode <= 199
		// treat 101 as a terminal status, see https://github.com/golang/go/issues/26161
		is1xxNonTerminal := is1xx && resCode != http.StatusSwitchingProtocols
		if is1xxNonTerminal {
			num1xx++
			if num1xx > max1xxResponses {
				str.CancelRead(quic.StreamErrorCode(ErrCodeExcessiveLoad))
				str.CancelWrite(quic.StreamErrorCode(ErrCodeExcessiveLoad))
				return nil, errors.New("http3: too many 1xx informational responses")
			}
			traceGot1xxResponse(trace, resCode, textproto.MIMEHeader(res.Header))
			if resCode == http.StatusContinue {
				traceGot100Continue(trace)
			}
			continue
		}
		break
	}
	connState := c.conn.ConnectionState().TLS
	res.TLS = &connState
	res.Request = req
	return res, nil
}

// RawClientConn is a low-level HTTP/3 client connection.
// It allows the application to take control of the stream accept loops,
// giving the application the ability to handle streams originating from the server.
type RawClientConn struct {
	*ClientConn
}

// HandleUnidirectionalStream handles an incoming unidirectional stream.
func (c *RawClientConn) HandleUnidirectionalStream(str *quic.ReceiveStream) {
	c.rawConn.handleUnidirectionalStream(str, false)
}

// HandleBidirectionalStream handles an incoming bidirectional stream.
func (c *ClientConn) HandleBidirectionalStream(str *quic.Stream) {
	// According to RFC 9114, the server is not allowed to open bidirectional streams.
	c.rawConn.CloseWithError(
		quic.ApplicationErrorCode(ErrCodeStreamCreationError),
		fmt.Sprintf("server opened bidirectional stream %d", str.StreamID()),
	)
}
