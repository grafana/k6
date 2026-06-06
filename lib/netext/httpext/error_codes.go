package httpext

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/net/http2"

	"go.k6.io/k6/v2/lib/netext"
)

// TODO: maybe rename the type errorCode, so we can have errCode variables? and
// also the constants would probably be better of if `ErrorCode` was a prefix,
// not a suffix - they would be much easier for auto-autocompletion at least...

type errCode uint32

const (
	// non specific
	defaultErrorCode          errCode = 1000
	defaultNetNonTCPErrorCode errCode = 1010
	invalidURLErrorCode       errCode = 1020
	requestTimeoutErrorCode   errCode = 1050
	// DNS errors
	defaultDNSErrorCode      errCode = 1100
	dnsNoSuchHostErrorCode   errCode = 1101
	blackListedIPErrorCode   errCode = 1110
	blockedHostnameErrorCode errCode = 1111
	// tcp errors
	defaultTCPErrorCode      errCode = 1200
	tcpBrokenPipeErrorCode   errCode = 1201
	netUnknownErrnoErrorCode errCode = 1202
	tcpDialErrorCode         errCode = 1210
	tcpDialTimeoutErrorCode  errCode = 1211
	tcpDialRefusedErrorCode  errCode = 1212
	tcpDialUnknownErrnoCode  errCode = 1213
	tcpResetByPeerErrorCode  errCode = 1220
	// TLS errors
	defaultTLSErrorCode           errCode = 1300
	tlsHeaderErrorCode            errCode = 1301
	x509UnknownAuthorityErrorCode errCode = 1310
	x509HostnameErrorCode         errCode = 1311

	// HTTP2 errors
	defaultHTTP2ErrorCode errCode = 1600 //nolint:unused
	// HTTP2 GoAway errors
	unknownHTTP2GoAwayErrorCode errCode = 1610
	// errors till 1611 + 13 are other HTTP2 GoAway errors with a specific errCode

	// HTTP2 Stream errors
	unknownHTTP2StreamErrorCode errCode = 1630
	// errors till 1631 + 13 are other HTTP2 Stream errors with a specific errCode

	// HTTP2 Connection errors
	unknownHTTP2ConnectionErrorCode errCode = 1650
	// errors till 1651 + 13 are other HTTP2 Connection errors with a specific errCode

	// Custom k6 content errors, i.e. when the magic fails
	// defaultContentError errCode = 1700 // reserved for future use
	responseDecompressionErrorCode errCode = 1701
)

const (
	tcpResetByPeerErrorCodeMsg  = "%s: connection reset by peer"
	tcpDialTimeoutErrorCodeMsg  = "dial: i/o timeout"
	tcpDialRefusedErrorCodeMsg  = "dial: connection refused"
	tcpBrokenPipeErrorCodeMsg   = "%s: broken pipe"
	netUnknownErrnoErrorCodeMsg = "%s: unknown errno `%d` on %s with message `%s`"
	dnsNoSuchHostErrorCodeMsg   = "lookup: no such host"
	blackListedIPErrorCodeMsg   = "ip is blacklisted"
	blockedHostnameErrorMsg     = "hostname is blocked"
	http2GoAwayErrorCodeMsg     = "http2: received GoAway with http2 ErrCode %s"
	http2StreamErrorCodeMsg     = "http2: stream error with http2 ErrCode %s"
	http2ConnectionErrorCodeMsg = "http2: connection error with http2 ErrCode %s"
	x509HostnameErrorCodeMsg    = "x509: certificate doesn't match hostname"
	x509UnknownAuthority        = "x509: unknown authority"
	requestTimeoutErrorCodeMsg  = "request timeout"
	invalidURLErrorCodeMsg      = "invalid URL"
)

func http2ErrCodeOffset(code http2.ErrCode) errCode {
	if code > http2.ErrCodeHTTP11Required {
		return 0
	}
	return 1 + errCode(code)
}

// http2ErrCodeByName maps the textual form produced by http2.ErrCode.String()
// (e.g. "PROTOCOL_ERROR") back to its code. Only the spec-defined codes are
// recognized; anything else returns false.
func http2ErrCodeByName(name string) (http2.ErrCode, bool) {
	for code := http2.ErrCodeNo; code <= http2.ErrCodeHTTP11Required; code++ {
		if code.String() == name {
			return code, true
		}
	}
	return 0, false
}

