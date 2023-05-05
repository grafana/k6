// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"
)

var strPermessageDeflate = []byte("permessage-deflate")

var poolWriteBuffer = sync.Pool{
	New: func() interface{} {
		var buf []byte
		return buf
	},
}

// FastHTTPHandler receives a websocket connection after the handshake has been
// completed. This must be provided.
type FastHTTPHandler func(*Conn)

// FastHTTPUpgrader specifies parameters for upgrading an HTTP connection to a
// WebSocket connection.
type FastHTTPUpgrader struct {
	// HandshakeTimeout specifies the duration for the handshake to complete.
	HandshakeTimeout time.Duration

	// ReadBufferSize and WriteBufferSize specify I/O buffer sizes in bytes. If a buffer
	// size is zero, then buffers allocated by the HTTP server are used. The
	// I/O buffer sizes do not limit the size of the messages that can be sent
	// or received.
	ReadBufferSize, WriteBufferSize int

	// WriteBufferPool is a pool of buffers for write operations. If the value
	// is not set, then write buffers are allocated to the connection for the
	// lifetime of the connection.
	//
	// A pool is most useful when the application has a modest volume of writes
	// across a large number of connections.
	//
	// Applications should use a single pool for each unique value of
	// WriteBufferSize.
	WriteBufferPool BufferPool

	// Subprotocols specifies the server's supported protocols in order of
	// preference. If this field is not nil, then the Upgrade method negotiates a
	// subprotocol by selecting the first match in this list with a protocol
	// requested by the client. If there's no match, then no protocol is
	// negotiated (the Sec-Websocket-Protocol header is not included in the
	// handshake response).
	Subprotocols []string

	// Error specifies the function for generating HTTP error responses. If Error
	// is nil, then http.Error is used to generate the HTTP response.
	Error func(ctx *fasthttp.RequestCtx, status int, reason error)

	// CheckOrigin returns true if the request Origin header is acceptable. If
	// CheckOrigin is nil, then a safe default is used: return false if the
	// Origin request header is present and the origin host is not equal to
	// request Host header.
	//
	// A CheckOrigin function should carefully validate the request origin to
	// prevent cross-site request forgery.
	CheckOrigin func(ctx *fasthttp.RequestCtx) bool

	// EnableCompression specify if the server should attempt to negotiate per
	// message compression (RFC 7692). Setting this value to true does not
	// guarantee that compression will be supported. Currently only "no context
	// takeover" modes are supported.
	EnableCompression bool
}

func (u *FastHTTPUpgrader) responseError(ctx *fasthttp.RequestCtx, status int, reason string) error {
	err := HandshakeError{reason}
	if u.Error != nil {
		u.Error(ctx, status, err)
	} else {
		ctx.Response.Header.Set("Sec-Websocket-Version", "13")
		ctx.Error(fasthttp.StatusMessage(status), status)
	}

	return err
}

func (u *FastHTTPUpgrader) selectSubprotocol(ctx *fasthttp.RequestCtx) []byte {
	if u.Subprotocols != nil {
		clientProtocols := parseDataHeader(ctx.Request.Header.Peek("Sec-Websocket-Protocol"))

		for _, serverProtocol := range u.Subprotocols {
			for _, clientProtocol := range clientProtocols {
				if strconv.B2S(clientProtocol) == serverProtocol {
					return clientProtocol
				}
			}
		}
	} else if ctx.Response.Header.Len() > 0 {
		return ctx.Response.Header.Peek("Sec-Websocket-Protocol")
	}

	return nil
}

func (u *FastHTTPUpgrader) isCompressionEnable(ctx *fasthttp.RequestCtx) bool {
	extensions := parseDataHeader(ctx.Request.Header.Peek("Sec-WebSocket-Extensions"))

	// Negotiate PMCE
	if u.EnableCompression {
		for _, ext := range extensions {
			if bytes.HasPrefix(ext, strPermessageDeflate) {
				return true
			}
		}
	}

	return false
}

