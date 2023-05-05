package fasthttp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

const (
	rChar = byte('\r')
	nChar = byte('\n')
)

// ResponseHeader represents HTTP response header.
//
// It is forbidden copying ResponseHeader instances.
// Create new instances instead and use CopyTo.
//
// ResponseHeader instance MUST NOT be used from concurrently running
// goroutines.
type ResponseHeader struct {
	noCopy noCopy

	disableNormalizing   bool
	noHTTP11             bool
	connectionClose      bool
	noDefaultContentType bool
	noDefaultDate        bool

	statusCode            int
	statusMessage         []byte
	protocol              []byte
	contentLength         int
	contentLengthBytes    []byte
	secureErrorLogMessage bool

	contentType     []byte
	contentEncoding []byte
	server          []byte
	mulHeader       [][]byte

	h       []argsKV
	trailer []argsKV
	bufKV   argsKV

	cookies []argsKV
}

// RequestHeader represents HTTP request header.
//
// It is forbidden copying RequestHeader instances.
// Create new instances instead and use CopyTo.
//
// RequestHeader instance MUST NOT be used from concurrently running
// goroutines.
type RequestHeader struct {
	noCopy noCopy

	disableNormalizing   bool
	noHTTP11             bool
	connectionClose      bool
	noDefaultContentType bool

	// These two fields have been moved close to other bool fields
	// for reducing RequestHeader object size.
	cookiesCollected bool

	contentLength         int
	contentLengthBytes    []byte
	secureErrorLogMessage bool

	method      []byte
	requestURI  []byte
	proto       []byte
	host        []byte
	contentType []byte
	userAgent   []byte
	mulHeader   [][]byte

	h       []argsKV
	trailer []argsKV
	bufKV   argsKV

	cookies []argsKV

	// stores an immutable copy of headers as they were received from the
	// wire.
	rawHeaders []byte
}

// SetContentRange sets 'Content-Range: bytes startPos-endPos/contentLength'
// header.
func (h *ResponseHeader) SetContentRange(startPos, endPos, contentLength int) {
	b := h.bufKV.value[:0]
	b = append(b, strBytes...)
	b = append(b, ' ')
	b = AppendUint(b, startPos)
	b = append(b, '-')
	b = AppendUint(b, endPos)
	b = append(b, '/')
	b = AppendUint(b, contentLength)
	h.bufKV.value = b

	h.setNonSpecial(strContentRange, h.bufKV.value)
}

// SetByteRange sets 'Range: bytes=startPos-endPos' header.
//
//   - If startPos is negative, then 'bytes=-startPos' value is set.
//   - If endPos is negative, then 'bytes=startPos-' value is set.
func (h *RequestHeader) SetByteRange(startPos, endPos int) {
	b := h.bufKV.value[:0]
	b = append(b, strBytes...)
	b = append(b, '=')
	if startPos >= 0 {
		b = AppendUint(b, startPos)
	} else {
		endPos = -startPos
	}
	b = append(b, '-')
	if endPos >= 0 {
		b = AppendUint(b, endPos)
	}
	h.bufKV.value = b

	h.setNonSpecial(strRange, h.bufKV.value)
}

// StatusCode returns response status code.
func (h *ResponseHeader) StatusCode() int {
	if h.statusCode == 0 {
		return StatusOK
	}
	return h.statusCode
}

// SetStatusCode sets response status code.
func (h *ResponseHeader) SetStatusCode(statusCode int) {
	h.statusCode = statusCode
}

// StatusMessage returns response status message.
func (h *ResponseHeader) StatusMessage() []byte {
	return h.statusMessage
}

// SetStatusMessage sets response status message bytes.
func (h *ResponseHeader) SetStatusMessage(statusMessage []byte) {
	h.statusMessage = append(h.statusMessage[:0], statusMessage...)
}

// Protocol returns response protocol bytes.
func (h *ResponseHeader) Protocol() []byte {
	if len(h.protocol) > 0 {
		return h.protocol
	}
	return strHTTP11
}

// SetProtocol sets response protocol bytes.
func (h *ResponseHeader) SetProtocol(protocol []byte) {
	h.protocol = append(h.protocol[:0], protocol...)
}

// SetLastModified sets 'Last-Modified' header to the given value.
func (h *ResponseHeader) SetLastModified(t time.Time) {
	h.bufKV.value = AppendHTTPDate(h.bufKV.value[:0], t)
	h.setNonSpecial(strLastModified, h.bufKV.value)
}

// ConnectionClose returns true if 'Connection: close' header is set.
func (h *ResponseHeader) ConnectionClose() bool {
	return h.connectionClose
}

// SetConnectionClose sets 'Connection: close' header.
func (h *ResponseHeader) SetConnectionClose() {
	h.connectionClose = true
}

// ResetConnectionClose clears 'Connection: close' header if it exists.
func (h *ResponseHeader) ResetConnectionClose() {
	if h.connectionClose {
		h.connectionClose = false
		h.h = delAllArgsBytes(h.h, strConnection)
	}
}

// ConnectionClose returns true if 'Connection: close' header is set.
func (h *RequestHeader) ConnectionClose() bool {
	return h.connectionClose
}

// SetConnectionClose sets 'Connection: close' header.
func (h *RequestHeader) SetConnectionClose() {
	h.connectionClose = true
}

// ResetConnectionClose clears 'Connection: close' header if it exists.
func (h *RequestHeader) ResetConnectionClose() {
	if h.connectionClose {
		h.connectionClose = false
		h.h = delAllArgsBytes(h.h, strConnection)
	}
}

// ConnectionUpgrade returns true if 'Connection: Upgrade' header is set.
func (h *ResponseHeader) ConnectionUpgrade() bool {
	return hasHeaderValue(h.Peek(HeaderConnection), strUpgrade)
}

// ConnectionUpgrade returns true if 'Connection: Upgrade' header is set.
func (h *RequestHeader) ConnectionUpgrade() bool {
	return hasHeaderValue(h.Peek(HeaderConnection), strUpgrade)
}

// PeekCookie is able to returns cookie by a given key from response.
func (h *ResponseHeader) PeekCookie(key string) []byte {
	return peekArgStr(h.cookies, key)
}

// ContentLength returns Content-Length header value.
//
// It may be negative:
// -1 means Transfer-Encoding: chunked.
// -2 means Transfer-Encoding: identity.
func (h *ResponseHeader) ContentLength() int {
	return h.contentLength
}

// SetContentLength sets Content-Length header value.
//
// Content-Length may be negative:
// -1 means Transfer-Encoding: chunked.
// -2 means Transfer-Encoding: identity.
func (h *ResponseHeader) SetContentLength(contentLength int) {
	if h.mustSkipContentLength() {
		return
	}
	h.contentLength = contentLength
	if contentLength >= 0 {
		h.contentLengthBytes = AppendUint(h.contentLengthBytes[:0], contentLength)
		h.h = delAllArgsBytes(h.h, strTransferEncoding)
	} else {
		h.contentLengthBytes = h.contentLengthBytes[:0]
		value := strChunked
		if contentLength == -2 {
			h.SetConnectionClose()
			value = strIdentity
		}
		h.h = setArgBytes(h.h, strTransferEncoding, value, argsHasValue)
	}
}

func (h *ResponseHeader) mustSkipContentLength() bool {
	// From http/1.1 specs:
	// All 1xx (informational), 204 (no content), and 304 (not modified) responses MUST NOT include a message-body
	statusCode := h.StatusCode()

	// Fast path.
	if statusCode < 100 || statusCode == StatusOK {
		return false
	}

	// Slow path.
	return statusCode == StatusNotModified || statusCode == StatusNoContent || statusCode < 200
}

// ContentLength returns Content-Length header value.
//
// It may be negative:
// -1 means Transfer-Encoding: chunked.
func (h *RequestHeader) ContentLength() int {
	return h.realContentLength()
}

// realContentLength returns the actual Content-Length set in the request,
// including positive lengths for GET/HEAD requests.
func (h *RequestHeader) realContentLength() int {
	return h.contentLength
}

// SetContentLength sets Content-Length header value.
//
// Negative content-length sets 'Transfer-Encoding: chunked' header.
func (h *RequestHeader) SetContentLength(contentLength int) {
	h.contentLength = contentLength
	if contentLength >= 0 {
		h.contentLengthBytes = AppendUint(h.contentLengthBytes[:0], contentLength)
		h.h = delAllArgsBytes(h.h, strTransferEncoding)
	} else {
		h.contentLengthBytes = h.contentLengthBytes[:0]
		h.h = setArgBytes(h.h, strTransferEncoding, strChunked, argsHasValue)
	}
}

func (h *ResponseHeader) isCompressibleContentType() bool {
	contentType := h.ContentType()
	return bytes.HasPrefix(contentType, strTextSlash) ||
		bytes.HasPrefix(contentType, strApplicationSlash) ||
		bytes.HasPrefix(contentType, strImageSVG) ||
		bytes.HasPrefix(contentType, strImageIcon) ||
		bytes.HasPrefix(contentType, strFontSlash) ||
		bytes.HasPrefix(contentType, strMultipartSlash)
}

// ContentType returns Content-Type header value.
func (h *ResponseHeader) ContentType() []byte {
	contentType := h.contentType
	if !h.noDefaultContentType && len(h.contentType) == 0 {
		contentType = defaultContentType
	}
	return contentType
}

// SetContentType sets Content-Type header value.
func (h *ResponseHeader) SetContentType(contentType string) {
	h.contentType = append(h.contentType[:0], contentType...)
}

// SetContentTypeBytes sets Content-Type header value.
func (h *ResponseHeader) SetContentTypeBytes(contentType []byte) {
	h.contentType = append(h.contentType[:0], contentType...)
}

// ContentEncoding returns Content-Encoding header value.
func (h *ResponseHeader) ContentEncoding() []byte {
	return h.contentEncoding
}

// SetContentEncoding sets Content-Encoding header value.
func (h *ResponseHeader) SetContentEncoding(contentEncoding string) {
	h.contentEncoding = append(h.contentEncoding[:0], contentEncoding...)
}

// SetContentEncodingBytes sets Content-Encoding header value.
func (h *ResponseHeader) SetContentEncodingBytes(contentEncoding []byte) {
	h.contentEncoding = append(h.contentEncoding[:0], contentEncoding...)
}

// Server returns Server header value.
func (h *ResponseHeader) Server() []byte {
	return h.server
}

// SetServer sets Server header value.
func (h *ResponseHeader) SetServer(server string) {
	h.server = append(h.server[:0], server...)
}

// SetServerBytes sets Server header value.
func (h *ResponseHeader) SetServerBytes(server []byte) {
	h.server = append(h.server[:0], server...)
}

// ContentType returns Content-Type header value.
func (h *RequestHeader) ContentType() []byte {
	return h.contentType
}

// SetContentType sets Content-Type header value.
func (h *RequestHeader) SetContentType(contentType string) {
	h.contentType = append(h.contentType[:0], contentType...)
}

// SetContentTypeBytes sets Content-Type header value.
func (h *RequestHeader) SetContentTypeBytes(contentType []byte) {
	h.contentType = append(h.contentType[:0], contentType...)
}

// ContentEncoding returns Content-Encoding header value.
func (h *RequestHeader) ContentEncoding() []byte {
	return peekArgBytes(h.h, strContentEncoding)
}

// SetContentEncoding sets Content-Encoding header value.
func (h *RequestHeader) SetContentEncoding(contentEncoding string) {
	h.SetBytesK(strContentEncoding, contentEncoding)
}

// SetContentEncodingBytes sets Content-Encoding header value.
func (h *RequestHeader) SetContentEncodingBytes(contentEncoding []byte) {
	h.setNonSpecial(strContentEncoding, contentEncoding)
}

// SetMultipartFormBoundary sets the following Content-Type:
// 'multipart/form-data; boundary=...'
// where ... is substituted by the given boundary.
func (h *RequestHeader) SetMultipartFormBoundary(boundary string) {
	b := h.bufKV.value[:0]
	b = append(b, strMultipartFormData...)
	b = append(b, ';', ' ')
	b = append(b, strBoundary...)
	b = append(b, '=')
	b = append(b, boundary...)
	h.bufKV.value = b

	h.SetContentTypeBytes(h.bufKV.value)
}

// SetMultipartFormBoundaryBytes sets the following Content-Type:
// 'multipart/form-data; boundary=...'
// where ... is substituted by the given boundary.
func (h *RequestHeader) SetMultipartFormBoundaryBytes(boundary []byte) {
	b := h.bufKV.value[:0]
	b = append(b, strMultipartFormData...)
	b = append(b, ';', ' ')
	b = append(b, strBoundary...)
	b = append(b, '=')
	b = append(b, boundary...)
	h.bufKV.value = b

	h.SetContentTypeBytes(h.bufKV.value)
}

// SetTrailer sets header Trailer value for chunked response
// to indicate which headers will be sent after the body.
//
// Use Set to set the trailer header later.
//
// Trailers are only supported with chunked transfer.
// Trailers allow the sender to include additional headers at the end of chunked messages.
//
// The following trailers are forbidden:
// 1. necessary for message framing (e.g., Transfer-Encoding and Content-Length),
// 2. routing (e.g., Host),
// 3. request modifiers (e.g., controls and conditionals in Section 5 of [RFC7231]),
// 4. authentication (e.g., see [RFC7235] and [RFC6265]),
// 5. response control data (e.g., see Section 7.1 of [RFC7231]),
// 6. determining how to process the payload (e.g., Content-Encoding, Content-Type, Content-Range, and Trailer)
//
// Return ErrBadTrailer if contain any forbidden trailers.
func (h *ResponseHeader) SetTrailer(trailer string) error {
	return h.SetTrailerBytes(s2b(trailer))
}

