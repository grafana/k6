package brotli

import (
	"compress/gzip"
	"errors"
	"io"
	"net/http"

	"github.com/golang/gddo/httputil"
)

const (
	BestSpeed          = 0
	BestCompression    = 11
	DefaultCompression = 6
)

// WriterOptions configures Writer.
type WriterOptions struct {
	// Quality controls the compression-speed vs compression-density trade-offs.
	// The higher the quality, the slower the compression. Range is 0 to 11.
	Quality int
	// LGWin is the base 2 logarithm of the sliding window size.
	// Range is 10 to 24. 0 indicates automatic configuration based on Quality.
	LGWin int
}

var (
	errEncode       = errors.New("brotli: encode error")
	errWriterClosed = errors.New("brotli: Writer is closed")
)

// Writes to the returned writer are compressed and written to dst.
// It is the caller's responsibility to call Close on the Writer when done.
// Writes may be buffered and not flushed until Close.
func NewWriter(dst io.Writer) *Writer {
	return NewWriterLevel(dst, DefaultCompression)
}

// NewWriterLevel is like NewWriter but specifies the compression level instead
// of assuming DefaultCompression.
// The compression level can be DefaultCompression or any integer value between
// BestSpeed and BestCompression inclusive.
func NewWriterLevel(dst io.Writer, level int) *Writer {
	return NewWriterOptions(dst, WriterOptions{
		Quality: level,
	})
}

// NewWriterOptions is like NewWriter but specifies WriterOptions
func NewWriterOptions(dst io.Writer, options WriterOptions) *Writer {
	w := new(Writer)
	w.options = options
	w.Reset(dst)
	return w
}

// Reset discards the Writer's state and makes it equivalent to the result of
// its original state from NewWriter or NewWriterLevel, but writing to dst
// instead. This permits reusing a Writer rather than allocating a new one.
func (w *Writer) Reset(dst io.Writer) {
	encoderInitState(w)
	w.params.quality = w.options.Quality
	if w.options.LGWin > 0 {
		w.params.lgwin = uint(w.options.LGWin)
	}
	w.dst = dst
}

func (w *Writer) writeChunk(p []byte, op int) (n int, err error) {
	if w.dst == nil {
		return 0, errWriterClosed
	}

	for {
		availableIn := uint(len(p))
		nextIn := p
		success := encoderCompressStream(w, op, &availableIn, &nextIn)
		bytesConsumed := len(p) - int(availableIn)
		p = p[bytesConsumed:]
		n += bytesConsumed
		if !success {
			return n, errEncode
		}

		outputData := encoderTakeOutput(w)

		if len(outputData) > 0 {
			_, err = w.dst.Write(outputData)
			if err != nil {
				return n, err
			}
		}
		if len(p) == 0 {
			return n, nil
		}
	}
}

// Flush outputs encoded data for all input provided to Write. The resulting
// output can be decoded to match all input before Flush, but the stream is
// not yet complete until after Close.
// Flush has a negative impact on compression.
func (w *Writer) Flush() error {
	_, err := w.writeChunk(nil, operationFlush)
	return err
}

// Close flushes remaining data to the decorated writer.
func (w *Writer) Close() error {
	// If stream is already closed, it is reported by `writeChunk`.
	_, err := w.writeChunk(nil, operationFinish)
	w.dst = nil
	return err
}

// Write implements io.Writer. Flush or Close must be called to ensure that the
// encoded bytes are actually flushed to the underlying Writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	return w.writeChunk(p, operationProcess)
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

// HTTPCompressor chooses a compression method (brotli, gzip, or none) based on
// the Accept-Encoding header, sets the Content-Encoding header, and returns a
// WriteCloser that implements that compression. The Close method must be called
// before the current HTTP handler returns.
//
// Due to https://github.com/golang/go/issues/31753, the response will not be
// compressed unless you set a Content-Type header before you call
// HTTPCompressor.
func HTTPCompressor(w http.ResponseWriter, r *http.Request) io.WriteCloser {
	if w.Header().Get("Content-Type") == "" {
		return nopCloser{w}
	}

	if w.Header().Get("Vary") == "" {
		w.Header().Set("Vary", "Accept-Encoding")
	}

	encoding := httputil.NegotiateContentEncoding(r, []string{"br", "gzip"})
	switch encoding {
	case "br":
		w.Header().Set("Content-Encoding", "br")
		return NewWriter(w)
	case "gzip":
		w.Header().Set("Content-Encoding", "gzip")
		return gzip.NewWriter(w)
	}
	return nopCloser{w}
}
