package http

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"runtime"
	"strconv"
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
	var response = &Response{}
	response.setError(err)
	require.NotEqual(t, err.Error(), response.Error)
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
	var expectedError = "write: unknown errno `" + strconv.Itoa(int(syscall.EBFONT)) + "` on " + runtime.GOOS + " with message `" + err.Err.Error() + "`"
	var response = &Response{}
	response.setError(err)
	require.Equal(t, response.Error, expectedError)
}

func TestTCPErrors(t *testing.T) {
	var (
		nonTCPError       = new(net.OpError)
		econnreset        = new(net.OpError)
		epipeerror        = new(net.OpError)
		econnrefused      = new(net.OpError)
		tcperror          = new(net.OpError)
		timeoutedError    = new(net.OpError)
		notTimeoutedError = new(net.OpError)
	)
	nonTCPError.Net = "something"

	tcperror.Net = "tcp"

	econnreset.Net = "tcp"
	econnreset.Op = "write"
	econnreset.Err = syscall.ECONNRESET

	epipeerror.Net = "tcp"
	epipeerror.Op = "write"
	epipeerror.Err = syscall.EPIPE

	econnrefused.Net = "tcp"
	econnrefused.Op = "dial"
	econnrefused.Err = syscall.ECONNREFUSED

	timeoutedError.Net = "tcp"
	timeoutedError.Op = "dial"
	timeoutedError.Err = timeoutError(true)

	notTimeoutedError.Net = "tcp"
	notTimeoutedError.Op = "dial"
	notTimeoutedError.Err = timeoutError(false)

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
}

func testMapOfErrorCodes(t *testing.T, testTable map[errCode]error) {
	t.Helper()
	for code, err := range testTable {
		testErrorCode(t, code, err)
	}
}
