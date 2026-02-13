package http3

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/quic-go/qpack"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/qlogwriter"
)

// RawServerConn is an HTTP/3 server connection.
// It can be used for advanced use cases where the application wants to manage the QUIC connection lifecycle.
type RawServerConn struct {
	rawConn rawConn

	idleTimeout time.Duration
	idleTimer   *time.Timer

	serverContext  context.Context
	requestHandler http.Handler
	maxHeaderBytes int

	decoder *qpack.Decoder

	qlogger qlogwriter.Recorder
	logger  *slog.Logger
}

func newRawServerConn(
	conn *quic.Conn,
	enableDatagrams bool,
	idleTimeout time.Duration,
	qlogger qlogwriter.Recorder,
	logger *slog.Logger,
	serverContext context.Context,
	requestHandler http.Handler,
	maxHeaderBytes int,
) *RawServerConn {
	c := &RawServerConn{
		idleTimeout:    idleTimeout,
		serverContext:  serverContext,
		requestHandler: requestHandler,
		maxHeaderBytes: maxHeaderBytes,
		decoder:        qpack.NewDecoder(),
		qlogger:        qlogger,
		logger:         logger,
	}
	c.rawConn = *newRawConn(conn, enableDatagrams, c.onStreamsEmpty, nil, qlogger, logger)
	if idleTimeout > 0 {
		c.idleTimer = time.AfterFunc(idleTimeout, c.onIdleTimer)
	}
	return c
}

func (c *RawServerConn) onStreamsEmpty() {
	if c.idleTimeout > 0 {
		c.idleTimer.Reset(c.idleTimeout)
	}
}

func (c *RawServerConn) onIdleTimer() {
	c.CloseWithError(quic.ApplicationErrorCode(ErrCodeNoError), "idle timeout")
}

// CloseWithError closes the connection with the given error code and message.
func (c *RawServerConn) CloseWithError(code quic.ApplicationErrorCode, msg string) error {
	if c.idleTimer != nil {
		c.idleTimer.Stop()
	}
	return c.rawConn.CloseWithError(code, msg)
}

// HandleRequestStream handles an HTTP/3 request on a bidirectional request stream.
// The stream can either be obtained by calling AcceptStream on the underlying QUIC connection,
// or (internally) by using the server's stream accept loop.
func (c *RawServerConn) HandleRequestStream(str *quic.Stream) {
	hstr := c.rawConn.TrackStream(str)
	c.handleRequestStream(hstr)
}

func (c *RawServerConn) requestMaxHeaderBytes() int {
	if c.maxHeaderBytes <= 0 {
		return http.DefaultMaxHeaderBytes
	}
	return c.maxHeaderBytes
}

func (c *RawServerConn) openControlStream(settings *settingsFrame) (*quic.SendStream, error) {
	return c.rawConn.openControlStream(settings)
}