// SetTrailerBytes sets Trailer header value for chunked response
// to indicate which headers will be sent after the body.
//
// Use Set to set the trailer header later.
//
// Trailers are only supported with chunked transfer.
// Trailers allow the sender to include additional headers at the end of chunked messages.
//
// The following trailers are forbidden:
// 1. necessary for message framing (e.g., Transfer-Encoding and Content-Length),
// 2. routing (e.g., Host),
// 3. request modifiers (e.g., controls and conditionals in Section 5 of [RFC7231]),
// 4. authentication (e.g., see [RFC7235] and [RFC6265]),
// 5. response control data (e.g., see Section 7.1 of [RFC7231]),
// 6. determining how to process the payload (e.g., Content-Encoding, Content-Type, Content-Range, and Trailer)
//
// Return ErrBadTrailer if contain any forbidden trailers.
func (h *ResponseHeader) SetTrailerBytes(trailer []byte) error {
	h.trailer = h.trailer[:0]
	return h.AddTrailerBytes(trailer)
}

// AddTrailer add Trailer header value for chunked response
// to indicate which headers will be sent after the body.
//
// Use Set to set the trailer header later.
//
// Trailers are only supported with chunked transfer.
// Trailers allow the sender to include additional headers at the end of chunked messages.
//
// The following trailers are forbidden:
// 1. necessary for message framing (e.g., Transfer-Encoding and Content-Length),
// 2. routing (e.g., Host),
// 3. request modifiers (e.g., controls and conditionals in Section 5 of [RFC7231]),
// 4. authentication (e.g., see [RFC7235] and [RFC6265]),
// 5. response control data (e.g., see Section 7.1 of [RFC7231]),
// 6. determining how to process the payload (e.g., Content-Encoding, Content-Type, Content-Range, and Trailer)
//
// Return ErrBadTrailer if contain any forbidden trailers.
func (h *ResponseHeader) AddTrailer(trailer string) error {
	return h.AddTrailerBytes(s2b(trailer))
}

var ErrBadTrailer = errors.New("contain forbidden trailer")

// AddTrailerBytes add Trailer header value for chunked response
// to indicate which headers will be sent after the body.
//
// Use Set to set the trailer header later.
//
// Trailers are only supported with chunked transfer.
// Trailers allow the sender to include additional headers at the end of chunked messages.
//
// The following trailers are forbidden:
// 1. necessary for message framing (e.g., Transfer-Encoding and Content-Length),
// 2. routing (e.g., Host),
// 3. request modifiers (e.g., controls and conditionals in Section 5 of [RFC7231]),
// 4. authentication (e.g., see [RFC7235] and [RFC6265]),
// 5. response control data (e.g., see Section 7.1 of [RFC7231]),
// 6. determining how to process the payload (e.g., Content-Encoding, Content-Type, Content-Range, and Trailer)
//
// Return ErrBadTrailer if contain any forbidden trailers.
func (h *ResponseHeader) AddTrailerBytes(trailer []byte) error {
	var err error
	for i := -1; i+1 < len(trailer); {
		trailer = trailer[i+1:]
		i = bytes.IndexByte(trailer, ',')
		if i < 0 {
			i = len(trailer)
		}
		key := trailer[:i]
		for len(key) > 0 && key[0] == ' ' {
			key = key[1:]
		}
		for len(key) > 0 && key[len(key)-1] == ' ' {
			key = key[:len(key)-1]
		}
		// Forbidden by RFC 7230, section 4.1.2
		if isBadTrailer(key) {
			err = ErrBadTrailer
			continue
		}
		h.bufKV.key = append(h.bufKV.key[:0], key...)
		normalizeHeaderKey(h.bufKV.key, h.disableNormalizing)
		h.trailer = appendArgBytes(h.trailer, h.bufKV.key, nil, argsNoValue)
	}

	return err
}

// MultipartFormBoundary returns boundary part
// from 'multipart/form-data; boundary=...' Content-Type.
func (h *RequestHeader) MultipartFormBoundary() []byte {
	b := h.ContentType()
	if !bytes.HasPrefix(b, strMultipartFormData) {
		return nil
	}
	b = b[len(strMultipartFormData):]
	if len(b) == 0 || b[0] != ';' {
		return nil
	}

	var n int
	for len(b) > 0 {
		n++
		for len(b) > n && b[n] == ' ' {
			n++
		}
		b = b[n:]
		if !bytes.HasPrefix(b, strBoundary) {
			if n = bytes.IndexByte(b, ';'); n < 0 {
				return nil
			}
			continue
		}

		b = b[len(strBoundary):]
		if len(b) == 0 || b[0] != '=' {
			return nil
		}
		b = b[1:]
		if n = bytes.IndexByte(b, ';'); n >= 0 {
			b = b[:n]
		}
		if len(b) > 1 && b[0] == '"' && b[len(b)-1] == '"' {
			b = b[1 : len(b)-1]
		}
		return b
	}
	return nil
}

// Host returns Host header value.
func (h *RequestHeader) Host() []byte {
	return h.host
}

// SetHost sets Host header value.
func (h *RequestHeader) SetHost(host string) {
	h.host = append(h.host[:0], host...)
}

// SetHostBytes sets Host header value.
func (h *RequestHeader) SetHostBytes(host []byte) {
	h.host = append(h.host[:0], host...)
}

// UserAgent returns User-Agent header value.
func (h *RequestHeader) UserAgent() []byte {
	return h.userAgent
}

// SetUserAgent sets User-Agent header value.
func (h *RequestHeader) SetUserAgent(userAgent string) {
	h.userAgent = append(h.userAgent[:0], userAgent...)
}

// SetUserAgentBytes sets User-Agent header value.
func (h *RequestHeader) SetUserAgentBytes(userAgent []byte) {
	h.userAgent = append(h.userAgent[:0], userAgent...)
}

// Referer returns Referer header value.
func (h *RequestHeader) Referer() []byte {
	return peekArgBytes(h.h, strReferer)
}

// SetReferer sets Referer header value.
func (h *RequestHeader) SetReferer(referer string) {
	h.SetBytesK(strReferer, referer)
}

// SetRefererBytes sets Referer header value.
func (h *RequestHeader) SetRefererBytes(referer []byte) {
	h.setNonSpecial(strReferer, referer)
}

// Method returns HTTP request method.
func (h *RequestHeader) Method() []byte {
	if len(h.method) == 0 {
		return []byte(MethodGet)
	}
	return h.method
}

// SetMethod sets HTTP request method.
func (h *RequestHeader) SetMethod(method string) {
	h.method = append(h.method[:0], method...)
}

// SetMethodBytes sets HTTP request method.
func (h *RequestHeader) SetMethodBytes(method []byte) {
	h.method = append(h.method[:0], method...)
}

// Protocol returns HTTP protocol.
func (h *RequestHeader) Protocol() []byte {
	if len(h.proto) == 0 {
		return strHTTP11
	}
	return h.proto
}

// SetProtocol sets HTTP request protocol.
func (h *RequestHeader) SetProtocol(method string) {
	h.proto = append(h.proto[:0], method...)
	h.noHTTP11 = !bytes.Equal(h.proto, strHTTP11)
}

// SetProtocolBytes sets HTTP request protocol.
func (h *RequestHeader) SetProtocolBytes(method []byte) {
	h.proto = append(h.proto[:0], method...)
	h.noHTTP11 = !bytes.Equal(h.proto, strHTTP11)
}

// RequestURI returns RequestURI from the first HTTP request line.
func (h *RequestHeader) RequestURI() []byte {
	requestURI := h.requestURI
	if len(requestURI) == 0 {
		requestURI = strSlash
	}
	return requestURI
}

// SetRequestURI sets RequestURI for the first HTTP request line.
// RequestURI must be properly encoded.
// Use URI.RequestURI for constructing proper RequestURI if unsure.
func (h *RequestHeader) SetRequestURI(requestURI string) {
	h.requestURI = append(h.requestURI[:0], requestURI...)
}

// SetRequestURIBytes sets RequestURI for the first HTTP request line.
// RequestURI must be properly encoded.
// Use URI.RequestURI for constructing proper RequestURI if unsure.
func (h *RequestHeader) SetRequestURIBytes(requestURI []byte) {
	h.requestURI = append(h.requestURI[:0], requestURI...)
}

// SetTrailer sets Trailer header value for chunked request
// to indicate which headers will be sent after the body.
//
// Use Set to set the trailer header later.
//
// Trailers are only supported with chunked transfer.
// Trailers allow the sender to include additional headers at the end of chunked messages.
//
// The following trailers are forbidden:
// 1. necessary for message framing (e.g., Transfer-Encoding and Content-Length),
// 2. routing (e.g., Host),
// 3. request modifiers (e.g., controls and conditionals in Section 5 of [RFC7231]),
// 4. authentication (e.g., see [RFC7235] and [RFC6265]),
// 5. response control data (e.g., see Section 7.1 of [RFC7231]),
// 6. determining how to process the payload (e.g., Content-Encoding, Content-Type, Content-Range, and Trailer)
//
// Return ErrBadTrailer if contain any forbidden trailers.
func (h *RequestHeader) SetTrailer(trailer string) error {
	return h.SetTrailerBytes(s2b(trailer))
}

// SetTrailerBytes sets Trailer header value for chunked request
// to indicate which headers will be sent after the body.
//
// Use Set to set the trailer header later.
//
// Trailers are only supported with chunked transfer.
// Trailers allow the sender to include additional headers at the end of chunked messages.
//
// The following trailers are forbidden:
// 1. necessary for message framing (e.g., Transfer-Encoding and Content-Length),
// 2. routing (e.g., Host),
// 3. request modifiers (e.g., controls and conditionals in Section 5 of [RFC7231]),
// 4. authentication (e.g., see [RFC7235] and [RFC6265]),
// 5. response control data (e.g., see Section 7.1 of [RFC7231]),
// 6. determining how to process the payload (e.g., Content-Encoding, Content-Type, Content-Range, and Trailer)
//
// Return ErrBadTrailer if contain any forbidden trailers.
func (h *RequestHeader) SetTrailerBytes(trailer []byte) error {
	h.trailer = h.trailer[:0]
	return h.AddTrailerBytes(trailer)
}

// AddTrailer add Trailer header value for chunked request
// to indicate which headers will be sent after the body.
//
// Use Set to set the trailer header later.
//
// Trailers are only supported with chunked transfer.
// Trailers allow the sender to include additional headers at the end of chunked messages.
//
// The following trailers are forbidden:
// 1. necessary for message framing (e.g., Transfer-Encoding and Content-Length),
// 2. routing (e.g., Host),
// 3. request modifiers (e.g., controls and conditionals in Section 5 of [RFC7231]),
// 4. authentication (e.g., see [RFC7235] and [RFC6265]),
// 5. response control data (e.g., see Section 7.1 of [RFC7231]),
// 6. determining how to process the payload (e.g., Content-Encoding, Content-Type, Content-Range, and Trailer)
//
// Return ErrBadTrailer if contain any forbidden trailers.
func (h *RequestHeader) AddTrailer(trailer string) error {
	return h.AddTrailerBytes(s2b(trailer))
}

// AddTrailerBytes add Trailer header value for chunked request
// to indicate which headers will be sent after the body.
//
// Use Set to set the trailer header later.
//
// Trailers are only supported with chunked transfer.
// Trailers allow the sender to include additional headers at the end of chunked messages.
//
// The following trailers are forbidden:
// 1. necessary for message framing (e.g., Transfer-Encoding and Content-Length),
// 2. routing (e.g., Host),
// 3. request modifiers (e.g., controls and conditionals in Section 5 of [RFC7231]),
// 4. authentication (e.g., see [RFC7235] and [RFC6265]),
// 5. response control data (e.g., see Section 7.1 of [RFC7231]),
// 6. determining how to process the payload (e.g., Content-Encoding, Content-Type, Content-Range, and Trailer)
//
// Return ErrBadTrailer if contain any forbidden trailers.
func (h *RequestHeader) AddTrailerBytes(trailer []byte) error {
	var err error
	for i := -1; i+1 < len(trailer); {
		trailer = trailer[i+1:]
		i = bytes.IndexByte(trailer, ',')
		if i < 0 {
			i = len(trailer)
		}
		key := trailer[:i]
		for len(key) > 0 && key[0] == ' ' {
			key = key[1:]
		}
		for len(key) > 0 && key[len(key)-1] == ' ' {
			key = key[:len(key)-1]
		}
		// Forbidden by RFC 7230, section 4.1.2
		if isBadTrailer(key) {
			err = ErrBadTrailer
			continue
		}
		h.bufKV.key = append(h.bufKV.key[:0], key...)
		normalizeHeaderKey(h.bufKV.key, h.disableNormalizing)
		h.trailer = appendArgBytes(h.trailer, h.bufKV.key, nil, argsNoValue)
	}

	return err
}

// IsGet returns true if request method is GET.
func (h *RequestHeader) IsGet() bool {
	return string(h.Method()) == MethodGet
}

// IsPost returns true if request method is POST.
func (h *RequestHeader) IsPost() bool {
	return string(h.Method()) == MethodPost
}

// IsPut returns true if request method is PUT.
func (h *RequestHeader) IsPut() bool {
	return string(h.Method()) == MethodPut
}

// IsHead returns true if request method is HEAD.
func (h *RequestHeader) IsHead() bool {
	return string(h.Method()) == MethodHead
}

// IsDelete returns true if request method is DELETE.
func (h *RequestHeader) IsDelete() bool {
	return string(h.Method()) == MethodDelete
}

