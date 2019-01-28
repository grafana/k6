package http

import (
	"fmt"
	"net"
	"syscall"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

func TestErrorCodeForError(t *testing.T) {
	// TODO find better way to test
	var (
		nonTCPError  = new(net.OpError)
		econnreset   = new(net.OpError)
		epipeerror   = new(net.OpError)
		econnrefused = new(net.OpError)
		tcperror     = new(net.OpError)
	)
	nonTCPError.Net = "something"

	econnreset.Net = "tcp"
	econnreset.Op = "write"
	econnreset.Err = syscall.ECONNRESET

	epipeerror.Net = "tcp"
	epipeerror.Op = "write"
	epipeerror.Err = syscall.EPIPE

	econnrefused.Net = "tcp"
	econnrefused.Op = "dial"
	econnrefused.Err = syscall.ECONNREFUSED

	tcperror.Net = "tcp"

	var testTable = map[error]errCode{
		fmt.Errorf("random error"): defaultErrorCode,
		new(http2.ConnectionError): http2ConnectionErrorCode,
		new(http2.StreamError):     http2StreamErrorCode,
		new(http2.GoAwayError):     http2GoAwayErrorCode,
		nonTCPError:                defaultNetNonTCPErrorCode,
		econnreset:                 tcpResetByPeerErrorCode,
		epipeerror:                 tcpBrokenPipeErrorCode,
		econnrefused:               tcpDialRefusedErrorCode,
		tcperror:                   defaultTCPErrorCode,
	}

	for err, code := range testTable {
		require.Equalf(t, code, errorCodeForError(err), "Wrong error code for error `%s`", err)
		require.Equalf(t, code, errorCodeForError(errors.WithStack(err)), "Wrong error code for error `%s`", err)
	}
}
