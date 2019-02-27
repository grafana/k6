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
	"fmt"
	"net"
	"runtime"
	"syscall"

	"github.com/loadimpact/k6/lib/netext"
	"github.com/pkg/errors"
	"golang.org/x/net/http2"
)

type errCode uint32

const (
	// non specific
	defaultErrorCode          errCode = 1000
	defaultNetNonTCPErrorCode errCode = 1010
	// DNS errors
	defaultDNSErrorCode    errCode = 1100
	dnsNoSuchHostErrorCode errCode = 1101
	blackListedIPErrorCode errCode = 1110
	// tcp errors
	defaultTCPErrorCode      errCode = 1200
	tcpBrokenPipeErrorCode   errCode = 1201
	netUnknownErrnoErrorCode errCode = 1202
	tcpDialErrorCode         errCode = 1210
	tcpDialTimeoutErrorCode  errCode = 1211
	tcpDialRefusedErrorCode  errCode = 1212
	tcpResetByPeerErrorCode  errCode = 1220
	// TLS errors
	defaultTLSErrorCode           errCode = 1300
	x509UnknownAuthorityErrorCode errCode = 1310
	x509HostnameErrorCode         errCode = 1311

	// HTTP2 errors
	// defaultHTTP2ErrorCode errCode = 1600 // commented because of golint
	// HTTP2 GoAway errors
	unknownHTTP2GoAwayErrorCode errCode = 1610
	// errors till 1611 + 13 are other HTTP2 GoAway errors with a specific errCode

	// HTTP2 Stream errors
	unknownHTTP2StreamErrorCode errCode = 1630
	// errors till 1631 + 13 are other HTTP2 Stream errors with a specific errCode

	// HTTP2 Connection errors
	unknownHTTP2ConnectionErrorCode errCode = 1650
	// errors till 1651 + 13 are other HTTP2 Connection errors with a specific errCode
)

var (
	tcpResetByPeerErrorCodeMsg  = "write: connection reset by peer"
	tcpDialTimeoutErrorCodeMsg  = "dial: i/o timeout"
	tcpDialRefusedErrorCodeMsg  = "dial: connection refused"
	tcpBrokenPipeErrorCodeMsg   = "write: broken pipe"
	netUnknownErrnoErrorCodeMsg = "%s: unknown errno `%d` on %s with message `%s`"
	dnsNoSuchHostErrorCodeMsg   = "lookup: no such host"
	blackListedIPErrorCodeMsg   = "ip is blacklisted"
	http2GoAwayErrorCodeMsg     = "http2: received GoAway with http2 ErrCode %s"
	http2StreamErrorCodeMsg     = "http2: stream error with http2 ErrCode %s"
	http2ConnectionErrorCodeMsg = "http2: connection error with http2 ErrCode %s"
	x509HostnameErrorCodeMsg    = "x509: certificate doesn't match hostname"
)

func http2ErrCodeOffset(code http2.ErrCode) errCode {
	if code > http2.ErrCodeHTTP11Required {
		return 0
	}
	return 1 + errCode(code)
}

// returns the errorCode and a specific error message for given error. If errror message is empty
// than the original error string should be used
func errorCodeForError(err error) (errCode, string) {
	switch e := errors.Cause(err).(type) {
	case *net.DNSError:
		switch e.Err {
		case "no such host": // defined as private in the go stdlib
			return dnsNoSuchHostErrorCode, dnsNoSuchHostErrorCodeMsg
		default:
			return defaultDNSErrorCode, ""
		}
	case netext.BlackListedIPError:
		return blackListedIPErrorCode, blackListedIPErrorCodeMsg
	case *http2.GoAwayError:
		return unknownHTTP2GoAwayErrorCode + http2ErrCodeOffset(e.ErrCode), fmt.Sprintf(http2GoAwayErrorCodeMsg, e.ErrCode)
	case *http2.StreamError:
		return unknownHTTP2StreamErrorCode + http2ErrCodeOffset(e.Code), fmt.Sprintf(http2StreamErrorCodeMsg, e.Code)
	case *http2.ConnectionError:
		return unknownHTTP2ConnectionErrorCode + http2ErrCodeOffset(http2.ErrCode(*e)), fmt.Sprintf(http2ConnectionErrorCodeMsg, http2.ErrCode(*e))
	case *net.OpError:
		if e.Net != "tcp" && e.Net != "tcp6" {
			// TODO: figure out how this happens
			return defaultNetNonTCPErrorCode, ""
		}
		if e.Op == "write" {
			switch e.Err.Error() {
			case syscall.ECONNRESET.Error():
				return tcpResetByPeerErrorCode, tcpResetByPeerErrorCodeMsg
			case syscall.EPIPE.Error():
				return tcpBrokenPipeErrorCode, tcpBrokenPipeErrorCodeMsg
			}
		}
		if e.Op == "dial" {
			if e.Timeout() {
				return tcpDialTimeoutErrorCode, tcpDialTimeoutErrorCodeMsg
			}
			switch e.Err.Error() {
			case syscall.ECONNREFUSED.Error():
				return tcpDialRefusedErrorCode, tcpDialRefusedErrorCodeMsg
			}
			return tcpDialErrorCode, ""
		}
		switch inErr := e.Err.(type) {
		case syscall.Errno:
			return netUnknownErrnoErrorCode,
				fmt.Sprintf(netUnknownErrnoErrorCodeMsg,
					e.Op, (int)(inErr), runtime.GOOS, inErr.Error())
		default:
			return defaultTCPErrorCode, ""
		}

	case *x509.UnknownAuthorityError:
		return x509UnknownAuthorityErrorCode, x509HostnameErrorCodeMsg
	case *x509.HostnameError:
		return x509HostnameErrorCode, x509HostnameErrorCodeMsg
	case *tls.RecordHeaderError:
		return defaultTLSErrorCode, ""
	default:
		return defaultErrorCode, ""
	}
}
