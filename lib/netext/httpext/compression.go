package httpext

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"

	"go.k6.io/k6/lib"
)

// CompressionType is used to specify what compression is to be used to compress the body of a
// request
// The conversion and validation methods are auto-generated with https://github.com/alvaroloes/enumer:
//
//go:generate enumer -type=CompressionType -transform=snake -trimprefix CompressionType -output compression_type_gen.go
type CompressionType uint

const (
	// CompressionTypeGzip compresses through gzip
	CompressionTypeGzip CompressionType = iota
	// CompressionTypeDeflate compresses through flate
	CompressionTypeDeflate
	// CompressionTypeZstd compresses through zstd
	CompressionTypeZstd
	// CompressionTypeBr compresses through brotli
	CompressionTypeBr
	// TODO: add compress(lzw), maybe bzip2 and others listed at
	// https://en.wikipedia.org/wiki/HTTP_compression#Content-Encoding_tokens
)

func compressBody(algos []CompressionType, body io.ReadCloser) (*bytes.Buffer, string, error) {
	var contentEncoding string
	var prevBuf io.Reader = body
	var buf *bytes.Buffer
	for _, compressionType := range algos {
		if buf != nil {
			prevBuf = buf
		}
		buf = new(bytes.Buffer)

		if contentEncoding != "" {
			contentEncoding += ", "
		}
		contentEncoding += compressionType.String()
		var w io.WriteCloser
		switch compressionType {
		case CompressionTypeGzip:
			w = gzip.NewWriter(buf)
		case CompressionTypeDeflate:
			w = zlib.NewWriter(buf)
		case CompressionTypeZstd:
			w, _ = zstd.NewWriter(buf)
		case CompressionTypeBr:
			w = brotli.NewWriter(buf)
		default:
			return nil, "", fmt.Errorf("unknown compressionType %s", compressionType)
		}
		// we don't close in defer because zlib will write it's checksum again if it closes twice :(
		_, err := io.Copy(w, prevBuf)
		if err != nil {
			_ = w.Close()
			return nil, "", err
		}

		if err = w.Close(); err != nil {
			return nil, "", err
		}
	}

	return buf, contentEncoding, body.Close()
}

//nolint:gochecknoglobals
var decompressionErrors = [...]error{
	zlib.ErrChecksum, zlib.ErrDictionary, zlib.ErrHeader,
	gzip.ErrChecksum, gzip.ErrHeader,
	// TODO: handle brotli errors - currently unexported
	zstd.ErrReservedBlockType, zstd.ErrCompressedSizeTooBig, zstd.ErrBlockTooSmall, zstd.ErrMagicMismatch,
	zstd.ErrWindowSizeExceeded, zstd.ErrWindowSizeTooSmall, zstd.ErrDecoderSizeExceeded, zstd.ErrUnknownDictionary,
	zstd.ErrFrameSizeExceeded, zstd.ErrCRCMismatch, zstd.ErrDecoderClosed,
}

func newDecompressionError(originalErr error) K6Error {
	return NewK6Error(
		responseDecompressionErrorCode,
		fmt.Sprintf("error decompressing response body (%s)", originalErr.Error()),
		originalErr,
	)
}

func wrapDecompressionError(err error) error {
	if err == nil {
		return nil
	}

	// TODO: something more optimized? for example, we won't get zstd errors if
	// we don't use it... maybe the code that builds the decompression readers
	// could also add an appropriate error-wrapper layer?
	for _, decErr := range &decompressionErrors {
		if err == decErr {
			return newDecompressionError(err)
		}
	}
	if strings.HasPrefix(err.Error(), "brotli: ") { // TODO: submit an upstream patch and fix...
		return newDecompressionError(err)
	}
	return err
}

func readResponseBody(
	state *lib.State,
	respType ResponseType,
	resp *http.Response,
	respErr error,
) (interface{}, error) {
	if resp == nil || respErr != nil {
		return nil, respErr
	}

	if respType == ResponseTypeNone {
		_, err := io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			respErr = err
		}
		return nil, respErr
	}

	rc := &readCloser{resp.Body}
	// Ensure that the entire response body is read and closed, e.g. in case of decoding errors
	defer func(respBody io.ReadCloser) {
		_, _ = io.Copy(io.Discard, respBody)
		_ = respBody.Close()
	}(resp.Body)

	if (resp.StatusCode >= 100 && resp.StatusCode <= 199) || // 1xx
		resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotModified {
		// for all three of this status code there is always no content
		// https://www.rfc-editor.org/rfc/rfc9110.html#section-6.4.1-8
		// this also prevents trying to read
		return nil, nil //nolint:nilnil
	}
	contentEncodings := strings.Split(resp.Header.Get("Content-Encoding"), ",")
	// Transparently decompress the body if it's has a content-encoding we
	// support. If not, simply return it as it is.
	for i := len(contentEncodings) - 1; i >= 0; i-- {
		contentEncoding := strings.TrimSpace(contentEncodings[i])
		if compression, err := CompressionTypeString(contentEncoding); err == nil {
			var decoder io.Reader
			var err error
			switch compression {
			case CompressionTypeDeflate:
				decoder, err = zlib.NewReader(rc)
			case CompressionTypeGzip:
				decoder, err = gzip.NewReader(rc)
			case CompressionTypeZstd:
				decoder, err = zstd.NewReader(rc)
			case CompressionTypeBr:
				decoder = brotli.NewReader(rc)
			default:
				// We have not implemented a compression ... :(
				err = fmt.Errorf(
					"unsupported compression type %s - this is a bug in k6, please report it",
					compression,
				)
			}
			if err != nil {
				return nil, newDecompressionError(err)
			}
			rc = &readCloser{decoder}
		}
	}

	buf := state.BufferPool.Get()
	defer state.BufferPool.Put(buf)
	_, err := io.Copy(buf, rc.Reader)
	if err != nil {
		respErr = wrapDecompressionError(err)
	}

	err = rc.Close()
	if err != nil && respErr == nil { // Don't overwrite previous errors
		respErr = wrapDecompressionError(err)
	}

	var result interface{}
	// Binary or string
	switch respType {
	case ResponseTypeText:
		result = buf.String()
	case ResponseTypeBinary:
		// Copy the data to a new slice before we return the buffer to the pool,
		// because buf.Bytes() points to the underlying buffer byte slice.
		// The ArrayBuffer wrapping will be done in the js/modules/k6/http
		// package to avoid a reverse dependency, since it depends on goja.
		binData := make([]byte, buf.Len())
		copy(binData, buf.Bytes())
		result = binData
	default:
		respErr = fmt.Errorf("unknown responseType %s", respType)
	}

	return result, respErr
}