// IsConnect returns true if request method is CONNECT.
func (h *RequestHeader) IsConnect() bool {
	return string(h.Method()) == MethodConnect
}

// IsOptions returns true if request method is OPTIONS.
func (h *RequestHeader) IsOptions() bool {
	return string(h.Method()) == MethodOptions
}

// IsTrace returns true if request method is TRACE.
func (h *RequestHeader) IsTrace() bool {
	return string(h.Method()) == MethodTrace
}

// IsPatch returns true if request method is PATCH.
func (h *RequestHeader) IsPatch() bool {
	return string(h.Method()) == MethodPatch
}

// IsHTTP11 returns true if the request is HTTP/1.1.
func (h *RequestHeader) IsHTTP11() bool {
	return !h.noHTTP11
}

// IsHTTP11 returns true if the response is HTTP/1.1.
func (h *ResponseHeader) IsHTTP11() bool {
	return !h.noHTTP11
}

// HasAcceptEncoding returns true if the header contains
// the given Accept-Encoding value.
func (h *RequestHeader) HasAcceptEncoding(acceptEncoding string) bool {
	h.bufKV.value = append(h.bufKV.value[:0], acceptEncoding...)
	return h.HasAcceptEncodingBytes(h.bufKV.value)
}

// HasAcceptEncodingBytes returns true if the header contains
// the given Accept-Encoding value.
func (h *RequestHeader) HasAcceptEncodingBytes(acceptEncoding []byte) bool {
	ae := h.peek(strAcceptEncoding)
	n := bytes.Index(ae, acceptEncoding)
	if n < 0 {
		return false
	}
	b := ae[n+len(acceptEncoding):]
	if len(b) > 0 && b[0] != ',' {
		return false
	}
	if n == 0 {
		return true
	}
	return ae[n-1] == ' '
}

// Len returns the number of headers set,
// i.e. the number of times f is called in VisitAll.
func (h *ResponseHeader) Len() int {
	n := 0
	h.VisitAll(func(_, _ []byte) { n++ })
	return n
}

// Len returns the number of headers set,
// i.e. the number of times f is called in VisitAll.
func (h *RequestHeader) Len() int {
	n := 0
	h.VisitAll(func(_, _ []byte) { n++ })
	return n
}

// DisableNormalizing disables header names' normalization.
//
// By default all the header names are normalized by uppercasing
// the first letter and all the first letters following dashes,
// while lowercasing all the other letters.
// Examples:
//
//   - CONNECTION -> Connection
//   - conteNT-tYPE -> Content-Type
//   - foo-bar-baz -> Foo-Bar-Baz
//
// Disable header names' normalization only if know what are you doing.
func (h *RequestHeader) DisableNormalizing() {
	h.disableNormalizing = true
}

// EnableNormalizing enables header names' normalization.
//
// Header names are normalized by uppercasing the first letter and
// all the first letters following dashes, while lowercasing all
// the other letters.
// Examples:
//
//   - CONNECTION -> Connection
//   - conteNT-tYPE -> Content-Type
//   - foo-bar-baz -> Foo-Bar-Baz
//
// This is enabled by default unless disabled using DisableNormalizing()
func (h *RequestHeader) EnableNormalizing() {
	h.disableNormalizing = false
}

// DisableNormalizing disables header names' normalization.
//
// By default all the header names are normalized by uppercasing
// the first letter and all the first letters following dashes,
// while lowercasing all the other letters.
// Examples:
//
//   - CONNECTION -> Connection
//   - conteNT-tYPE -> Content-Type
//   - foo-bar-baz -> Foo-Bar-Baz
//
// Disable header names' normalization only if know what are you doing.
func (h *ResponseHeader) DisableNormalizing() {
	h.disableNormalizing = true
}

// EnableNormalizing enables header names' normalization.
//
// Header names are normalized by uppercasing the first letter and
// all the first letters following dashes, while lowercasing all
// the other letters.
// Examples:
//
//   - CONNECTION -> Connection
//   - conteNT-tYPE -> Content-Type
//   - foo-bar-baz -> Foo-Bar-Baz
//
// This is enabled by default unless disabled using DisableNormalizing()
func (h *ResponseHeader) EnableNormalizing() {
	h.disableNormalizing = false
}

// SetNoDefaultContentType allows you to control if a default Content-Type header will be set (false) or not (true).
func (h *ResponseHeader) SetNoDefaultContentType(noDefaultContentType bool) {
	h.noDefaultContentType = noDefaultContentType
}

// Reset clears response header.
func (h *ResponseHeader) Reset() {
	h.disableNormalizing = false
	h.SetNoDefaultContentType(false)
	h.noDefaultDate = false
	h.resetSkipNormalize()
}

func (h *ResponseHeader) resetSkipNormalize() {
	h.noHTTP11 = false
	h.connectionClose = false

	h.statusCode = 0
	h.statusMessage = h.statusMessage[:0]
	h.protocol = h.protocol[:0]
	h.contentLength = 0
	h.contentLengthBytes = h.contentLengthBytes[:0]

	h.contentType = h.contentType[:0]
	h.contentEncoding = h.contentEncoding[:0]
	h.server = h.server[:0]

	h.h = h.h[:0]
	h.cookies = h.cookies[:0]
	h.trailer = h.trailer[:0]
	h.mulHeader = h.mulHeader[:0]
}

// SetNoDefaultContentType allows you to control if a default Content-Type header will be set (false) or not (true).
func (h *RequestHeader) SetNoDefaultContentType(noDefaultContentType bool) {
	h.noDefaultContentType = noDefaultContentType
}

// Reset clears request header.
func (h *RequestHeader) Reset() {
	h.disableNormalizing = false
	h.SetNoDefaultContentType(false)
	h.resetSkipNormalize()
}

func (h *RequestHeader) resetSkipNormalize() {
	h.noHTTP11 = false
	h.connectionClose = false

	h.contentLength = 0
	h.contentLengthBytes = h.contentLengthBytes[:0]

	h.method = h.method[:0]
	h.proto = h.proto[:0]
	h.requestURI = h.requestURI[:0]
	h.host = h.host[:0]
	h.contentType = h.contentType[:0]
	h.userAgent = h.userAgent[:0]
	h.trailer = h.trailer[:0]
	h.mulHeader = h.mulHeader[:0]

	h.h = h.h[:0]
	h.cookies = h.cookies[:0]
	h.cookiesCollected = false

	h.rawHeaders = h.rawHeaders[:0]
}

// CopyTo copies all the headers to dst.
func (h *ResponseHeader) CopyTo(dst *ResponseHeader) {
	dst.Reset()

	dst.disableNormalizing = h.disableNormalizing
	dst.noHTTP11 = h.noHTTP11
	dst.connectionClose = h.connectionClose
	dst.noDefaultContentType = h.noDefaultContentType
	dst.noDefaultDate = h.noDefaultDate

	dst.statusCode = h.statusCode
	dst.statusMessage = append(dst.statusMessage, h.statusMessage...)
	dst.protocol = append(dst.protocol, h.protocol...)
	dst.contentLength = h.contentLength
	dst.contentLengthBytes = append(dst.contentLengthBytes, h.contentLengthBytes...)
	dst.contentType = append(dst.contentType, h.contentType...)
	dst.contentEncoding = append(dst.contentEncoding, h.contentEncoding...)
	dst.server = append(dst.server, h.server...)
	dst.h = copyArgs(dst.h, h.h)
	dst.cookies = copyArgs(dst.cookies, h.cookies)
	dst.trailer = copyArgs(dst.trailer, h.trailer)
}

// CopyTo copies all the headers to dst.
func (h *RequestHeader) CopyTo(dst *RequestHeader) {
	dst.Reset()

	dst.disableNormalizing = h.disableNormalizing
	dst.noHTTP11 = h.noHTTP11
	dst.connectionClose = h.connectionClose
	dst.noDefaultContentType = h.noDefaultContentType

	dst.contentLength = h.contentLength
	dst.contentLengthBytes = append(dst.contentLengthBytes, h.contentLengthBytes...)
	dst.method = append(dst.method, h.method...)
	dst.proto = append(dst.proto, h.proto...)
	dst.requestURI = append(dst.requestURI, h.requestURI...)
	dst.host = append(dst.host, h.host...)
	dst.contentType = append(dst.contentType, h.contentType...)
	dst.userAgent = append(dst.userAgent, h.userAgent...)
	dst.trailer = append(dst.trailer, h.trailer...)
	dst.h = copyArgs(dst.h, h.h)
	dst.cookies = copyArgs(dst.cookies, h.cookies)
	dst.cookiesCollected = h.cookiesCollected
	dst.rawHeaders = append(dst.rawHeaders, h.rawHeaders...)
}

// VisitAll calls f for each header.
//
// f must not retain references to key and/or value after returning.
// Copy key and/or value contents before returning if you need retaining them.
func (h *ResponseHeader) VisitAll(f func(key, value []byte)) {
	if len(h.contentLengthBytes) > 0 {
		f(strContentLength, h.contentLengthBytes)
	}
	contentType := h.ContentType()
	if len(contentType) > 0 {
		f(strContentType, contentType)
	}
	contentEncoding := h.ContentEncoding()
	if len(contentEncoding) > 0 {
		f(strContentEncoding, contentEncoding)
	}
	server := h.Server()
	if len(server) > 0 {
		f(strServer, server)
	}
	if len(h.cookies) > 0 {
		visitArgs(h.cookies, func(_, v []byte) {
			f(strSetCookie, v)
		})
	}
	if len(h.trailer) > 0 {
		f(strTrailer, appendArgsKeyBytes(nil, h.trailer, strCommaSpace))
	}
	visitArgs(h.h, f)
	if h.ConnectionClose() {
		f(strConnection, strClose)
	}
}

// VisitAllTrailer calls f for each response Trailer.
//
// f must not retain references to value after returning.
func (h *ResponseHeader) VisitAllTrailer(f func(value []byte)) {
	visitArgsKey(h.trailer, f)
}

// VisitAllTrailer calls f for each request Trailer.
//
// f must not retain references to value after returning.
func (h *RequestHeader) VisitAllTrailer(f func(value []byte)) {
	visitArgsKey(h.trailer, f)
}

// VisitAllCookie calls f for each response cookie.
//
// Cookie name is passed in key and the whole Set-Cookie header value
// is passed in value on each f invocation. Value may be parsed
// with Cookie.ParseBytes().
//
// f must not retain references to key and/or value after returning.
func (h *ResponseHeader) VisitAllCookie(f func(key, value []byte)) {
	visitArgs(h.cookies, f)
}

// VisitAllCookie calls f for each request cookie.
//
// f must not retain references to key and/or value after returning.
func (h *RequestHeader) VisitAllCookie(f func(key, value []byte)) {
	h.collectCookies()
	visitArgs(h.cookies, f)
}

// VisitAll calls f for each header.
//
// f must not retain references to key and/or value after returning.
// Copy key and/or value contents before returning if you need retaining them.
//
// To get the headers in order they were received use VisitAllInOrder.
func (h *RequestHeader) VisitAll(f func(key, value []byte)) {
	host := h.Host()
	if len(host) > 0 {
		f(strHost, host)
	}
	if len(h.contentLengthBytes) > 0 {
		f(strContentLength, h.contentLengthBytes)
	}
	contentType := h.ContentType()
	if len(contentType) > 0 {
		f(strContentType, contentType)
	}
	userAgent := h.UserAgent()
	if len(userAgent) > 0 {
		f(strUserAgent, userAgent)
	}
	if len(h.trailer) > 0 {
		f(strTrailer, appendArgsKeyBytes(nil, h.trailer, strCommaSpace))
	}

	h.collectCookies()
	if len(h.cookies) > 0 {
		h.bufKV.value = appendRequestCookieBytes(h.bufKV.value[:0], h.cookies)
		f(strCookie, h.bufKV.value)
	}
	visitArgs(h.h, f)
	if h.ConnectionClose() {
		f(strConnection, strClose)
	}
}

// VisitAllInOrder calls f for each header in the order they were received.
//
// f must not retain references to key and/or value after returning.
// Copy key and/or value contents before returning if you need retaining them.
//
// This function is slightly slower than VisitAll because it has to reparse the
// raw headers to get the order.
func (h *RequestHeader) VisitAllInOrder(f func(key, value []byte)) {
	var s headerScanner
	s.b = h.rawHeaders
	s.disableNormalizing = h.disableNormalizing
	for s.next() {
		if len(s.key) > 0 {
			f(s.key, s.value)
		}
	}
}

// Del deletes header with the given key.
func (h *ResponseHeader) Del(key string) {
	k := getHeaderKeyBytes(&h.bufKV, key, h.disableNormalizing)
	h.del(k)
}

// DelBytes deletes header with the given key.
func (h *ResponseHeader) DelBytes(key []byte) {
	h.bufKV.key = append(h.bufKV.key[:0], key...)
	normalizeHeaderKey(h.bufKV.key, h.disableNormalizing)
	h.del(h.bufKV.key)
}

func (h *ResponseHeader) del(key []byte) {
	switch string(key) {
	case HeaderContentType:
		h.contentType = h.contentType[:0]
	case HeaderContentEncoding:
		h.contentEncoding = h.contentEncoding[:0]
	case HeaderServer:
		h.server = h.server[:0]
	case HeaderSetCookie:
		h.cookies = h.cookies[:0]
	case HeaderContentLength:
		h.contentLength = 0
		h.contentLengthBytes = h.contentLengthBytes[:0]
	case HeaderConnection:
		h.connectionClose = false
	case HeaderTrailer:
		h.trailer = h.trailer[:0]
	}
	h.h = delAllArgsBytes(h.h, key)
}

