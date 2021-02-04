/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package httpext

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"

	"github.com/loadimpact/k6/lib"
)

// CompressionType is used to specify what compression is to be used to compress the body of a
// request
// The conversion and validation methods are auto-generated with https://github.com/alvaroloes/enumer:
//nolint: lll
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
		var _, err = io.Copy(w, prevBuf)
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
		_, err := io.Copy(ioutil.Discard, resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			respErr = err
		}
		return nil, respErr
	}

	rc := &readCloser{resp.Body}
	// Ensure that the entire response body is read and closed, e.g. in case of decoding errors
	defer func(respBody io.ReadCloser) {
		_, _ = io.Copy(ioutil.Discard, respBody)
		_ = respBody.Close()
	}(resp.Body)

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
	buf := state.BPool.Get()
	defer state.BPool.Put(buf)
	buf.Reset()
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
