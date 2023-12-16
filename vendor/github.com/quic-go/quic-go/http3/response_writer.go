package http3

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/internal/utils"

	"github.com/quic-go/qpack"
)

// The maximum length of an encoded HTTP/3 frame header is 16:
// The frame has a type and length field, both QUIC varints (maximum 8 bytes in length)
const frameHeaderLen = 16

// headerWriter wraps the stream, so that the first Write call flushes the header to the stream
type headerWriter struct {
	str     quic.Stream
	header  http.Header
	status  int // status code passed to WriteHeader
	written bool

	logger utils.Logger
}

// writeHeader encodes and flush header to the stream
func (hw *headerWriter) writeHeader() error {
	var headers bytes.Buffer
	enc := qpack.NewEncoder(&headers)
	enc.WriteField(qpack.HeaderField{Name: ":status", Value: strconv.Itoa(hw.status)})

	for k, v := range hw.header {
		for index := range v {
			enc.WriteField(qpack.HeaderField{Name: strings.ToLower(k), Value: v[index]})
		}
	}

	buf := make([]byte, 0, frameHeaderLen+headers.Len())
	buf = (&headersFrame{Length: uint64(headers.Len())}).Append(buf)
	hw.logger.Infof("Responding with %d", hw.status)
	buf = append(buf, headers.Bytes()...)

	_, err := hw.str.Write(buf)
	return err
}

// first Write will trigger flushing header
func (hw *headerWriter) Write(p []byte) (int, error) {
	if !hw.written {
		if err := hw.writeHeader(); err != nil {
			return 0, err
		}
		hw.written = true
	}
	return hw.str.Write(p)
}

type responseWriter struct {
	*headerWriter
	conn        quic.Connection
	bufferedStr *bufio.Writer
	buf         []byte

	contentLen    int64 // if handler set valid Content-Length header
	numWritten    int64 // bytes written
	headerWritten bool
	isHead        bool
}

var (
	_ http.ResponseWriter = &responseWriter{}
	_ http.Flusher        = &responseWriter{}
	_ Hijacker            = &responseWriter{}
)

func newResponseWriter(str quic.Stream, conn quic.Connection, logger utils.Logger) *responseWriter {
	hw := &headerWriter{
		str:    str,
		header: http.Header{},
		logger: logger,
	}
	return &responseWriter{
		headerWriter: hw,
		buf:          make([]byte, frameHeaderLen),
		conn:         conn,
		bufferedStr:  bufio.NewWriter(hw),
	}
}

func (w *responseWriter) Header() http.Header {
	return w.header
}

func (w *responseWriter) WriteHeader(status int) {
	if w.headerWritten {
		return
	}

	// http status must be 3 digits
	if status < 100 || status > 999 {
		panic(fmt.Sprintf("invalid WriteHeader code %v", status))
	}

	if status >= 200 {
		w.headerWritten = true
		// Add Date header.
		// This is what the standard library does.
		// Can be disabled by setting the Date header to nil.
		if _, ok := w.header["Date"]; !ok {
			w.header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
		}
		// Content-Length checking
		// use ParseUint instead of ParseInt, as negative values are invalid
		if clen := w.header.Get("Content-Length"); clen != "" {
			if cl, err := strconv.ParseUint(clen, 10, 63); err == nil {
				w.contentLen = int64(cl)
			} else {
				// emit a warning for malformed Content-Length and remove it
				w.logger.Errorf("Malformed Content-Length %s", clen)
				w.header.Del("Content-Length")
			}
		}
	}
	w.status = status

	if !w.headerWritten {
		w.writeHeader()
	}
}

func (w *responseWriter) Write(p []byte) (int, error) {
	bodyAllowed := bodyAllowedForStatus(w.status)
	if !w.headerWritten {
		// If body is not allowed, we don't need to (and we can't) sniff the content type.
		if bodyAllowed {
			// If no content type, apply sniffing algorithm to body.
			// We can't use `w.header.Get` here since if the Content-Type was set to nil, we shoundn't do sniffing.
			_, haveType := w.header["Content-Type"]

			// If the Transfer-Encoding or Content-Encoding was set and is non-blank,
			// we shouldn't sniff the body.
			hasTE := w.header.Get("Transfer-Encoding") != ""
			hasCE := w.header.Get("Content-Encoding") != ""
			if !hasCE && !haveType && !hasTE && len(p) > 0 {
				w.header.Set("Content-Type", http.DetectContentType(p))
			}
		}
		w.WriteHeader(http.StatusOK)
		bodyAllowed = true
	}
	if !bodyAllowed {
		return 0, http.ErrBodyNotAllowed
	}

	w.numWritten += int64(len(p))
	if w.contentLen != 0 && w.numWritten > w.contentLen {
		return 0, http.ErrContentLength
	}

	if w.isHead {
		return len(p), nil
	}

	df := &dataFrame{Length: uint64(len(p))}
	w.buf = w.buf[:0]
	w.buf = df.Append(w.buf)
	if _, err := w.bufferedStr.Write(w.buf); err != nil {
		return 0, maybeReplaceError(err)
	}
	n, err := w.bufferedStr.Write(p)
	return n, maybeReplaceError(err)
}

func (w *responseWriter) FlushError() error {
	if !w.headerWritten {
		w.WriteHeader(http.StatusOK)
	}
	if !w.written {
		if err := w.writeHeader(); err != nil {
			return maybeReplaceError(err)
		}
		w.written = true
	}
	return w.bufferedStr.Flush()
}

func (w *responseWriter) Flush() {
	if err := w.FlushError(); err != nil {
		w.logger.Errorf("could not flush to stream: %s", err.Error())
	}
}

func (w *responseWriter) StreamCreator() StreamCreator {
	return w.conn
}

func (w *responseWriter) SetReadDeadline(deadline time.Time) error {
	return w.str.SetReadDeadline(deadline)
}

func (w *responseWriter) SetWriteDeadline(deadline time.Time) error {
	return w.str.SetWriteDeadline(deadline)
}

// copied from http2/http2.go
// bodyAllowedForStatus reports whether a given response status code
// permits a body. See RFC 2616, section 4.4.
func bodyAllowedForStatus(status int) bool {
	switch {
	case status >= 100 && status <= 199:
		return false
	case status == http.StatusNoContent:
		return false
	case status == http.StatusNotModified:
		return false
	}
	return true
}