// Del deletes header with the given key.
func (h *RequestHeader) Del(key string) {
	k := getHeaderKeyBytes(&h.bufKV, key, h.disableNormalizing)
	h.del(k)
}

// DelBytes deletes header with the given key.
func (h *RequestHeader) DelBytes(key []byte) {
	h.bufKV.key = append(h.bufKV.key[:0], key...)
	normalizeHeaderKey(h.bufKV.key, h.disableNormalizing)
	h.del(h.bufKV.key)
}

func (h *RequestHeader) del(key []byte) {
	switch string(key) {
	case HeaderHost:
		h.host = h.host[:0]
	case HeaderContentType:
		h.contentType = h.contentType[:0]
	case HeaderUserAgent:
		h.userAgent = h.userAgent[:0]
	case HeaderCookie:
		h.cookies = h.cookies[:0]
	case HeaderContentLength:
		h.contentLength = 0
		h.contentLengthBytes = h.contentLengthBytes[:0]
	case HeaderConnection:
		h.connectionClose = false
	case HeaderTrailer:
		h.trailer = h.trailer[:0]
	}
	h.h = delAllArgsBytes(h.h, key)
}

// setSpecialHeader handles special headers and return true when a header is processed.
func (h *ResponseHeader) setSpecialHeader(key, value []byte) bool {
	if len(key) == 0 {
		return false
	}

	switch key[0] | 0x20 {
	case 'c':
		if caseInsensitiveCompare(strContentType, key) {
			h.SetContentTypeBytes(value)
			return true
		} else if caseInsensitiveCompare(strContentLength, key) {
			if contentLength, err := parseContentLength(value); err == nil {
				h.contentLength = contentLength
				h.contentLengthBytes = append(h.contentLengthBytes[:0], value...)
			}
			return true
		} else if caseInsensitiveCompare(strContentEncoding, key) {
			h.SetContentEncodingBytes(value)
			return true
		} else if caseInsensitiveCompare(strConnection, key) {
			if bytes.Equal(strClose, value) {
				h.SetConnectionClose()
			} else {
				h.ResetConnectionClose()
				h.setNonSpecial(key, value)
			}
			return true
		}
	case 's':
		if caseInsensitiveCompare(strServer, key) {
			h.SetServerBytes(value)
			return true
		} else if caseInsensitiveCompare(strSetCookie, key) {
			var kv *argsKV
			h.cookies, kv = allocArg(h.cookies)
			kv.key = getCookieKey(kv.key, value)
			kv.value = append(kv.value[:0], value...)
			return true
		}
	case 't':
		if caseInsensitiveCompare(strTransferEncoding, key) {
			// Transfer-Encoding is managed automatically.
			return true
		} else if caseInsensitiveCompare(strTrailer, key) {
			_ = h.SetTrailerBytes(value)
			return true
		}
	case 'd':
		if caseInsensitiveCompare(strDate, key) {
			// Date is managed automatically.
			return true
		}
	}

	return false
}

// setNonSpecial directly put into map i.e. not a basic header
func (h *ResponseHeader) setNonSpecial(key []byte, value []byte) {
	h.h = setArgBytes(h.h, key, value, argsHasValue)
}

// setSpecialHeader handles special headers and return true when a header is processed.
func (h *RequestHeader) setSpecialHeader(key, value []byte) bool {
	if len(key) == 0 {
		return false
	}

	switch key[0] | 0x20 {
	case 'c':
		if caseInsensitiveCompare(strContentType, key) {
			h.SetContentTypeBytes(value)
			return true
		} else if caseInsensitiveCompare(strContentLength, key) {
			if contentLength, err := parseContentLength(value); err == nil {
				h.contentLength = contentLength
				h.contentLengthBytes = append(h.contentLengthBytes[:0], value...)
			}
			return true
		} else if caseInsensitiveCompare(strConnection, key) {
			if bytes.Equal(strClose, value) {
				h.SetConnectionClose()
			} else {
				h.ResetConnectionClose()
				h.setNonSpecial(key, value)
			}
			return true
		} else if caseInsensitiveCompare(strCookie, key) {
			h.collectCookies()
			h.cookies = parseRequestCookies(h.cookies, value)
			return true
		}
	case 't':
		if caseInsensitiveCompare(strTransferEncoding, key) {
			// Transfer-Encoding is managed automatically.
			return true
		} else if caseInsensitiveCompare(strTrailer, key) {
			_ = h.SetTrailerBytes(value)
			return true
		}
	case 'h':
		if caseInsensitiveCompare(strHost, key) {
			h.SetHostBytes(value)
			return true
		}
	case 'u':
		if caseInsensitiveCompare(strUserAgent, key) {
			h.SetUserAgentBytes(value)
			return true
		}
	}

	return false
}

// setNonSpecial directly put into map i.e. not a basic header
func (h *RequestHeader) setNonSpecial(key []byte, value []byte) {
	h.h = setArgBytes(h.h, key, value, argsHasValue)
}

// Add adds the given 'key: value' header.
//
// Multiple headers with the same key may be added with this function.
// Use Set for setting a single header for the given key.
//
// the Content-Type, Content-Length, Connection, Server, Set-Cookie,
// Transfer-Encoding and Date headers can only be set once and will
// overwrite the previous value.
//
// If the header is set as a Trailer (forbidden trailers will not be set, see AddTrailer for more details),
// it will be sent after the chunked response body.
func (h *ResponseHeader) Add(key, value string) {
	h.AddBytesKV(s2b(key), s2b(value))
}

// AddBytesK adds the given 'key: value' header.
//
// Multiple headers with the same key may be added with this function.
// Use SetBytesK for setting a single header for the given key.
//
// the Content-Type, Content-Length, Connection, Server, Set-Cookie,
// Transfer-Encoding and Date headers can only be set once and will
// overwrite the previous value.
//
// If the header is set as a Trailer (forbidden trailers will not be set, see AddTrailer for more details),
// it will be sent after the chunked response body.
func (h *ResponseHeader) AddBytesK(key []byte, value string) {
	h.AddBytesKV(key, s2b(value))
}

// AddBytesV adds the given 'key: value' header.
//
// Multiple headers with the same key may be added with this function.
// Use SetBytesV for setting a single header for the given key.
//
// the Content-Type, Content-Length, Connection, Server, Set-Cookie,
// Transfer-Encoding and Date headers can only be set once and will
// overwrite the previous value.
//
// If the header is set as a Trailer (forbidden trailers will not be set, see AddTrailer for more details),
// it will be sent after the chunked response body.
func (h *ResponseHeader) AddBytesV(key string, value []byte) {
	h.AddBytesKV(s2b(key), value)
}

// AddBytesKV adds the given 'key: value' header.
//
// Multiple headers with the same key may be added with this function.
// Use SetBytesKV for setting a single header for the given key.
//
// the Content-Type, Content-Length, Connection, Server, Set-Cookie,
// Transfer-Encoding and Date headers can only be set once and will
// overwrite the previous value.
//
// If the header is set as a Trailer (forbidden trailers will not be set, see AddTrailer for more details),
// it will be sent after the chunked response body.
func (h *ResponseHeader) AddBytesKV(key, value []byte) {
	if h.setSpecialHeader(key, value) {
		return
	}

	k := getHeaderKeyBytes(&h.bufKV, b2s(key), h.disableNormalizing)
	h.h = appendArgBytes(h.h, k, value, argsHasValue)
}

// Set sets the given 'key: value' header.
//
// If the header is set as a Trailer (forbidden trailers will not be set, see SetTrailer for more details),
// it will be sent after the chunked response body.
//
// Use Add for setting multiple header values under the same key.
func (h *ResponseHeader) Set(key, value string) {
	initHeaderKV(&h.bufKV, key, value, h.disableNormalizing)
	h.SetCanonical(h.bufKV.key, h.bufKV.value)
}

// SetBytesK sets the given 'key: value' header.
//
// If the header is set as a Trailer (forbidden trailers will not be set, see SetTrailer for more details),
// it will be sent after the chunked response body.
//
// Use AddBytesK for setting multiple header values under the same key.
func (h *ResponseHeader) SetBytesK(key []byte, value string) {
	h.bufKV.value = append(h.bufKV.value[:0], value...)
	h.SetBytesKV(key, h.bufKV.value)
}

// SetBytesV sets the given 'key: value' header.
//
// If the header is set as a Trailer (forbidden trailers will not be set, see SetTrailer for more details),
// it will be sent after the chunked response body.
//
// Use AddBytesV for setting multiple header values under the same key.
func (h *ResponseHeader) SetBytesV(key string, value []byte) {
	k := getHeaderKeyBytes(&h.bufKV, key, h.disableNormalizing)
	h.SetCanonical(k, value)
}

// SetBytesKV sets the given 'key: value' header.
//
// If the header is set as a Trailer (forbidden trailers will not be set, see SetTrailer for more details),
// it will be sent after the chunked response body.
//
// Use AddBytesKV for setting multiple header values under the same key.
func (h *ResponseHeader) SetBytesKV(key, value []byte) {
	h.bufKV.key = append(h.bufKV.key[:0], key...)
	normalizeHeaderKey(h.bufKV.key, h.disableNormalizing)
	h.SetCanonical(h.bufKV.key, value)
}

// SetCanonical sets the given 'key: value' header assuming that
// key is in canonical form.
//
// If the header is set as a Trailer (forbidden trailers will not be set, see SetTrailer for more details),
// it will be sent after the chunked response body.
func (h *ResponseHeader) SetCanonical(key, value []byte) {
	if h.setSpecialHeader(key, value) {
		return
	}
	h.setNonSpecial(key, value)
}

// SetCookie sets the given response cookie.
//
// It is safe re-using the cookie after the function returns.
func (h *ResponseHeader) SetCookie(cookie *Cookie) {
	h.cookies = setArgBytes(h.cookies, cookie.Key(), cookie.Cookie(), argsHasValue)
}

// SetCookie sets 'key: value' cookies.
func (h *RequestHeader) SetCookie(key, value string) {
	h.collectCookies()
	h.cookies = setArg(h.cookies, key, value, argsHasValue)
}

// SetCookieBytesK sets 'key: value' cookies.
func (h *RequestHeader) SetCookieBytesK(key []byte, value string) {
	h.SetCookie(b2s(key), value)
}

// SetCookieBytesKV sets 'key: value' cookies.
func (h *RequestHeader) SetCookieBytesKV(key, value []byte) {
	h.SetCookie(b2s(key), b2s(value))
}

// DelClientCookie instructs the client to remove the given cookie.
// This doesn't work for a cookie with specific domain or path,
// you should delete it manually like:
//
//	c := AcquireCookie()
//	c.SetKey(key)
//	c.SetDomain("example.com")
//	c.SetPath("/path")
//	c.SetExpire(CookieExpireDelete)
//	h.SetCookie(c)
//	ReleaseCookie(c)
//
// Use DelCookie if you want just removing the cookie from response header.
func (h *ResponseHeader) DelClientCookie(key string) {
	h.DelCookie(key)

	c := AcquireCookie()
	c.SetKey(key)
	c.SetExpire(CookieExpireDelete)
	h.SetCookie(c)
	ReleaseCookie(c)
}

// DelClientCookieBytes instructs the client to remove the given cookie.
// This doesn't work for a cookie with specific domain or path,
// you should delete it manually like:
//
//	c := AcquireCookie()
//	c.SetKey(key)
//	c.SetDomain("example.com")
//	c.SetPath("/path")
//	c.SetExpire(CookieExpireDelete)
//	h.SetCookie(c)
//	ReleaseCookie(c)
//
// Use DelCookieBytes if you want just removing the cookie from response header.
func (h *ResponseHeader) DelClientCookieBytes(key []byte) {
	h.DelClientCookie(b2s(key))
}

// DelCookie removes cookie under the given key from response header.
//
// Note that DelCookie doesn't remove the cookie from the client.
// Use DelClientCookie instead.
func (h *ResponseHeader) DelCookie(key string) {
	h.cookies = delAllArgs(h.cookies, key)
}

// DelCookieBytes removes cookie under the given key from response header.
//
// Note that DelCookieBytes doesn't remove the cookie from the client.
// Use DelClientCookieBytes instead.
func (h *ResponseHeader) DelCookieBytes(key []byte) {
	h.DelCookie(b2s(key))
}

// DelCookie removes cookie under the given key.
func (h *RequestHeader) DelCookie(key string) {
	h.collectCookies()
	h.cookies = delAllArgs(h.cookies, key)
}

// DelCookieBytes removes cookie under the given key.
func (h *RequestHeader) DelCookieBytes(key []byte) {
	h.DelCookie(b2s(key))
}

// DelAllCookies removes all the cookies from response headers.
func (h *ResponseHeader) DelAllCookies() {
	h.cookies = h.cookies[:0]
}

// DelAllCookies removes all the cookies from request headers.
func (h *RequestHeader) DelAllCookies() {
	h.collectCookies()
	h.cookies = h.cookies[:0]
}

// Add adds the given 'key: value' header.
//
// Multiple headers with the same key may be added with this function.
// Use Set for setting a single header for the given key.
//
// If the header is set as a Trailer (forbidden trailers will not be set, see AddTrailer for more details),
// it will be sent after the chunked request body.
func (h *RequestHeader) Add(key, value string) {
	h.AddBytesKV(s2b(key), s2b(value))
}

// AddBytesK adds the given 'key: value' header.
//
// Multiple headers with the same key may be added with this function.
// Use SetBytesK for setting a single header for the given key.
//
// If the header is set as a Trailer (forbidden trailers will not be set, see AddTrailer for more details),
// it will be sent after the chunked request body.
func (h *RequestHeader) AddBytesK(key []byte, value string) {
	h.AddBytesKV(key, s2b(value))
}

