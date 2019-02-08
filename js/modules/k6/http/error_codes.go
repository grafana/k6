package http

import (
	"crypto/tls"
	"crypto/x509"
	"net"
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
	defaultTCPErrorCode     errCode = 1200
	tcpBrokenPipeErrorCode  errCode = 1201
	tcpDialErrorCode        errCode = 1210
	tcpDialTimeoutErrorCode errCode = 1211
	tcpDialRefusedErrorCode errCode = 1212
	tcpResetByPeerErrorCode errCode = 1220
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
	dnsNoSuchHostErrorCode:   "lookup: no such host",
	blackListedIPErrorCode:   "ip is blacklisted",
	http2GoAwayErrorCode:     "http2: received GoAway",
	http2StreamErrorCode:     "http2: stream error",
	http2ConnectionErrorCode: "http2: connection error",
	x509HostnameErrorCode:    "x509: certificate doesn't match hostname",
}

func errorCodeForError(err error) errCode {
	switch e := errors.Cause(err).(type) {
	case *net.DNSError:
		switch e.Err {
		case "no such host": // defined as private in the go stdlib
			return dnsNoSuchHostErrorCode
		default:
			return defaultDNSErrorCode
		}
	case netext.BlackListedIPError:
		return blackListedIPErrorCode
	case *http2.GoAwayError:
		// TODO: Add different error for all errcode for goaway
		return http2GoAwayErrorCode
	case *http2.StreamError:
		// TODO: Add different error for all errcode for stream error
		return http2StreamErrorCode
	case *http2.ConnectionError:
		// TODO: Add different error for all errcode for connetion error
		return http2ConnectionErrorCode
	case *net.OpError:
		if e.Net != "tcp" && e.Net != "tcp6" {
			// TODO: figure out how this happens
			return defaultNetNonTCPErrorCode
		}
		if e.Op == "write" {
			switch e.Err.Error() {
			case syscall.ECONNRESET.Error():
				return tcpResetByPeerErrorCode
			case syscall.EPIPE.Error():
				return tcpBrokenPipeErrorCode
			}
		}
		if e.Op == "dial" {
			if e.Timeout() {
				return tcpDialTimeoutErrorCode
			}
			switch e.Err.Error() {
			case syscall.ECONNREFUSED.Error():
				return tcpDialRefusedErrorCode
			}
			return tcpDialErrorCode
		}
		return defaultTCPErrorCode

	case *x509.UnknownAuthorityError:
		return x509UnknownAuthorityErrorCode
	case *x509.HostnameError:
		return x509HostnameErrorCode
	case *tls.RecordHeaderError:
		return defaultTLSErrorCode
	default:
		return defaultErrorCode
	}
}
