package http

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"syscall"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

func TestDefaultError(t *testing.T) {
	// TODO find better way to test
	testErrorCode(t, defaultErrorCode, fmt.Errorf("random error"))
}

func TestHTTP2Errors(t *testing.T) {
	var testTable = map[errCode]error{
		http2ConnectionErrorCode: new(http2.ConnectionError),
		http2StreamErrorCode:     new(http2.StreamError),
		http2GoAwayErrorCode:     new(http2.GoAwayError),
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

type timeoutError bool

func (t timeoutError) Timeout() bool {
	return (bool)(t)
}

func (t timeoutError) Error() string {
	return fmt.Sprintf("%t", t)
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
	require.Equalf(t, code, errorCodeForError(err), "Wrong error code for error `%s`", err)
	require.Equalf(t, code, errorCodeForError(errors.WithStack(err)), "Wrong error code for error `%s`", err)
}

func testMapOfErrorCodes(t *testing.T, testTable map[errCode]error) {
	t.Helper()
	for code, err := range testTable {
		testErrorCode(t, code, err)
	}
}