// AddBytesV adds the given 'key: value' header.
//
// Multiple headers with the same key may be added with this function.
// Use SetBytesV for setting a single header for the given key.
//
// If the header is set as a Trailer (forbidden trailers will not be set, see AddTrailer for more details),
// it will be sent after the chunked request body.
func (h *RequestHeader) AddBytesV(key string, value []byte) {
	h.AddBytesKV(s2b(key), value)
}

// AddBytesKV adds the given 'key: value' header.
//
// Multiple headers with the same key may be added with this function.
// Use SetBytesKV for setting a single header for the given key.
//
// the Content-Type, Content-Length, Connection, Cookie,
// Transfer-Encoding, Host and User-Agent headers can only be set once
// and will overwrite the previous value.
//
// If the header is set as a Trailer (forbidden trailers will not be set, see AddTrailer for more details),
// it will be sent after the chunked request body.
func (h *RequestHeader) AddBytesKV(key, value []byte) {
	if h.setSpecialHeader(key, value) {
		return
	}

	k := getHeaderKeyBytes(&h.bufKV, b2s(key), h.disableNormalizing)
	h.h = appendArgBytes(h.h, k, value, argsHasValue)
}

// Set sets the given 'key: value' header.
//
// If the header is set as a Trailer (forbidden trailers will not be set, see SetTrailer for more details),
// it will be sent after the chunked request body.
//
// Use Add for setting multiple header values under the same key.
func (h *RequestHeader) Set(key, value string) {
	initHeaderKV(&h.bufKV, key, value, h.disableNormalizing)
	h.SetCanonical(h.bufKV.key, h.bufKV.value)
}

// SetBytesK sets the given 'key: value' header.
//
// If the header is set as a Trailer (forbidden trailers will not be set, see SetTrailer for more details),
// it will be sent after the chunked request body.
//
// Use AddBytesK for setting multiple header values under the same key.
func (h *RequestHeader) SetBytesK(key []byte, value string) {
	h.bufKV.value = append(h.bufKV.value[:0], value...)
	h.SetBytesKV(key, h.bufKV.value)
}

// SetBytesV sets the given 'key: value' header.
//
// If the header is set as a Trailer (forbidden trailers will not be set, see SetTrailer for more details),
// it will be sent after the chunked request body.
//
// Use AddBytesV for setting multiple header values under the same key.
func (h *RequestHeader) SetBytesV(key string, value []byte) {
	k := getHeaderKeyBytes(&h.bufKV, key, h.disableNormalizing)
	h.SetCanonical(k, value)
}

// SetBytesKV sets the given 'key: value' header.
//
// If the header is set as a Trailer (forbidden trailers will not be set, see SetTrailer for more details),
// it will be sent after the chunked request body.
//
// Use AddBytesKV for setting multiple header values under the same key.
func (h *RequestHeader) SetBytesKV(key, value []byte) {
	h.bufKV.key = append(h.bufKV.key[:0], key...)
	normalizeHeaderKey(h.bufKV.key, h.disableNormalizing)
	h.SetCanonical(h.bufKV.key, value)
}

// SetCanonical sets the given 'key: value' header assuming that
// key is in canonical form.
//
// If the header is set as a Trailer (forbidden trailers will not be set, see SetTrailer for more details),
// it will be sent after the chunked request body.
func (h *RequestHeader) SetCanonical(key, value []byte) {
	if h.setSpecialHeader(key, value) {
		return
	}
	h.setNonSpecial(key, value)
}

// Peek returns header value for the given key.
//
// The returned value is valid until the response is released,
// either though ReleaseResponse or your request handler returning.
// Do not store references to the returned value. Make copies instead.
func (h *ResponseHeader) Peek(key string) []byte {
	k := getHeaderKeyBytes(&h.bufKV, key, h.disableNormalizing)
	return h.peek(k)
}

// PeekBytes returns header value for the given key.
//
// The returned value is valid until the response is released,
// either though ReleaseResponse or your request handler returning.
// Do not store references to returned value. Make copies instead.
func (h *ResponseHeader) PeekBytes(key []byte) []byte {
	h.bufKV.key = append(h.bufKV.key[:0], key...)
	normalizeHeaderKey(h.bufKV.key, h.disableNormalizing)
	return h.peek(h.bufKV.key)
}

// Peek returns header value for the given key.
//
// The returned value is valid until the request is released,
// either though ReleaseRequest or your request handler returning.
// Do not store references to returned value. Make copies instead.
func (h *RequestHeader) Peek(key string) []byte {
	k := getHeaderKeyBytes(&h.bufKV, key, h.disableNormalizing)
	return h.peek(k)
}

// PeekBytes returns header value for the given key.
//
// The returned value is valid until the request is released,
// either though ReleaseRequest or your request handler returning.
// Do not store references to returned value. Make copies instead.
func (h *RequestHeader) PeekBytes(key []byte) []byte {
	h.bufKV.key = append(h.bufKV.key[:0], key...)
	normalizeHeaderKey(h.bufKV.key, h.disableNormalizing)
	return h.peek(h.bufKV.key)
}

func (h *ResponseHeader) peek(key []byte) []byte {
	switch string(key) {
	case HeaderContentType:
		return h.ContentType()
	case HeaderContentEncoding:
		return h.ContentEncoding()
	case HeaderServer:
		return h.Server()
	case HeaderConnection:
		if h.ConnectionClose() {
			return strClose
		}
		return peekArgBytes(h.h, key)
	case HeaderContentLength:
		return h.contentLengthBytes
	case HeaderSetCookie:
		return appendResponseCookieBytes(nil, h.cookies)
	case HeaderTrailer:
		return appendArgsKeyBytes(nil, h.trailer, strCommaSpace)
	default:
		return peekArgBytes(h.h, key)
	}
}

func (h *RequestHeader) peek(key []byte) []byte {
	switch string(key) {
	case HeaderHost:
		return h.Host()
	case HeaderContentType:
		return h.ContentType()
	case HeaderUserAgent:
		return h.UserAgent()
	case HeaderConnection:
		if h.ConnectionClose() {
			return strClose
		}
		return peekArgBytes(h.h, key)
	case HeaderContentLength:
		return h.contentLengthBytes
	case HeaderCookie:
		if h.cookiesCollected {
			return appendRequestCookieBytes(nil, h.cookies)
		}
		return peekArgBytes(h.h, key)
	case HeaderTrailer:
		return appendArgsKeyBytes(nil, h.trailer, strCommaSpace)
	default:
		return peekArgBytes(h.h, key)
	}
}

// PeekAll returns all header value for the given key.
//
// The returned value is valid until the request is released,
// either though ReleaseRequest or your request handler returning.
// Any future calls to the Peek* will modify the returned value.
// Do not store references to returned value. Make copies instead.
func (h *RequestHeader) PeekAll(key string) [][]byte {
	k := getHeaderKeyBytes(&h.bufKV, key, h.disableNormalizing)
	return h.peekAll(k)
}

func (h *RequestHeader) peekAll(key []byte) [][]byte {
	h.mulHeader = h.mulHeader[:0]
	switch string(key) {
	case HeaderHost:
		if host := h.Host(); len(host) > 0 {
			h.mulHeader = append(h.mulHeader, host)
		}
	case HeaderContentType:
		if contentType := h.ContentType(); len(contentType) > 0 {
			h.mulHeader = append(h.mulHeader, contentType)
		}
	case HeaderUserAgent:
		if ua := h.UserAgent(); len(ua) > 0 {
			h.mulHeader = append(h.mulHeader, ua)
		}
	case HeaderConnection:
		if h.ConnectionClose() {
			h.mulHeader = append(h.mulHeader, strClose)
		} else {
			h.mulHeader = peekAllArgBytesToDst(h.mulHeader, h.h, key)
		}
	case HeaderContentLength:
		h.mulHeader = append(h.mulHeader, h.contentLengthBytes)
	case HeaderCookie:
		if h.cookiesCollected {
			h.mulHeader = append(h.mulHeader, appendRequestCookieBytes(nil, h.cookies))
		} else {
			h.mulHeader = peekAllArgBytesToDst(h.mulHeader, h.h, key)
		}
	case HeaderTrailer:
		h.mulHeader = append(h.mulHeader, appendArgsKeyBytes(nil, h.trailer, strCommaSpace))
	default:
		h.mulHeader = peekAllArgBytesToDst(h.mulHeader, h.h, key)
	}
	return h.mulHeader
}

// PeekAll returns all header value for the given key.
//
// The returned value is valid until the request is released,
// either though ReleaseResponse or your request handler returning.
// Any future calls to the Peek* will modify the returned value.
// Do not store references to returned value. Make copies instead.
func (h *ResponseHeader) PeekAll(key string) [][]byte {
	k := getHeaderKeyBytes(&h.bufKV, key, h.disableNormalizing)
	return h.peekAll(k)
}

func (h *ResponseHeader) peekAll(key []byte) [][]byte {
	h.mulHeader = h.mulHeader[:0]
	switch string(key) {
	case HeaderContentType:
		if contentType := h.ContentType(); len(contentType) > 0 {
			h.mulHeader = append(h.mulHeader, contentType)
		}
	case HeaderContentEncoding:
		if contentEncoding := h.ContentEncoding(); len(contentEncoding) > 0 {
			h.mulHeader = append(h.mulHeader, contentEncoding)
		}
	case HeaderServer:
		if server := h.Server(); len(server) > 0 {
			h.mulHeader = append(h.mulHeader, server)
		}
	case HeaderConnection:
		if h.ConnectionClose() {
			h.mulHeader = append(h.mulHeader, strClose)
		} else {
			h.mulHeader = peekAllArgBytesToDst(h.mulHeader, h.h, key)
		}
	case HeaderContentLength:
		h.mulHeader = append(h.mulHeader, h.contentLengthBytes)
	case HeaderSetCookie:
		h.mulHeader = append(h.mulHeader, appendResponseCookieBytes(nil, h.cookies))
	case HeaderTrailer:
		h.mulHeader = append(h.mulHeader, appendArgsKeyBytes(nil, h.trailer, strCommaSpace))
	default:
		h.mulHeader = peekAllArgBytesToDst(h.mulHeader, h.h, key)
	}
	return h.mulHeader
}

// PeekKeys return all header keys.
//
// The returned value is valid until the request is released,
// either though ReleaseRequest or your request handler returning.
// Any future calls to the Peek* will modify the returned value.
// Do not store references to returned value. Make copies instead.
func (h *RequestHeader) PeekKeys() [][]byte {
	h.mulHeader = h.mulHeader[:0]
	h.mulHeader = peekArgsKeys(h.mulHeader, h.h)
	return h.mulHeader
}

// PeekTrailerKeys return all trailer keys.
//
// The returned value is valid until the request is released,
// either though ReleaseRequest or your request handler returning.
// Any future calls to the Peek* will modify the returned value.
// Do not store references to returned value. Make copies instead.
func (h *RequestHeader) PeekTrailerKeys() [][]byte {
	h.mulHeader = h.mulHeader[:0]
	h.mulHeader = peekArgsKeys(h.mulHeader, h.trailer)
	return h.mulHeader
}

// PeekKeys return all header keys.
//
// The returned value is valid until the request is released,
// either though ReleaseResponse or your request handler returning.
// Any future calls to the Peek* will modify the returned value.
// Do not store references to returned value. Make copies instead.
func (h *ResponseHeader) PeekKeys() [][]byte {
	h.mulHeader = h.mulHeader[:0]
	h.mulHeader = peekArgsKeys(h.mulHeader, h.h)
	return h.mulHeader
}

// PeekTrailerKeys return all trailer keys.
//
// The returned value is valid until the request is released,
// either though ReleaseResponse or your request handler returning.
// Any future calls to the Peek* will modify the returned value.
// Do not store references to returned value. Make copies instead.
func (h *ResponseHeader) PeekTrailerKeys() [][]byte {
	h.mulHeader = h.mulHeader[:0]
	h.mulHeader = peekArgsKeys(h.mulHeader, h.trailer)
	return h.mulHeader
}

// Cookie returns cookie for the given key.
func (h *RequestHeader) Cookie(key string) []byte {
	h.collectCookies()
	return peekArgStr(h.cookies, key)
}

// CookieBytes returns cookie for the given key.
func (h *RequestHeader) CookieBytes(key []byte) []byte {
	h.collectCookies()
	return peekArgBytes(h.cookies, key)
}

// Cookie fills cookie for the given cookie.Key.
//
// Returns false if cookie with the given cookie.Key is missing.
func (h *ResponseHeader) Cookie(cookie *Cookie) bool {
	v := peekArgBytes(h.cookies, cookie.Key())
	if v == nil {
		return false
	}
	cookie.ParseBytes(v) //nolint:errcheck
	return true
}

// Read reads response header from r.
//
// io.EOF is returned if r is closed before reading the first header byte.
func (h *ResponseHeader) Read(r *bufio.Reader) error {
	n := 1
	for {
		err := h.tryRead(r, n)
		if err == nil {
			return nil
		}
		if err != errNeedMore {
			h.resetSkipNormalize()
			return err
		}
		n = r.Buffered() + 1
	}
}

