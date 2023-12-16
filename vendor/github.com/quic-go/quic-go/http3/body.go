package http3

import (
	"context"
	"io"
	"net"

	"github.com/quic-go/quic-go"
)

// The HTTPStreamer allows taking over a HTTP/3 stream. The interface is implemented by:
// * for the server: the http.Request.Body
// * for the client: the http.Response.Body
// On the client side, the stream will be closed for writing, unless the DontCloseRequestStream RoundTripOpt was set.
// When a stream is taken over, it's the caller's responsibility to close the stream.
type HTTPStreamer interface {
	HTTPStream() Stream
}

type StreamCreator interface {
	// Context returns a context that is cancelled when the underlying connection is closed.
	Context() context.Context
	OpenStream() (quic.Stream, error)
	OpenStreamSync(context.Context) (quic.Stream, error)
	OpenUniStream() (quic.SendStream, error)
	OpenUniStreamSync(context.Context) (quic.SendStream, error)
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	ConnectionState() quic.ConnectionState
}

var _ StreamCreator = quic.Connection(nil)

// A Hijacker allows hijacking of the stream creating part of a quic.Session from a http.Response.Body.
// It is used by WebTransport to create WebTransport streams after a session has been established.
type Hijacker interface {
	StreamCreator() StreamCreator
}

// The body of a http.Request or http.Response.
type body struct {
	str quic.Stream

	wasHijacked bool // set when HTTPStream is called
}

var (
	_ io.ReadCloser = &body{}
	_ HTTPStreamer  = &body{}
)

func newRequestBody(str Stream) *body {
	return &body{str: str}
}

func (r *body) HTTPStream() Stream {
	r.wasHijacked = true
	return r.str
}

func (r *body) wasStreamHijacked() bool {
	return r.wasHijacked
}

func (r *body) Read(b []byte) (int, error) {
	n, err := r.str.Read(b)
	return n, maybeReplaceError(err)
}

func (r *body) Close() error {
	r.str.CancelRead(quic.StreamErrorCode(ErrCodeRequestCanceled))
	return nil
}

type hijackableBody struct {
	body
	conn quic.Connection // only needed to implement Hijacker

	// only set for the http.Response
	// The channel is closed when the user is done with this response:
	// either when Read() errors, or when Close() is called.
	reqDone       chan<- struct{}
	reqDoneClosed bool
}

var (
	_ Hijacker     = &hijackableBody{}
	_ HTTPStreamer = &hijackableBody{}
)

func newResponseBody(str Stream, conn quic.Connection, done chan<- struct{}) *hijackableBody {
	return &hijackableBody{
		body: body{
			str: str,
		},
		reqDone: done,
		conn:    conn,
	}
}

func (r *hijackableBody) StreamCreator() StreamCreator {
	return r.conn
}

func (r *hijackableBody) Read(b []byte) (int, error) {
	n, err := r.str.Read(b)
	if err != nil {
		r.requestDone()
	}
	return n, maybeReplaceError(err)
}

func (r *hijackableBody) requestDone() {
	if r.reqDoneClosed || r.reqDone == nil {
		return
	}
	if r.reqDone != nil {
		close(r.reqDone)
	}
	r.reqDoneClosed = true
}

func (r *body) StreamID() quic.StreamID {
	return r.str.StreamID()
}

func (r *hijackableBody) Close() error {
	r.requestDone()
	// If the EOF was read, CancelRead() is a no-op.
	r.str.CancelRead(quic.StreamErrorCode(ErrCodeRequestCanceled))
	return nil
}

func (r *hijackableBody) HTTPStream() Stream {
	return r.str
}