// errorCodeForHTTP2Message classifies an HTTP/2 connection or GOAWAY error by
// the text of its Error() method, as a fallback to the type switch in
// errorCodeForError.
//
// Since Go 1.27, golang.org/x/net/http2 compiles a "wrapping implementation"
// (build tag `go1.27 && !http2legacy`) that delegates to net/http. HTTP/2
// errors then surface as values of the unexported net/http/internal/http2
// connection- and GOAWAY-error types, which the http2.{ConnectionError,
// GoAwayError} cases in errorCodeForError cannot match and which - unlike
// http2.StreamError - have no errors.As bridge to the x/net types. Their
// Error() text is identical to x/net's, so we match on that instead. Only
// spec-defined error codes are recognized; an unknown code falls through.
func errorCodeForHTTP2Message(msg string) (errCode, string, bool) {
	switch {
	case strings.HasPrefix(msg, "connection error: "):
		name := strings.TrimPrefix(msg, "connection error: ")
		if code, ok := http2ErrCodeByName(name); ok {
			return unknownHTTP2ConnectionErrorCode + http2ErrCodeOffset(code),
				fmt.Sprintf(http2ConnectionErrorCodeMsg, code), true
		}
	case strings.HasPrefix(msg, "http2: server sent GOAWAY"):
		if name, ok := stringBetween(msg, "ErrCode=", ","); ok {
			if code, ok := http2ErrCodeByName(name); ok {
				return unknownHTTP2GoAwayErrorCode + http2ErrCodeOffset(code),
					fmt.Sprintf(http2GoAwayErrorCodeMsg, code), true
			}
		}
	}
	return 0, "", false
}

// stringBetween returns the substring of s found between the first occurrence
// of prefix and the next occurrence of sep that follows it.
func stringBetween(s, prefix, sep string) (string, bool) {
	_, after, found := strings.Cut(s, prefix)
	if !found {
		return "", false
	}
	mid, _, found := strings.Cut(after, sep)
	if !found {
		return "", false
	}
	return mid, true
}

//nolint:errorlint
func errorCodeForNetOpError(err *net.OpError) (errCode, string) {
	// TODO: refactor this further - a big switch would be more readable, maybe
	// we should even check for *os.SyscallError in the main switch body in the
	// parent errorCodeForError() function?

	if err.Net != "tcp" && err.Net != "tcp6" {
		// TODO: figure out how this happens
		return defaultNetNonTCPErrorCode, err.Error()
	}
	if sErr, ok := err.Err.(*os.SyscallError); ok {
		switch sErr.Unwrap() {
		case syscall.ECONNRESET:
			return tcpResetByPeerErrorCode, fmt.Sprintf(tcpResetByPeerErrorCodeMsg, err.Op)
		case syscall.EPIPE:
			return tcpBrokenPipeErrorCode, fmt.Sprintf(tcpBrokenPipeErrorCodeMsg, err.Op)
		}
		code, msg := getOSSyscallErrorCode(err, sErr)
		if code != 0 {
			return code, msg
		}
	}
	if err.Op != "dial" {
		switch inErr := err.Err.(type) {
		case syscall.Errno:
			return netUnknownErrnoErrorCode,
				fmt.Sprintf(netUnknownErrnoErrorCodeMsg,
					err.Op, (int)(inErr), runtime.GOOS, inErr.Error()) //nolint:gosec
		default:
			return defaultTCPErrorCode, err.Error()
		}
	}

	if iErr, ok := err.Err.(*os.SyscallError); ok {
		if errno, ok := iErr.Err.(syscall.Errno); ok {
			if errno == syscall.ECONNREFUSED ||
				// 10061 is some connection refused like thing on windows
				// TODO: fix by moving to x/sys instead of syscall after
				// https://github.com/golang/go/issues/31360 gets resolved
				(errno == 10061 && runtime.GOOS == "windows") {
				return tcpDialRefusedErrorCode, tcpDialRefusedErrorCodeMsg
			}
			return tcpDialUnknownErrnoCode,
				fmt.Sprintf("dial: unknown errno %d error with msg `%s`", errno, iErr.Err)
		}
	}

	// Check if the wrapped error isn't something we recognize, e.g. a DNS error
	if wrappedErr := errors.Unwrap(err); wrappedErr != nil {
		errCodeForWrapped, errForWrapped := errorCodeForError(wrappedErr)
		if errCodeForWrapped != defaultErrorCode {
			return errCodeForWrapped, errForWrapped
		}
	}

	// If it's not, return a generic TCP dial error
	return tcpDialErrorCode, err.Error()
}