func (h *ResponseHeader) tryRead(r *bufio.Reader, n int) error {
	h.resetSkipNormalize()
	b, err := r.Peek(n)
	if len(b) == 0 {
		// Return ErrTimeout on any timeout.
		if x, ok := err.(interface{ Timeout() bool }); ok && x.Timeout() {
			return ErrTimeout
		}
		// treat all other errors on the first byte read as EOF
		if n == 1 || err == io.EOF {
			return io.EOF
		}

		// This is for go 1.6 bug. See https://github.com/golang/go/issues/14121 .
		if err == bufio.ErrBufferFull {
			if h.secureErrorLogMessage {
				return &ErrSmallBuffer{
					error: fmt.Errorf("error when reading response headers"),
				}
			}
			return &ErrSmallBuffer{
				error: fmt.Errorf("error when reading response headers: %w", errSmallBuffer),
			}
		}

		return fmt.Errorf("error when reading response headers: %w", err)
	}
	b = mustPeekBuffered(r)
	headersLen, errParse := h.parse(b)
	if errParse != nil {
		return headerError("response", err, errParse, b, h.secureErrorLogMessage)
	}
	mustDiscard(r, headersLen)
	return nil
}

// ReadTrailer reads response trailer header from r.
//
// io.EOF is returned if r is closed before reading the first byte.
func (h *ResponseHeader) ReadTrailer(r *bufio.Reader) error {
	n := 1
	for {
		err := h.tryReadTrailer(r, n)
		if err == nil {
			return nil
		}
		if err != errNeedMore {
			return err
		}
		n = r.Buffered() + 1
	}
}

func (h *ResponseHeader) tryReadTrailer(r *bufio.Reader, n int) error {
	b, err := r.Peek(n)
	if len(b) == 0 {
		// Return ErrTimeout on any timeout.
		if x, ok := err.(interface{ Timeout() bool }); ok && x.Timeout() {
			return ErrTimeout
		}

		if n == 1 || err == io.EOF {
			return io.EOF
		}

		// This is for go 1.6 bug. See https://github.com/golang/go/issues/14121 .
		if err == bufio.ErrBufferFull {
			if h.secureErrorLogMessage {
				return &ErrSmallBuffer{
					error: fmt.Errorf("error when reading response trailer"),
				}
			}
			return &ErrSmallBuffer{
				error: fmt.Errorf("error when reading response trailer: %w", errSmallBuffer),
			}
		}

		return fmt.Errorf("error when reading response trailer: %w", err)
	}
	b = mustPeekBuffered(r)
	headersLen, errParse := h.parseTrailer(b)
	if errParse != nil {
		if err == io.EOF {
			return err
		}
		return headerError("response", err, errParse, b, h.secureErrorLogMessage)
	}
	mustDiscard(r, headersLen)
	return nil
}

func headerError(typ string, err, errParse error, b []byte, secureErrorLogMessage bool) error {
	if errParse != errNeedMore {
		return headerErrorMsg(typ, errParse, b, secureErrorLogMessage)
	}
	if err == nil {
		return errNeedMore
	}

	// Buggy servers may leave trailing CRLFs after http body.
	// Treat this case as EOF.
	if isOnlyCRLF(b) {
		return io.EOF
	}

	if err != bufio.ErrBufferFull {
		return headerErrorMsg(typ, err, b, secureErrorLogMessage)
	}
	return &ErrSmallBuffer{
		error: headerErrorMsg(typ, errSmallBuffer, b, secureErrorLogMessage),
	}
}

func headerErrorMsg(typ string, err error, b []byte, secureErrorLogMessage bool) error {
	if secureErrorLogMessage {
		return fmt.Errorf("error when reading %s headers: %w. Buffer size=%d", typ, err, len(b))
	}
	return fmt.Errorf("error when reading %s headers: %w. Buffer size=%d, contents: %s", typ, err, len(b), bufferSnippet(b))
}

// Read reads request header from r.
//
// io.EOF is returned if r is closed before reading the first header byte.
func (h *RequestHeader) Read(r *bufio.Reader) error {
	return h.readLoop(r, true)
}

// readLoop reads request header from r optionally loops until it has enough data.
//
// io.EOF is returned if r is closed before reading the first header byte.
func (h *RequestHeader) readLoop(r *bufio.Reader, waitForMore bool) error {
	n := 1
	for {
		err := h.tryRead(r, n)
		if err == nil {
			return nil
		}
		if !waitForMore || err != errNeedMore {
			h.resetSkipNormalize()
			return err
		}
		n = r.Buffered() + 1
	}
}

// ReadTrailer reads request trailer header from r.
//
// io.EOF is returned if r is closed before reading the first byte.
func (h *RequestHeader) ReadTrailer(r *bufio.Reader) error {
	n := 1
	for {
		err := h.tryReadTrailer(r, n)
		if err == nil {
			return nil
		}
		if err != errNeedMore {
			return err
		}
		n = r.Buffered() + 1
	}
}

func (h *RequestHeader) tryReadTrailer(r *bufio.Reader, n int) error {
	b, err := r.Peek(n)
	if len(b) == 0 {
		// Return ErrTimeout on any timeout.
		if x, ok := err.(interface{ Timeout() bool }); ok && x.Timeout() {
			return ErrTimeout
		}

		if n == 1 || err == io.EOF {
			return io.EOF
		}

		// This is for go 1.6 bug. See https://github.com/golang/go/issues/14121 .
		if err == bufio.ErrBufferFull {
			if h.secureErrorLogMessage {
				return &ErrSmallBuffer{
					error: fmt.Errorf("error when reading request trailer"),
				}
			}
			return &ErrSmallBuffer{
				error: fmt.Errorf("error when reading request trailer: %w", errSmallBuffer),
			}
		}

		return fmt.Errorf("error when reading request trailer: %w", err)
	}
	b = mustPeekBuffered(r)
	headersLen, errParse := h.parseTrailer(b)
	if errParse != nil {
		if err == io.EOF {
			return err
		}
		return headerError("request", err, errParse, b, h.secureErrorLogMessage)
	}
	mustDiscard(r, headersLen)
	return nil
}

func (h *RequestHeader) tryRead(r *bufio.Reader, n int) error {
	h.resetSkipNormalize()
	b, err := r.Peek(n)
	if len(b) == 0 {
		if err == io.EOF {
			return err
		}

		if err == nil {
			panic("bufio.Reader.Peek() returned nil, nil")
		}

		// This is for go 1.6 bug. See https://github.com/golang/go/issues/14121 .
		if err == bufio.ErrBufferFull {
			return &ErrSmallBuffer{
				error: fmt.Errorf("error when reading request headers: %w (n=%d, r.Buffered()=%d)", errSmallBuffer, n, r.Buffered()),
			}
		}

		// n == 1 on the first read for the request.
		if n == 1 {
			// We didn't read a single byte.
			return ErrNothingRead{err}
		}

		return fmt.Errorf("error when reading request headers: %w", err)
	}
	b = mustPeekBuffered(r)
	headersLen, errParse := h.parse(b)
	if errParse != nil {
		return headerError("request", err, errParse, b, h.secureErrorLogMessage)
	}
	mustDiscard(r, headersLen)
	return nil
}

func bufferSnippet(b []byte) string {
	n := len(b)
	start := 200
	end := n - start
	if start >= end {
		start = n
		end = n
	}
	bStart, bEnd := b[:start], b[end:]
	if len(bEnd) == 0 {
		return fmt.Sprintf("%q", b)
	}
	return fmt.Sprintf("%q...%q", bStart, bEnd)
}

func isOnlyCRLF(b []byte) bool {
	for _, ch := range b {
		if ch != rChar && ch != nChar {
			return false
		}
	}
	return true
}

func updateServerDate() {
	refreshServerDate()
	go func() {
		for {
			time.Sleep(time.Second)
			refreshServerDate()
		}
	}()
}

var (
	serverDate     atomic.Value
	serverDateOnce sync.Once // serverDateOnce.Do(updateServerDate)
)

func refreshServerDate() {
	b := AppendHTTPDate(nil, time.Now())
	serverDate.Store(b)
}

// Write writes response header to w.
func (h *ResponseHeader) Write(w *bufio.Writer) error {
	_, err := w.Write(h.Header())
	return err
}

// WriteTo writes response header to w.
//
// WriteTo implements io.WriterTo interface.
func (h *ResponseHeader) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(h.Header())
	return int64(n), err
}

// Header returns response header representation.
//
// Headers that set as Trailer will not represent. Use TrailerHeader for trailers.
//
// The returned value is valid until the request is released,
// either though ReleaseRequest or your request handler returning.
// Do not store references to returned value. Make copies instead.
func (h *ResponseHeader) Header() []byte {
	h.bufKV.value = h.AppendBytes(h.bufKV.value[:0])
	return h.bufKV.value
}

// writeTrailer writes response trailer to w.
func (h *ResponseHeader) writeTrailer(w *bufio.Writer) error {
	_, err := w.Write(h.TrailerHeader())
	return err
}

// TrailerHeader returns response trailer header representation.
//
// Trailers will only be received with chunked transfer.
//
// The returned value is valid until the request is released,
// either though ReleaseRequest or your request handler returning.
// Do not store references to returned value. Make copies instead.
func (h *ResponseHeader) TrailerHeader() []byte {
	h.bufKV.value = h.bufKV.value[:0]
	for _, t := range h.trailer {
		value := h.peek(t.key)
		h.bufKV.value = appendHeaderLine(h.bufKV.value, t.key, value)
	}
	h.bufKV.value = append(h.bufKV.value, strCRLF...)
	return h.bufKV.value
}

// String returns response header representation.
func (h *ResponseHeader) String() string {
	return string(h.Header())
}

// appendStatusLine appends the response status line to dst and returns
// the extended dst.
func (h *ResponseHeader) appendStatusLine(dst []byte) []byte {
	statusCode := h.StatusCode()
	if statusCode < 0 {
		statusCode = StatusOK
	}
	return formatStatusLine(dst, h.Protocol(), statusCode, h.StatusMessage())
}

// AppendBytes appends response header representation to dst and returns
// the extended dst.
func (h *ResponseHeader) AppendBytes(dst []byte) []byte {
	dst = h.appendStatusLine(dst[:0])

	server := h.Server()
	if len(server) != 0 {
		dst = appendHeaderLine(dst, strServer, server)
	}

	if !h.noDefaultDate {
		serverDateOnce.Do(updateServerDate)
		dst = appendHeaderLine(dst, strDate, serverDate.Load().([]byte))
	}

	// Append Content-Type only for non-zero responses
	// or if it is explicitly set.
	// See https://github.com/valyala/fasthttp/issues/28 .
	if h.ContentLength() != 0 || len(h.contentType) > 0 {
		contentType := h.ContentType()
		if len(contentType) > 0 {
			dst = appendHeaderLine(dst, strContentType, contentType)
		}
	}
	contentEncoding := h.ContentEncoding()
	if len(contentEncoding) > 0 {
		dst = appendHeaderLine(dst, strContentEncoding, contentEncoding)
	}

	if len(h.contentLengthBytes) > 0 {
		dst = appendHeaderLine(dst, strContentLength, h.contentLengthBytes)
	}

	for i, n := 0, len(h.h); i < n; i++ {
		kv := &h.h[i]

		// Exclude trailer from header
		exclude := false
		for _, t := range h.trailer {
			if bytes.Equal(kv.key, t.key) {
				exclude = true
				break
			}
		}
		if !exclude && (h.noDefaultDate || !bytes.Equal(kv.key, strDate)) {
			dst = appendHeaderLine(dst, kv.key, kv.value)
		}
	}

	if len(h.trailer) > 0 {
		dst = appendHeaderLine(dst, strTrailer, appendArgsKeyBytes(nil, h.trailer, strCommaSpace))
	}

	n := len(h.cookies)
	if n > 0 {
		for i := 0; i < n; i++ {
			kv := &h.cookies[i]
			dst = appendHeaderLine(dst, strSetCookie, kv.value)
		}
	}

	if h.ConnectionClose() {
		dst = appendHeaderLine(dst, strConnection, strClose)
	}

	return append(dst, strCRLF...)
}

// Write writes request header to w.
func (h *RequestHeader) Write(w *bufio.Writer) error {
	_, err := w.Write(h.Header())
	return err
}

// WriteTo writes request header to w.
//
// WriteTo implements io.WriterTo interface.
func (h *RequestHeader) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(h.Header())
	return int64(n), err
}

// Header returns request header representation.
//
// Headers that set as Trailer will not represent. Use TrailerHeader for trailers.
//
// The returned value is valid until the request is released,
// either though ReleaseRequest or your request handler returning.
// Do not store references to returned value. Make copies instead.
func (h *RequestHeader) Header() []byte {
	h.bufKV.value = h.AppendBytes(h.bufKV.value[:0])
	return h.bufKV.value
}

// writeTrailer writes request trailer to w.
func (h *RequestHeader) writeTrailer(w *bufio.Writer) error {
	_, err := w.Write(h.TrailerHeader())
	return err
}

// TrailerHeader returns request trailer header representation.
//
// Trailers will only be received with chunked transfer.
//
// The returned value is valid until the request is released,
// either though ReleaseRequest or your request handler returning.
// Do not store references to returned value. Make copies instead.
func (h *RequestHeader) TrailerHeader() []byte {
	h.bufKV.value = h.bufKV.value[:0]
	for _, t := range h.trailer {
		value := h.peek(t.key)
		h.bufKV.value = appendHeaderLine(h.bufKV.value, t.key, value)
	}
	h.bufKV.value = append(h.bufKV.value, strCRLF...)
	return h.bufKV.value
}

// RawHeaders returns raw header key/value bytes.
//
// Depending on server configuration, header keys may be normalized to
// capital-case in place.
//
// This copy is set aside during parsing, so empty slice is returned for all
// cases where parsing did not happen. Similarly, request line is not stored
// during parsing and can not be returned.
//
// The slice is not safe to use after the handler returns.
func (h *RequestHeader) RawHeaders() []byte {
	return h.rawHeaders
}

