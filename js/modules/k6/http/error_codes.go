package http

import (
	"crypto/tls"
	"crypto/x509"
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
	http2GoAwayErrorCode      errCode = 1091
	http2StreamErrorCode      errCode = 1092
	http2ConnectionErrorCode  errCode = 1093
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
)

// If a given errorCode from above need to overwrite the error message this should be provided in
// this map
var customErrorMsgMap = map[errCode]string{
	tcpResetByPeerErrorCode:  "write: connection reset by peer",
	tcpDialTimeoutErrorCode:  "dial: i/o timeout",
	tcpDialRefusedErrorCode:  "dial: connection refused",
	tcpBrokenPipeErrorCode:   "write: broken pipe",
	netUnknownErrnoErrorCode: "%s: unknown errno `%d` on %s with message `%s`",
	dnsNoSuchHostErrorCode:   "lookup: no such host",
	blackListedIPErrorCode:   "ip is blacklisted",
	http2GoAwayErrorCode:     "http2: received GoAway",
	http2StreamErrorCode:     "http2: stream error",
	http2ConnectionErrorCode: "http2: connection error",
	x509HostnameErrorCode:    "x509: certificate doesn't match hostname",
}

func errorCodeForError(err error) (errCode, []interface{}) {
	switch e := errors.Cause(err).(type) {
	case *net.DNSError:
		switch e.Err {
		case "no such host": // defined as private in the go stdlib
			return dnsNoSuchHostErrorCode, nil
		default:
			return defaultDNSErrorCode, nil
		}
	case netext.BlackListedIPError:
		return blackListedIPErrorCode, nil
	case *http2.GoAwayError:
		// TODO: Add different error for all errcode for goaway
		return http2GoAwayErrorCode, nil
	case *http2.StreamError:
		// TODO: Add different error for all errcode for stream error
		return http2StreamErrorCode, nil
	case *http2.ConnectionError:
		// TODO: Add different error for all errcode for connetion error
		return http2ConnectionErrorCode, nil
	case *net.OpError:
		if e.Net != "tcp" && e.Net != "tcp6" {
			// TODO: figure out how this happens
			return defaultNetNonTCPErrorCode, nil
		}
		if e.Op == "write" {
			switch e.Err.Error() {
			case syscall.ECONNRESET.Error():
				return tcpResetByPeerErrorCode, nil
			case syscall.EPIPE.Error():
				return tcpBrokenPipeErrorCode, nil
			}
		}
		if e.Op == "dial" {
			if e.Timeout() {
				return tcpDialTimeoutErrorCode, nil
			}
			switch e.Err.Error() {
			case syscall.ECONNREFUSED.Error():
				return tcpDialRefusedErrorCode, nil
			}
			return tcpDialErrorCode, nil
		}
		switch inErr := e.Err.(type) {
		case syscall.Errno:
			return netUnknownErrnoErrorCode, []interface{}{
				e.Op, (int)(inErr), runtime.GOOS, inErr.Error(),
			}
		default:
			return defaultTCPErrorCode, nil
		}

	case *x509.UnknownAuthorityError:
		return x509UnknownAuthorityErrorCode, nil
	case *x509.HostnameError:
		return x509HostnameErrorCode, nil
	case *tls.RecordHeaderError:
		return defaultTLSErrorCode, nil
	default:
		return defaultErrorCode, nil
	}
}
