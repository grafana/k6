package tcp

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/promises"
)

var (
	errNoActiveConnection  = errors.New("no active connection")
	errUnsupportedEncoding = errors.New("unsupported encoding")
	errWrite0Bytes         = errors.New("write returned 0 bytes written")
)

type writeOptions struct {
	Encoding string
	Tags     map[string]string
}

func (s *socket) write(data sobek.Value, opts *writeOptions) (*sobek.Promise, error) {
	promise, resolve, reject := promises.New(s.vu)

	dataBytes, opts, err := s.writePrepare(data, opts)
	if err != nil {
		s.rejectWithTCPError(reject, err, "write", addToTagSet(s.currentTags(), opts.Tags))

		return promise, nil
	}

	go func() {
		err := s.writeExecute(dataBytes, opts)
		if err != nil {
			reject(err)

			return
		}

		resolve(sobek.Undefined())
	}()

	return promise, nil
}

func (s *socket) writePrepare(input sobek.Value, opts *writeOptions) ([]byte, *writeOptions, error) {
	if opts == nil {
		opts = &writeOptions{}
	}

	data, err := stringOrArrayBuffer(input, opts.Encoding, s.vu.Runtime())
	if err != nil {
		return nil, opts, err
	}

	return data, opts, nil
}

func (s *socket) writeExecute(data []byte, opts *writeOptions) error {
	// Copy connection under lock to avoid holding the mutex during blocking I/O.
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if conn == nil {
		return errNoActiveConnection
	}

	s.log.Debug("Writing data to TCP socket")

	var result error

	// Write all data in fragments until complete or error occurs
	var totalWritten int
	for totalWritten < len(data) {
		n, err := conn.Write(data[totalWritten:])
		if err != nil {
			result = fmt.Errorf("failed to write data after %d bytes: %w", totalWritten, err)

			break
		}

		if n == 0 {
			result = fmt.Errorf("%w: after %d bytes", errWrite0Bytes, totalWritten)

			break
		}

		totalWritten += n

		atomic.AddInt64(&s.totalWritten, int64(n))

		s.log.WithField("bytes_written", n).WithField("total_written", totalWritten).Debug("TCP write fragment completed")
	}

	if result != nil {
		s.log.WithError(result).Error("TCP write failed")

		// Track partial write failures separately
		if totalWritten > 0 && totalWritten < len(data) {
			s.addCounterMetrics(s.metrics.tcpPartialWrites, addToTagSet(s.currentTags(), opts.Tags))
		}
	}

	s.addCounterMetrics(s.metrics.tcpWrites, addToTagSet(s.currentTags(), opts.Tags))

	return result
}

func stringOrArrayBuffer(input sobek.Value, encoding string, runtime *sobek.Runtime) ([]byte, error) {
	switch input.ExportType() {
	case reflect.TypeFor[string]():
		var str string

		if err := runtime.ExportTo(input, &str); err != nil {
			return nil, err
		}

		return decodeString(str, encoding)

	case reflect.TypeFor[[]byte]():
		var data []byte

		if err := runtime.ExportTo(input, &data); err != nil {
			return nil, err
		}

		return data, nil

	case reflect.TypeFor[sobek.ArrayBuffer]():
		var ab sobek.ArrayBuffer

		if err := runtime.ExportTo(input, &ab); err != nil {
			return nil, err
		}

		return ab.Bytes(), nil

	default:
		return nil, fmt.Errorf("%w: String or ArrayBuffer expected", errInvalidType)
	}
}

func decodeString(s, encoding string) ([]byte, error) {
	switch strings.ToLower(encoding) {
	case "", "utf8", "utf-8", "ascii":
		return []byte(s), nil
	case "base64":
		return base64.StdEncoding.DecodeString(s)
	case "base64url":
		return decodeBase64URLString(s)
	case "hex":
		return hex.DecodeString(s)
	default:
		return nil, fmt.Errorf("%w %q", errUnsupportedEncoding, encoding)
	}
}

func decodeBase64URLString(s string) ([]byte, error) {
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err == nil {
		return data, nil
	}

	return base64.URLEncoding.DecodeString(s)
}