// String returns request header representation.
func (h *RequestHeader) String() string {
	return string(h.Header())
}

// AppendBytes appends request header representation to dst and returns
// the extended dst.
func (h *RequestHeader) AppendBytes(dst []byte) []byte {
	dst = append(dst, h.Method()...)
	dst = append(dst, ' ')
	dst = append(dst, h.RequestURI()...)
	dst = append(dst, ' ')
	dst = append(dst, h.Protocol()...)
	dst = append(dst, strCRLF...)

	userAgent := h.UserAgent()
	if len(userAgent) > 0 {
		dst = appendHeaderLine(dst, strUserAgent, userAgent)
	}

	host := h.Host()
	if len(host) > 0 {
		dst = appendHeaderLine(dst, strHost, host)
	}

	contentType := h.ContentType()
	if !h.noDefaultContentType && len(contentType) == 0 && !h.ignoreBody() {
		contentType = strDefaultContentType
	}
	if len(contentType) > 0 {
		dst = appendHeaderLine(dst, strContentType, contentType)
	}
	if len(h.contentLengthBytes) > 0 {
		dst = appendHeaderLine(dst, strContentLength, h.contentLengthBytes)
	}

	for i, n := 0, len(h.h); i < n; i++ {
		kv := &h.h[i]
		// Exclude trailer from header
		exclude := false
		for _, t := range h.trailer {
			if bytes.Equal(kv.key, t.key) {
				exclude = true
				break
			}
		}
		if !exclude {
			dst = appendHeaderLine(dst, kv.key, kv.value)
		}
	}

	if len(h.trailer) > 0 {
		dst = appendHeaderLine(dst, strTrailer, appendArgsKeyBytes(nil, h.trailer, strCommaSpace))
	}

	// there is no need in h.collectCookies() here, since if cookies aren't collected yet,
	// they all are located in h.h.
	n := len(h.cookies)
	if n > 0 {
		dst = append(dst, strCookie...)
		dst = append(dst, strColonSpace...)
		dst = appendRequestCookieBytes(dst, h.cookies)
		dst = append(dst, strCRLF...)
	}

	if h.ConnectionClose() {
		dst = appendHeaderLine(dst, strConnection, strClose)
	}

	return append(dst, strCRLF...)
}

func appendHeaderLine(dst, key, value []byte) []byte {
	dst = append(dst, key...)
	dst = append(dst, strColonSpace...)
	dst = append(dst, value...)
	return append(dst, strCRLF...)
}

func (h *ResponseHeader) parse(buf []byte) (int, error) {
	m, err := h.parseFirstLine(buf)
	if err != nil {
		return 0, err
	}
	n, err := h.parseHeaders(buf[m:])
	if err != nil {
		return 0, err
	}
	return m + n, nil
}

func (h *ResponseHeader) parseTrailer(buf []byte) (int, error) {
	// Skip any 0 length chunk.
	if buf[0] == '0' {
		skip := len(strCRLF) + 1
		if len(buf) < skip {
			return 0, io.EOF
		}
		buf = buf[skip:]
	}

	var s headerScanner
	s.b = buf
	s.disableNormalizing = h.disableNormalizing
	var err error
	for s.next() {
		if len(s.key) > 0 {
			if bytes.IndexByte(s.key, ' ') != -1 || bytes.IndexByte(s.key, '\t') != -1 {
				err = fmt.Errorf("invalid trailer key %q", s.key)
				continue
			}
			// Forbidden by RFC 7230, section 4.1.2
			if isBadTrailer(s.key) {
				err = fmt.Errorf("forbidden trailer key %q", s.key)
				continue
			}
			h.h = appendArgBytes(h.h, s.key, s.value, argsHasValue)
		}
	}
	if s.err != nil {
		return 0, s.err
	}
	if err != nil {
		return 0, err
	}
	return s.hLen, nil
}

func (h *RequestHeader) ignoreBody() bool {
	return h.IsGet() || h.IsHead()
}

func (h *RequestHeader) parse(buf []byte) (int, error) {
	m, err := h.parseFirstLine(buf)
	if err != nil {
		return 0, err
	}

	h.rawHeaders, _, err = readRawHeaders(h.rawHeaders[:0], buf[m:])
	if err != nil {
		return 0, err
	}
	var n int
	n, err = h.parseHeaders(buf[m:])
	if err != nil {
		return 0, err
	}
	return m + n, nil
}

func (h *RequestHeader) parseTrailer(buf []byte) (int, error) {
	// Skip any 0 length chunk.
	if buf[0] == '0' {
		skip := len(strCRLF) + 1
		if len(buf) < skip {
			return 0, io.EOF
		}
		buf = buf[skip:]
	}

	var s headerScanner
	s.b = buf
	s.disableNormalizing = h.disableNormalizing
	var err error
	for s.next() {
		if len(s.key) > 0 {
			if bytes.IndexByte(s.key, ' ') != -1 || bytes.IndexByte(s.key, '\t') != -1 {
				err = fmt.Errorf("invalid trailer key %q", s.key)
				continue
			}
			// Forbidden by RFC 7230, section 4.1.2
			if isBadTrailer(s.key) {
				err = fmt.Errorf("forbidden trailer key %q", s.key)
				continue
			}
			h.h = appendArgBytes(h.h, s.key, s.value, argsHasValue)
		}
	}
	if s.err != nil {
		return 0, s.err
	}
	if err != nil {
		return 0, err
	}
	return s.hLen, nil
}

func isBadTrailer(key []byte) bool {
	if len(key) == 0 {
		return true
	}

	switch key[0] | 0x20 {
	case 'a':
		return caseInsensitiveCompare(key, strAuthorization)
	case 'c':
		if len(key) > len(HeaderContentType) && caseInsensitiveCompare(key[:8], strContentType[:8]) {
			// skip compare prefix 'Content-'
			return caseInsensitiveCompare(key[8:], strContentEncoding[8:]) ||
				caseInsensitiveCompare(key[8:], strContentLength[8:]) ||
				caseInsensitiveCompare(key[8:], strContentType[8:]) ||
				caseInsensitiveCompare(key[8:], strContentRange[8:])
		}
		return caseInsensitiveCompare(key, strConnection)
	case 'e':
		return caseInsensitiveCompare(key, strExpect)
	case 'h':
		return caseInsensitiveCompare(key, strHost)
	case 'k':
		return caseInsensitiveCompare(key, strKeepAlive)
	case 'm':
		return caseInsensitiveCompare(key, strMaxForwards)
	case 'p':
		if len(key) > len(HeaderProxyConnection) && caseInsensitiveCompare(key[:6], strProxyConnection[:6]) {
			// skip compare prefix 'Proxy-'
			return caseInsensitiveCompare(key[6:], strProxyConnection[6:]) ||
				caseInsensitiveCompare(key[6:], strProxyAuthenticate[6:]) ||
				caseInsensitiveCompare(key[6:], strProxyAuthorization[6:])
		}
	case 'r':
		return caseInsensitiveCompare(key, strRange)
	case 't':
		return caseInsensitiveCompare(key, strTE) ||
			caseInsensitiveCompare(key, strTrailer) ||
			caseInsensitiveCompare(key, strTransferEncoding)
	case 'w':
		return caseInsensitiveCompare(key, strWWWAuthenticate)
	}
	return false
}

func (h *ResponseHeader) parseFirstLine(buf []byte) (int, error) {
	bNext := buf
	var b []byte
	var err error
	for len(b) == 0 {
		if b, bNext, err = nextLine(bNext); err != nil {
			return 0, err
		}
	}

	// parse protocol
	n := bytes.IndexByte(b, ' ')
	if n < 0 {
		if h.secureErrorLogMessage {
			return 0, fmt.Errorf("cannot find whitespace in the first line of response")
		}
		return 0, fmt.Errorf("cannot find whitespace in the first line of response %q", buf)
	}
	h.noHTTP11 = !bytes.Equal(b[:n], strHTTP11)
	b = b[n+1:]

	// parse status code
	h.statusCode, n, err = parseUintBuf(b)
	if err != nil {
		if h.secureErrorLogMessage {
			return 0, fmt.Errorf("cannot parse response status code: %w", err)
		}
		return 0, fmt.Errorf("cannot parse response status code: %w. Response %q", err, buf)
	}
	if len(b) > n && b[n] != ' ' {
		if h.secureErrorLogMessage {
			return 0, fmt.Errorf("unexpected char at the end of status code")
		}
		return 0, fmt.Errorf("unexpected char at the end of status code. Response %q", buf)
	}
	if len(b) > n+1 {
		h.SetStatusMessage(b[n+1:])
	}

	return len(buf) - len(bNext), nil
}

func (h *RequestHeader) parseFirstLine(buf []byte) (int, error) {
	bNext := buf
	var b []byte
	var err error
	for len(b) == 0 {
		if b, bNext, err = nextLine(bNext); err != nil {
			return 0, err
		}
	}

	// parse method
	n := bytes.IndexByte(b, ' ')
	if n <= 0 {
		if h.secureErrorLogMessage {
			return 0, fmt.Errorf("cannot find http request method")
		}
		return 0, fmt.Errorf("cannot find http request method in %q", buf)
	}
	h.method = append(h.method[:0], b[:n]...)
	b = b[n+1:]

	protoStr := strHTTP11
	// parse requestURI
	n = bytes.LastIndexByte(b, ' ')
	if n < 0 {
		h.noHTTP11 = true
		n = len(b)
		protoStr = strHTTP10
	} else if n == 0 {
		if h.secureErrorLogMessage {
			return 0, fmt.Errorf("requestURI cannot be empty")
		}
		return 0, fmt.Errorf("requestURI cannot be empty in %q", buf)
	} else if !bytes.Equal(b[n+1:], strHTTP11) {
		h.noHTTP11 = true
		protoStr = b[n+1:]
	}

	h.proto = append(h.proto[:0], protoStr...)
	h.requestURI = append(h.requestURI[:0], b[:n]...)

	return len(buf) - len(bNext), nil
}

func readRawHeaders(dst, buf []byte) ([]byte, int, error) {
	n := bytes.IndexByte(buf, nChar)
	if n < 0 {
		return dst[:0], 0, errNeedMore
	}
	if (n == 1 && buf[0] == rChar) || n == 0 {
		// empty headers
		return dst, n + 1, nil
	}

	n++
	b := buf
	m := n
	for {
		b = b[m:]
		m = bytes.IndexByte(b, nChar)
		if m < 0 {
			return dst, 0, errNeedMore
		}
		m++
		n += m
		if (m == 2 && b[0] == rChar) || m == 1 {
			dst = append(dst, buf[:n]...)
			return dst, n, nil
		}
	}
}

func (h *ResponseHeader) parseHeaders(buf []byte) (int, error) {
	// 'identity' content-length by default
	h.contentLength = -2

	var s headerScanner
	s.b = buf
	s.disableNormalizing = h.disableNormalizing
	var err error
	var kv *argsKV
	for s.next() {
		if len(s.key) > 0 {
			switch s.key[0] | 0x20 {
			case 'c':
				if caseInsensitiveCompare(s.key, strContentType) {
					h.contentType = append(h.contentType[:0], s.value...)
					continue
				}
				if caseInsensitiveCompare(s.key, strContentEncoding) {
					h.contentEncoding = append(h.contentEncoding[:0], s.value...)
					continue
				}
				if caseInsensitiveCompare(s.key, strContentLength) {
					if h.contentLength != -1 {
						if h.contentLength, err = parseContentLength(s.value); err != nil {
							h.contentLength = -2
						} else {
							h.contentLengthBytes = append(h.contentLengthBytes[:0], s.value...)
						}
					}
					continue
				}
				if caseInsensitiveCompare(s.key, strConnection) {
					if bytes.Equal(s.value, strClose) {
						h.connectionClose = true
					} else {
						h.connectionClose = false
						h.h = appendArgBytes(h.h, s.key, s.value, argsHasValue)
					}
					continue
				}
			case 's':
				if caseInsensitiveCompare(s.key, strServer) {
					h.server = append(h.server[:0], s.value...)
					continue
				}
				if caseInsensitiveCompare(s.key, strSetCookie) {
					h.cookies, kv = allocArg(h.cookies)
					kv.key = getCookieKey(kv.key, s.value)
					kv.value = append(kv.value[:0], s.value...)
					continue
				}
			case 't':
				if caseInsensitiveCompare(s.key, strTransferEncoding) {
					if len(s.value) > 0 && !bytes.Equal(s.value, strIdentity) {
						h.contentLength = -1
						h.h = setArgBytes(h.h, strTransferEncoding, strChunked, argsHasValue)
					}
					continue
				}
				if caseInsensitiveCompare(s.key, strTrailer) {
					err = h.SetTrailerBytes(s.value)
					continue
				}
			}
			h.h = appendArgBytes(h.h, s.key, s.value, argsHasValue)
		}
	}
	if s.err != nil {
		h.connectionClose = true
		return 0, s.err
	}

	if h.contentLength < 0 {
		h.contentLengthBytes = h.contentLengthBytes[:0]
	}
	if h.contentLength == -2 && !h.ConnectionUpgrade() && !h.mustSkipContentLength() {
		h.h = setArgBytes(h.h, strTransferEncoding, strIdentity, argsHasValue)
		h.connectionClose = true
	}
	if h.noHTTP11 && !h.connectionClose {
		// close connection for non-http/1.1 response unless 'Connection: keep-alive' is set.
		v := peekArgBytes(h.h, strConnection)
		h.connectionClose = !hasHeaderValue(v, strKeepAlive)
	}

	return len(buf) - len(s.b), err
}

