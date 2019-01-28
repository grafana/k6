package http

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"runtime"
	"syscall"
	"testing"

	"github.com/loadimpact/k6/lib/netext"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

func TestDefaultError(t *testing.T) {
	testErrorCode(t, defaultErrorCode, fmt.Errorf("random error"))
}

func TestHTTP2Errors(t *testing.T) {
	var unknownErrorCode = 220
	var connectionError = http2.ConnectionError(unknownErrorCode)
	var testTable = map[errCode]error{
		unknownHTTP2ConnectionErrorCode + 1: new(http2.ConnectionError),
		unknownHTTP2StreamErrorCode + 1:     new(http2.StreamError),
		unknownHTTP2GoAwayErrorCode + 1:     new(http2.GoAwayError),

		unknownHTTP2ConnectionErrorCode: &connectionError,
		unknownHTTP2StreamErrorCode:     &http2.StreamError{Code: 220},
		unknownHTTP2GoAwayErrorCode:     &http2.GoAwayError{ErrCode: 220},
	}
	testMapOfErrorCodes(t, testTable)
}

func TestTLSErrors(t *testing.T) {
	var testTable = map[errCode]error{
		x509UnknownAuthorityErrorCode: new(x509.UnknownAuthorityError),
		x509HostnameErrorCode:         new(x509.HostnameError),
		defaultTLSErrorCode:           new(tls.RecordHeaderError),
	}
	testMapOfErrorCodes(t, testTable)
}

func TestDNSErrors(t *testing.T) {
	var (
		defaultDNSError = new(net.DNSError)
		noSuchHostError = new(net.DNSError)
	)

	noSuchHostError.Err = "no such host" // defined as private in go stdlib
	var testTable = map[errCode]error{
		defaultDNSErrorCode:    defaultDNSError,
		dnsNoSuchHostErrorCode: noSuchHostError,
	}
	testMapOfErrorCodes(t, testTable)
}

func TestBlackListedIPError(t *testing.T) {
	var err = netext.BlackListedIPError{}
	testErrorCode(t, blackListedIPErrorCode, err)
	var errorCode, errorMsg = errorCodeForError(err)
	require.NotEqual(t, err.Error(), errorMsg)
	require.Equal(t, blackListedIPErrorCode, errorCode)
}

type timeoutError bool

func (t timeoutError) Timeout() bool {
	return (bool)(t)
}

func (t timeoutError) Error() string {
	return fmt.Sprintf("%t", t)
}

func TestUnknownNetErrno(t *testing.T) {
	var err = new(net.OpError)
	err.Op = "write"
	err.Net = "tcp"
	err.Err = syscall.EBFONT // Highly unlikely to actually need to do anything with this error
	var expectedError = fmt.Sprintf(
		"write: unknown errno `%d` on %s with message `%s`",
		syscall.EBFONT, runtime.GOOS, err.Err)
	var errorCode, errorMsg = errorCodeForError(err)
	require.Equal(t, expectedError, errorMsg)
	require.Equal(t, netUnknownErrnoErrorCode, errorCode)
}

func TestTCPErrors(t *testing.T) {
	var (
		nonTCPError       = &net.OpError{Net: "something", Err: errors.New("non tcp error")}
		econnreset        = &net.OpError{Net: "tcp", Op: "write", Err: syscall.ECONNRESET}
		epipeerror        = &net.OpError{Net: "tcp", Op: "write", Err: syscall.EPIPE}
		econnrefused      = &net.OpError{Net: "tcp", Op: "dial", Err: syscall.ECONNREFUSED}
		tcperror          = &net.OpError{Net: "tcp", Err: errors.New("tcp error")}
		timeoutedError    = &net.OpError{Net: "tcp", Op: "dial", Err: timeoutError(true)}
		notTimeoutedError = &net.OpError{Net: "tcp", Op: "dial", Err: timeoutError(false)}
	)

	var testTable = map[errCode]error{
		defaultNetNonTCPErrorCode: nonTCPError,
		tcpResetByPeerErrorCode:   econnreset,
		tcpBrokenPipeErrorCode:    epipeerror,
		tcpDialRefusedErrorCode:   econnrefused,
		defaultTCPErrorCode:       tcperror,
		tcpDialErrorCode:          notTimeoutedError,
		tcpDialTimeoutErrorCode:   timeoutedError,
	}

	testMapOfErrorCodes(t, testTable)
}

func testErrorCode(t *testing.T, code errCode, err error) {
	t.Helper()
	result, _ := errorCodeForError(err)
	require.Equalf(t, code, result, "Wrong error code for error `%s`", err)

	result, _ = errorCodeForError(errors.WithStack(err))
	require.Equalf(t, code, result, "Wrong error code for error `%s`", err)

	result, _ = errorCodeForError(&url.Error{Err: err})
	require.Equalf(t, code, result, "Wrong error code for error `%s`", err)
}

func testMapOfErrorCodes(t *testing.T, testTable map[errCode]error) {
	t.Helper()
	for code, err := range testTable {
		testErrorCode(t, code, err)
	}
}
