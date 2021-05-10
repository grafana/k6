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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"

	"go.k6.io/k6/lib/netext"
)

func TestDefaultError(t *testing.T) {
	t.Parallel()
	testErrorCode(t, defaultErrorCode, fmt.Errorf("random error"))
}

func TestHTTP2Errors(t *testing.T) {
	t.Parallel()
	unknownErrorCode := 220
	connectionError := http2.ConnectionError(unknownErrorCode)
	testTable := map[errCode]error{
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
	t.Parallel()
	testTable := map[errCode]error{
		x509UnknownAuthorityErrorCode: new(x509.UnknownAuthorityError),
		x509HostnameErrorCode:         new(x509.HostnameError),
		defaultTLSErrorCode:           new(tls.RecordHeaderError),
	}
	testMapOfErrorCodes(t, testTable)
}

func TestDNSErrors(t *testing.T) {
	t.Parallel()
	var (
		defaultDNSError = new(net.DNSError)
		noSuchHostError = new(net.DNSError)
	)

	noSuchHostError.Err = "no such host" // defined as private in go stdlib
	testTable := map[errCode]error{
		defaultDNSErrorCode:    defaultDNSError,
		dnsNoSuchHostErrorCode: noSuchHostError,
	}
	testMapOfErrorCodes(t, testTable)
}

func TestBlackListedIPError(t *testing.T) {
	t.Parallel()
	err := netext.BlackListedIPError{}
	testErrorCode(t, blackListedIPErrorCode, err)
	errorCode, errorMsg := errorCodeForError(err)
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
	t.Parallel()
	err := new(net.OpError)
	err.Op = "write"
	err.Net = "tcp"
	err.Err = syscall.ENOTRECOVERABLE // Highly unlikely to actually need to do anything with this error
	expectedError := fmt.Sprintf(
		"write: unknown errno `%d` on %s with message `%s`",
		syscall.ENOTRECOVERABLE, runtime.GOOS, err.Err)
	errorCode, errorMsg := errorCodeForError(err)
	require.Equal(t, expectedError, errorMsg)
	require.Equal(t, netUnknownErrnoErrorCode, errorCode)
}

func TestTCPErrors(t *testing.T) {
	t.Parallel()
	var (
		nonTCPError       = &net.OpError{Net: "something", Err: errors.New("non tcp error")}
		econnreset        = &net.OpError{Net: "tcp", Op: "write", Err: &os.SyscallError{Err: syscall.ECONNRESET}}
		epipeerror        = &net.OpError{Net: "tcp", Op: "write", Err: &os.SyscallError{Err: syscall.EPIPE}}
		econnrefused      = &net.OpError{Net: "tcp", Op: "dial", Err: &os.SyscallError{Err: syscall.ECONNREFUSED}}
		errnounknown      = &net.OpError{Net: "tcp", Op: "dial", Err: &os.SyscallError{Err: syscall.E2BIG}}
		tcperror          = &net.OpError{Net: "tcp", Err: errors.New("tcp error")}
		timeoutedError    = &net.OpError{Net: "tcp", Op: "dial", Err: timeoutError(true)}
		notTimeoutedError = &net.OpError{Net: "tcp", Op: "dial", Err: timeoutError(false)}
	)

	testTable := map[errCode]error{
		defaultNetNonTCPErrorCode: nonTCPError,
		tcpResetByPeerErrorCode:   econnreset,
		tcpBrokenPipeErrorCode:    epipeerror,
		tcpDialRefusedErrorCode:   econnrefused,
		tcpDialUnknownErrnoCode:   errnounknown,
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

	result, _ = errorCodeForError(fmt.Errorf("foo: %w", err))
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

func TestConnReset(t *testing.T) {
	t.Parallel()
	// based on https://gist.github.com/jpittis/4357d817dc425ae99fbf719828ab1800
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr()
	ch := make(chan error, 10)

	go func() {
		defer close(ch)
		// Accept one connection.
		conn, innerErr := ln.Accept()
		if innerErr != nil {
			ch <- innerErr
			return
		}

		// Force an RST
		tcpConn, ok := conn.(*net.TCPConn)
		require.True(t, ok)
		innerErr = tcpConn.SetLinger(0)
		if innerErr != nil {
			ch <- innerErr
		}
		time.Sleep(time.Second) // Give time for the http request to start
		_ = conn.Close()
	}()

	res, err := http.Get("http://" + addr.String()) //nolint:bodyclose,noctx
	require.Nil(t, res)

	code, msg := errorCodeForError(err)
	assert.Equal(t, tcpResetByPeerErrorCode, code)
	assert.Contains(t, msg, fmt.Sprintf(tcpResetByPeerErrorCodeMsg, ""))
	for err := range ch {
		assert.Nil(t, err)
	}
}

func TestDnsResolve(t *testing.T) {
	t.Parallel()
	// this uses the Unwrap path
	// this is not happening in our current codebase as the resolution in our code
	// happens earlier so it doesn't get wrapped, but possibly happens in other cases as well
	_, err := http.Get("http://s.com") //nolint:bodyclose,noctx
	code, msg := errorCodeForError(err)

	assert.Equal(t, dnsNoSuchHostErrorCode, code)
	assert.Equal(t, dnsNoSuchHostErrorCodeMsg, msg)
}