func (h *RequestHeader) parseHeaders(buf []byte) (int, error) {
	h.contentLength = -2

	var s headerScanner
	s.b = buf
	s.disableNormalizing = h.disableNormalizing
	var err error
	for s.next() {
		if len(s.key) > 0 {
			// Spaces between the header key and colon are not allowed.
			// See RFC 7230, Section 3.2.4.
			if bytes.IndexByte(s.key, ' ') != -1 || bytes.IndexByte(s.key, '\t') != -1 {
				err = fmt.Errorf("invalid header key %q", s.key)
				continue
			}

			switch s.key[0] | 0x20 {
			case 'h':
				if caseInsensitiveCompare(s.key, strHost) {
					h.host = append(h.host[:0], s.value...)
					continue
				}
			case 'u':
				if caseInsensitiveCompare(s.key, strUserAgent) {
					h.userAgent = append(h.userAgent[:0], s.value...)
					continue
				}
			case 'c':
				if caseInsensitiveCompare(s.key, strContentType) {
					h.contentType = append(h.contentType[:0], s.value...)
					continue
				}
				if caseInsensitiveCompare(s.key, strContentLength) {
					if h.contentLength != -1 {
						var nerr error
						if h.contentLength, nerr = parseContentLength(s.value); nerr != nil {
							if err == nil {
								err = nerr
							}
							h.contentLength = -2
						} else {
							h.contentLengthBytes = append(h.contentLengthBytes[:0], s.value...)
						}
					}
					continue
				}
				if caseInsensitiveCompare(s.key, strConnection) {
					if bytes.Equal(s.value, strClose) {
						h.connectionClose = true
					} else {
						h.connectionClose = false
						h.h = appendArgBytes(h.h, s.key, s.value, argsHasValue)
					}
					continue
				}
			case 't':
				if caseInsensitiveCompare(s.key, strTransferEncoding) {
					if !bytes.Equal(s.value, strIdentity) {
						h.contentLength = -1
						h.h = setArgBytes(h.h, strTransferEncoding, strChunked, argsHasValue)
					}
					continue
				}
				if caseInsensitiveCompare(s.key, strTrailer) {
					if nerr := h.SetTrailerBytes(s.value); nerr != nil {
						if err == nil {
							err = nerr
						}
					}
					continue
				}
			}
		}
		h.h = appendArgBytes(h.h, s.key, s.value, argsHasValue)
	}
	if s.err != nil && err == nil {
		err = s.err
	}
	if err != nil {
		h.connectionClose = true
		return 0, err
	}

	if h.contentLength < 0 {
		h.contentLengthBytes = h.contentLengthBytes[:0]
	}
	if h.noHTTP11 && !h.connectionClose {
		// close connection for non-http/1.1 request unless 'Connection: keep-alive' is set.
		v := peekArgBytes(h.h, strConnection)
		h.connectionClose = !hasHeaderValue(v, strKeepAlive)
	}
	return s.hLen, nil
}

func (h *RequestHeader) collectCookies() {
	if h.cookiesCollected {
		return
	}

	for i, n := 0, len(h.h); i < n; i++ {
		kv := &h.h[i]
		if caseInsensitiveCompare(kv.key, strCookie) {
			h.cookies = parseRequestCookies(h.cookies, kv.value)
			tmp := *kv
			copy(h.h[i:], h.h[i+1:])
			n--
			i--
			h.h[n] = tmp
			h.h = h.h[:n]
		}
	}
	h.cookiesCollected = true
}

var errNonNumericChars = errors.New("non-numeric chars found")

func parseContentLength(b []byte) (int, error) {
	v, n, err := parseUintBuf(b)
	if err != nil {
		return -1, fmt.Errorf("cannot parse Content-Length: %w", err)
	}
	if n != len(b) {
		return -1, fmt.Errorf("cannot parse Content-Length: %w", errNonNumericChars)
	}
	return v, nil
}

type headerScanner struct {
	b     []byte
	key   []byte
	value []byte
	err   error

	// hLen stores header subslice len
	hLen int

	disableNormalizing bool

	// by checking whether the next line contains a colon or not to tell
	// it's a header entry or a multi line value of current header entry.
	// the side effect of this operation is that we know the index of the
	// next colon and new line, so this can be used during next iteration,
	// instead of find them again.
	nextColon   int
	nextNewLine int

	initialized bool
}

func (s *headerScanner) next() bool {
	if !s.initialized {
		s.nextColon = -1
		s.nextNewLine = -1
		s.initialized = true
	}
	bLen := len(s.b)
	if bLen >= 2 && s.b[0] == rChar && s.b[1] == nChar {
		s.b = s.b[2:]
		s.hLen += 2
		return false
	}
	if bLen >= 1 && s.b[0] == nChar {
		s.b = s.b[1:]
		s.hLen++
		return false
	}
	var n int
	if s.nextColon >= 0 {
		n = s.nextColon
		s.nextColon = -1
	} else {
		n = bytes.IndexByte(s.b, ':')

		// There can't be a \n inside the header name, check for this.
		x := bytes.IndexByte(s.b, nChar)
		if x < 0 {
			// A header name should always at some point be followed by a \n
			// even if it's the one that terminates the header block.
			s.err = errNeedMore
			return false
		}
		if x < n {
			// There was a \n before the :
			s.err = errInvalidName
			return false
		}
	}
	if n < 0 {
		s.err = errNeedMore
		return false
	}
	s.key = s.b[:n]
	normalizeHeaderKey(s.key, s.disableNormalizing)
	n++
	for len(s.b) > n && s.b[n] == ' ' {
		n++
		// the newline index is a relative index, and lines below trimmed `s.b` by `n`,
		// so the relative newline index also shifted forward. it's safe to decrease
		// to a minus value, it means it's invalid, and will find the newline again.
		s.nextNewLine--
	}
	s.hLen += n
	s.b = s.b[n:]
	if s.nextNewLine >= 0 {
		n = s.nextNewLine
		s.nextNewLine = -1
	} else {
		n = bytes.IndexByte(s.b, nChar)
	}
	if n < 0 {
		s.err = errNeedMore
		return false
	}
	isMultiLineValue := false
	for {
		if n+1 >= len(s.b) {
			break
		}
		if s.b[n+1] != ' ' && s.b[n+1] != '\t' {
			break
		}
		d := bytes.IndexByte(s.b[n+1:], nChar)
		if d <= 0 {
			break
		} else if d == 1 && s.b[n+1] == rChar {
			break
		}
		e := n + d + 1
		if c := bytes.IndexByte(s.b[n+1:e], ':'); c >= 0 {
			s.nextColon = c
			s.nextNewLine = d - c - 1
			break
		}
		isMultiLineValue = true
		n = e
	}
	if n >= len(s.b) {
		s.err = errNeedMore
		return false
	}
	oldB := s.b
	s.value = s.b[:n]
	s.hLen += n + 1
	s.b = s.b[n+1:]

	if n > 0 && s.value[n-1] == rChar {
		n--
	}
	for n > 0 && s.value[n-1] == ' ' {
		n--
	}
	s.value = s.value[:n]
	if isMultiLineValue {
		s.value, s.b, s.hLen = normalizeHeaderValue(s.value, oldB, s.hLen)
	}
	return true
}

type headerValueScanner struct {
	b     []byte
	value []byte
}

func (s *headerValueScanner) next() bool {
	b := s.b
	if len(b) == 0 {
		return false
	}
	n := bytes.IndexByte(b, ',')
	if n < 0 {
		s.value = stripSpace(b)
		s.b = b[len(b):]
		return true
	}
	s.value = stripSpace(b[:n])
	s.b = b[n+1:]
	return true
}

func stripSpace(b []byte) []byte {
	for len(b) > 0 && b[0] == ' ' {
		b = b[1:]
	}
	for len(b) > 0 && b[len(b)-1] == ' ' {
		b = b[:len(b)-1]
	}
	return b
}

func hasHeaderValue(s, value []byte) bool {
	var vs headerValueScanner
	vs.b = s
	for vs.next() {
		if caseInsensitiveCompare(vs.value, value) {
			return true
		}
	}
	return false
}

func nextLine(b []byte) ([]byte, []byte, error) {
	nNext := bytes.IndexByte(b, nChar)
	if nNext < 0 {
		return nil, nil, errNeedMore
	}
	n := nNext
	if n > 0 && b[n-1] == rChar {
		n--
	}
	return b[:n], b[nNext+1:], nil
}

func initHeaderKV(kv *argsKV, key, value string, disableNormalizing bool) {
	kv.key = getHeaderKeyBytes(kv, key, disableNormalizing)
	// https://tools.ietf.org/html/rfc7230#section-3.2.4
	kv.value = append(kv.value[:0], value...)
	kv.value = removeNewLines(kv.value)
}

func getHeaderKeyBytes(kv *argsKV, key string, disableNormalizing bool) []byte {
	kv.key = append(kv.key[:0], key...)
	normalizeHeaderKey(kv.key, disableNormalizing)
	return kv.key
}

func normalizeHeaderValue(ov, ob []byte, headerLength int) (nv, nb []byte, nhl int) {
	nv = ov
	length := len(ov)
	if length <= 0 {
		return
	}
	write := 0
	shrunk := 0
	lineStart := false
	for read := 0; read < length; read++ {
		c := ov[read]
		if c == rChar || c == nChar {
			shrunk++
			if c == nChar {
				lineStart = true
			}
			continue
		} else if lineStart && c == '\t' {
			c = ' '
		} else {
			lineStart = false
		}
		nv[write] = c
		write++
	}

	nv = nv[:write]
	copy(ob[write:], ob[write+shrunk:])

	// Check if we need to skip \r\n or just \n
	skip := 0
	if ob[write] == rChar {
		if ob[write+1] == nChar {
			skip += 2
		} else {
			skip++
		}
	} else if ob[write] == nChar {
		skip++
	}

	nb = ob[write+skip : len(ob)-shrunk]
	nhl = headerLength - shrunk
	return
}

func normalizeHeaderKey(b []byte, disableNormalizing bool) {
	if disableNormalizing {
		return
	}

	n := len(b)
	if n == 0 {
		return
	}

	b[0] = toUpperTable[b[0]]
	for i := 1; i < n; i++ {
		p := &b[i]
		if *p == '-' {
			i++
			if i < n {
				b[i] = toUpperTable[b[i]]
			}
			continue
		}
		*p = toLowerTable[*p]
	}
}

// removeNewLines will replace `\r` and `\n` with an empty space
func removeNewLines(raw []byte) []byte {
	// check if a `\r` is present and save the position.
	// if no `\r` is found, check if a `\n` is present.
	foundR := bytes.IndexByte(raw, rChar)
	foundN := bytes.IndexByte(raw, nChar)
	start := 0

	if foundN != -1 {
		if foundR > foundN {
			start = foundN
		} else if foundR != -1 {
			start = foundR
		}
	} else if foundR != -1 {
		start = foundR
	} else {
		return raw
	}

	for i := start; i < len(raw); i++ {
		switch raw[i] {
		case rChar, nChar:
			raw[i] = ' '
		default:
			continue
		}
	}
	return raw
}

// AppendNormalizedHeaderKey appends normalized header key (name) to dst
// and returns the resulting dst.
//
// Normalized header key starts with uppercase letter. The first letters
// after dashes are also uppercased. All the other letters are lowercased.
// Examples:
//
//   - coNTENT-TYPe -> Content-Type
//   - HOST -> Host
//   - foo-bar-baz -> Foo-Bar-Baz
func AppendNormalizedHeaderKey(dst []byte, key string) []byte {
	dst = append(dst, key...)
	normalizeHeaderKey(dst[len(dst)-len(key):], false)
	return dst
}

// AppendNormalizedHeaderKeyBytes appends normalized header key (name) to dst
// and returns the resulting dst.
//
// Normalized header key starts with uppercase letter. The first letters
// after dashes are also uppercased. All the other letters are lowercased.
// Examples:
//
//   - coNTENT-TYPe -> Content-Type
//   - HOST -> Host
//   - foo-bar-baz -> Foo-Bar-Baz
func AppendNormalizedHeaderKeyBytes(dst, key []byte) []byte {
	return AppendNormalizedHeaderKey(dst, b2s(key))
}

func appendArgsKeyBytes(dst []byte, args []argsKV, sep []byte) []byte {
	for i, n := 0, len(args); i < n; i++ {
		kv := &args[i]
		dst = append(dst, kv.key...)
		if i+1 < n {
			dst = append(dst, sep...)
		}
	}
	return dst
}

var (
	errNeedMore    = errors.New("need more data: cannot find trailing lf")
	errInvalidName = errors.New("invalid header name")
	errSmallBuffer = errors.New("small read buffer. Increase ReadBufferSize")
)

// ErrNothingRead is returned when a keep-alive connection is closed,
// either because the remote closed it or because of a read timeout.
type ErrNothingRead struct {
	error
}

// ErrSmallBuffer is returned when the provided buffer size is too small
// for reading request and/or response headers.
//
// ReadBufferSize value from Server or clients should reduce the number
// of such errors.
type ErrSmallBuffer struct {
	error
}

func mustPeekBuffered(r *bufio.Reader) []byte {
	buf, err := r.Peek(r.Buffered())
	if len(buf) == 0 || err != nil {
		panic(fmt.Sprintf("bufio.Reader.Peek() returned unexpected data (%q, %v)", buf, err))
	}
	return buf
}

func mustDiscard(r *bufio.Reader, n int) {
	if _, err := r.Discard(n); err != nil {
		panic(fmt.Sprintf("bufio.Reader.Discard(%d) failed: %v", n, err))
	}
}