// errorCodeForError returns the errorCode and a specific error message for given error.
//
//nolint:errorlint
func errorCodeForError(err error) (errCode, string) {
	// We explicitly check for `Unwrap()` in the default switch branch, but
	// checking for the concrete error types first gives us the opportunity to
	// also directly detect high-level errors, if we need to, even if they wrap
	// a low level error inside.
	switch e := err.(type) {
	case K6Error:
		return e.Code, e.Message
	case *net.DNSError:
		switch e.Err {
		case "no such host": // defined as private in the go stdlib
			return dnsNoSuchHostErrorCode, dnsNoSuchHostErrorCodeMsg
		default:
			return defaultDNSErrorCode, err.Error()
		}
	case netext.BlackListedIPError:
		return blackListedIPErrorCode, blackListedIPErrorCodeMsg
	case netext.BlockedHostError:
		return blockedHostnameErrorCode, blockedHostnameErrorMsg
	case http2.GoAwayError:
		return unknownHTTP2GoAwayErrorCode + http2ErrCodeOffset(e.ErrCode),
			fmt.Sprintf(http2GoAwayErrorCodeMsg, e.ErrCode)
	case http2.StreamError:
		return unknownHTTP2StreamErrorCode + http2ErrCodeOffset(e.Code),
			fmt.Sprintf(http2StreamErrorCodeMsg, e.Code)
	case http2.ConnectionError:
		return unknownHTTP2ConnectionErrorCode + http2ErrCodeOffset(http2.ErrCode(e)),
			fmt.Sprintf(http2ConnectionErrorCodeMsg, http2.ErrCode(e))
	case *net.OpError:
		return errorCodeForNetOpError(e)
	case x509.UnknownAuthorityError:
		return x509UnknownAuthorityErrorCode, x509UnknownAuthority
	case x509.HostnameError:
		return x509HostnameErrorCode, x509HostnameErrorCodeMsg
	case tls.RecordHeaderError:
		return tlsHeaderErrorCode, err.Error()
	case *url.Error:
		return errorCodeForError(e.Err)
	default:
		// Go 1.27+ fallback: x/net/http2's wrapping implementation surfaces HTTP/2
		// errors as unexported net/http/internal/http2 values that don't match the
		// http2.* cases above. StreamError can still be recovered via errors.As (the
		// standard-library type implements an As bridge to x/net's StreamError);
		// ConnectionError and GoAwayError have no such bridge and are matched by
		// their (implementation-independent) Error() text instead.
		var streamErr http2.StreamError
		if errors.As(err, &streamErr) {
			return unknownHTTP2StreamErrorCode + http2ErrCodeOffset(streamErr.Code),
				fmt.Sprintf(http2StreamErrorCodeMsg, streamErr.Code)
		}
		if code, msg, ok := errorCodeForHTTP2Message(err.Error()); ok {
			return code, msg
		}
		if wrappedErr := errors.Unwrap(err); wrappedErr != nil {
			return errorCodeForError(wrappedErr)
		}

		return defaultErrorCode, err.Error()
	}
}

// K6Error is a helper struct that enhances Go errors with custom k6-specific
// error-codes and more user-readable error messages.
type K6Error struct {
	Code          errCode
	Message       string
	OriginalError error
}

// NewK6Error is the constructor for K6Error
func NewK6Error(code errCode, msg string, originalErr error) K6Error {
	return K6Error{code, msg, originalErr}
}

// Error implements the `error` interface, so K6Errors are normal Go errors.
func (k6Err K6Error) Error() string {
	return k6Err.Message
}

// Unwrap implements the `xerrors.Wrapper` interface, so K6Errors are a bit
// future-proof Go 2 errors.
func (k6Err K6Error) Unwrap() error {
	return k6Err.OriginalError
}