// Upgrade upgrades the HTTP server connection to the WebSocket protocol.
//
// The responseHeader is included in the response to the client's upgrade
// request. Use the responseHeader to specify cookies (Set-Cookie) and the
// application negotiated subprotocol (Sec-WebSocket-Protocol).
//
// If the upgrade fails, then Upgrade replies to the client with an HTTP error
// response.
func (u *FastHTTPUpgrader) Upgrade(ctx *fasthttp.RequestCtx, handler FastHTTPHandler) error {
	if !ctx.IsGet() {
		return u.responseError(ctx, fasthttp.StatusMethodNotAllowed, fmt.Sprintf("%s request method is not GET", badHandshake))
	}

	if !tokenContainsValue(strconv.B2S(ctx.Request.Header.Peek("Connection")), "Upgrade") {
		return u.responseError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("%s 'upgrade' token not found in 'Connection' header", badHandshake))
	}

	if !tokenContainsValue(strconv.B2S(ctx.Request.Header.Peek("Upgrade")), "Websocket") {
		return u.responseError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("%s 'websocket' token not found in 'Upgrade' header", badHandshake))
	}

	if !tokenContainsValue(strconv.B2S(ctx.Request.Header.Peek("Sec-Websocket-Version")), "13") {
		return u.responseError(ctx, fasthttp.StatusBadRequest, "websocket: unsupported version: 13 not found in 'Sec-Websocket-Version' header")
	}

	if len(ctx.Response.Header.Peek("Sec-Websocket-Extensions")) > 0 {
		return u.responseError(ctx, fasthttp.StatusInternalServerError, "websocket: application specific 'Sec-WebSocket-Extensions' headers are unsupported")
	}

	checkOrigin := u.CheckOrigin
	if checkOrigin == nil {
		checkOrigin = fastHTTPcheckSameOrigin
	}
	if !checkOrigin(ctx) {
		return u.responseError(ctx, fasthttp.StatusForbidden, "websocket: request origin not allowed by FastHTTPUpgrader.CheckOrigin")
	}

	challengeKey := ctx.Request.Header.Peek("Sec-Websocket-Key")
	if len(challengeKey) == 0 {
		return u.responseError(ctx, fasthttp.StatusBadRequest, "websocket: not a websocket handshake: `Sec-WebSocket-Key' header is missing or blank")
	}

	subprotocol := u.selectSubprotocol(ctx)
	compress := u.isCompressionEnable(ctx)

	ctx.SetStatusCode(fasthttp.StatusSwitchingProtocols)
	ctx.Response.Header.Set("Upgrade", "websocket")
	ctx.Response.Header.Set("Connection", "Upgrade")
	ctx.Response.Header.Set("Sec-WebSocket-Accept", computeAcceptKeyBytes(challengeKey))
	if compress {
		ctx.Response.Header.Set("Sec-WebSocket-Extensions", "permessage-deflate; server_no_context_takeover; client_no_context_takeover")
	}
	if subprotocol != nil {
		ctx.Response.Header.SetBytesV("Sec-WebSocket-Protocol", subprotocol)
	}

	ctx.Hijack(func(netConn net.Conn) {
		// var br *bufio.Reader  // Always nil
		writeBuf := poolWriteBuffer.Get().([]byte)

		c := newConn(netConn, true, u.ReadBufferSize, u.WriteBufferSize, u.WriteBufferPool, nil, writeBuf)
		if subprotocol != nil {
			c.subprotocol = strconv.B2S(subprotocol)
		}

		if compress {
			c.newCompressionWriter = compressNoContextTakeover
			c.newDecompressionReader = decompressNoContextTakeover
		}

		// Clear deadlines set by HTTP server.
		netConn.SetDeadline(time.Time{})

		handler(c)

		writeBuf = writeBuf[0:0]
		poolWriteBuffer.Put(writeBuf)
	})

	return nil
}

// fastHTTPcheckSameOrigin returns true if the origin is not set or is equal to the request host.
func fastHTTPcheckSameOrigin(ctx *fasthttp.RequestCtx) bool {
	origin := ctx.Request.Header.Peek("Origin")
	if len(origin) == 0 {
		return true
	}
	u, err := url.Parse(strconv.B2S(origin))
	if err != nil {
		return false
	}
	return equalASCIIFold(u.Host, strconv.B2S(ctx.Host()))
}

// FastHTTPIsWebSocketUpgrade returns true if the client requested upgrade to the
// WebSocket protocol.
func FastHTTPIsWebSocketUpgrade(ctx *fasthttp.RequestCtx) bool {
	return tokenContainsValue(strconv.B2S(ctx.Request.Header.Peek("Connection")), "Upgrade") &&
		tokenContainsValue(strconv.B2S(ctx.Request.Header.Peek("Upgrade")), "Websocket")
}