func (c *RawServerConn) handleRequestStream(str *stateTrackingStream) {
	if c.idleTimeout > 0 {
		// This only applies if the stream is the first active stream,
		// but it's ok to stop a stopped timer.
		c.idleTimer.Stop()
	}

	conn := &c.rawConn
	qlogger := c.qlogger
	decoder := c.decoder
	connCtx := c.serverContext
	maxHeaderBytes := c.requestMaxHeaderBytes()

	fp := &frameParser{closeConn: conn.CloseWithError, r: str, streamID: str.StreamID()}
	frame, err := fp.ParseNext(qlogger)
	if err != nil {
		str.CancelRead(quic.StreamErrorCode(ErrCodeRequestIncomplete))
		str.CancelWrite(quic.StreamErrorCode(ErrCodeRequestIncomplete))
		return
	}
	hf, ok := frame.(*headersFrame)
	if !ok {
		conn.CloseWithError(quic.ApplicationErrorCode(ErrCodeFrameUnexpected), "expected first frame to be a HEADERS frame")
		return
	}
	if hf.Length > uint64(maxHeaderBytes) {
		maybeQlogInvalidHeadersFrame(qlogger, str.StreamID(), hf.Length)
		// stop the client from sending more data
		str.CancelRead(quic.StreamErrorCode(ErrCodeExcessiveLoad))
		// send a 431 Response (Request Header Fields Too Large)
		c.rejectWithHeaderFieldsTooLarge(str)
		return
	}
	headerBlock := make([]byte, hf.Length)
	if _, err := io.ReadFull(str, headerBlock); err != nil {
		maybeQlogInvalidHeadersFrame(qlogger, str.StreamID(), hf.Length)
		str.CancelRead(quic.StreamErrorCode(ErrCodeRequestIncomplete))
		str.CancelWrite(quic.StreamErrorCode(ErrCodeRequestIncomplete))
		return
	}
	decodeFn := decoder.Decode(headerBlock)
	var hfs []qpack.HeaderField
	if qlogger != nil {
		hfs = make([]qpack.HeaderField, 0, 16)
	}
	req, err := requestFromHeaders(decodeFn, maxHeaderBytes, &hfs)
	if qlogger != nil {
		qlogParsedHeadersFrame(qlogger, str.StreamID(), hf, hfs)
	}
	if err != nil {
		if errors.Is(err, errHeaderTooLarge) {
			// stop the client from sending more data
			str.CancelRead(quic.StreamErrorCode(ErrCodeExcessiveLoad))
			// send a 431 Response (Request Header Fields Too Large)
			c.rejectWithHeaderFieldsTooLarge(str)
			return
		}

		errCode := ErrCodeMessageError
		var qpackErr *qpackError
		if errors.As(err, &qpackErr) {
			errCode = ErrCodeQPACKDecompressionFailed
		}
		str.CancelRead(quic.StreamErrorCode(errCode))
		str.CancelWrite(quic.StreamErrorCode(errCode))
		return
	}

	connState := conn.ConnectionState().TLS
	req.TLS = &connState
	req.RemoteAddr = conn.RemoteAddr().String()

	// Check that the client doesn't send more data in DATA frames than indicated by the Content-Length header (if set).
	// See section 4.1.2 of RFC 9114.
	contentLength := int64(-1)
	if _, ok := req.Header["Content-Length"]; ok && req.ContentLength >= 0 {
		contentLength = req.ContentLength
	}
	hstr := newStream(str, conn, nil, func(r io.Reader, hf *headersFrame) error {
		trailers, err := decodeTrailers(r, hf, maxHeaderBytes, decoder, qlogger, str.StreamID())
		if err != nil {
			return err
		}
		req.Trailer = trailers
		return nil
	}, qlogger)
	body := newRequestBody(hstr, contentLength, connCtx, conn.ReceivedSettings(), conn.Settings)
	req.Body = body

	if c.logger != nil {
		c.logger.Debug("handling request", "method", req.Method, "host", req.Host, "uri", req.RequestURI)
	}

	ctx, cancel := context.WithCancel(connCtx)
	req = req.WithContext(ctx)
	context.AfterFunc(str.Context(), cancel)

	r := newResponseWriter(hstr, conn, req.Method == http.MethodHead, c.logger)
	handler := c.requestHandler
	if handler == nil {
		handler = http.DefaultServeMux
	}

	// It's the client's responsibility to decide which requests are eligible for 0-RTT.
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
				logger := c.logger
				if logger == nil {
					logger = slog.Default()
				}
				logger.Error("http3: panic serving", "arg", p, "trace", string(buf))
			}
		}()
		handler.ServeHTTP(r, req)
	}()

	if r.wasStreamHijacked() {
		return
	}

	// abort the stream when there is a panic
	if panicked {
		str.CancelRead(quic.StreamErrorCode(ErrCodeInternalError))
		str.CancelWrite(quic.StreamErrorCode(ErrCodeInternalError))
		return
	}

	// response not written to the client yet, set Content-Length
	if !r.headerWritten {
		if _, haveCL := r.header["Content-Length"]; !haveCL {
			r.header.Set("Content-Length", strconv.FormatInt(r.numWritten, 10))
		}
	}
	r.Flush()
	r.flushTrailers()

	// If the EOF was read by the handler, CancelRead() is a no-op.
	str.CancelRead(quic.StreamErrorCode(ErrCodeNoError))
	str.Close()
}

func (c *RawServerConn) rejectWithHeaderFieldsTooLarge(str *stateTrackingStream) {
	hstr := newStream(str, &c.rawConn, nil, nil, c.qlogger)
	defer hstr.Close()
	r := newResponseWriter(hstr, &c.rawConn, false, c.logger)
	r.WriteHeader(http.StatusRequestHeaderFieldsTooLarge)
	r.Flush()
}

// HandleUnidirectionalStream handles an incoming unidirectional stream.
func (c *RawServerConn) HandleUnidirectionalStream(str *quic.ReceiveStream) {
	c.rawConn.handleUnidirectionalStream(str, true)
}
