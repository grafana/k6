package qerr

import (
	"fmt"
	"net"

	"github.com/quic-go/quic-go/internal/protocol"
)

var (
	ErrHandshakeTimeout = &HandshakeTimeoutError{}
	ErrIdleTimeout      = &IdleTimeoutError{}
)

type TransportError struct {
	Remote       bool
	FrameType    uint64
	ErrorCode    TransportErrorCode
	ErrorMessage string
	error        error // only set for local errors, sometimes
}

var _ error = &TransportError{}

// NewLocalCryptoError create a new TransportError instance for a crypto error
func NewLocalCryptoError(tlsAlert uint8, err error) *TransportError {
	return &TransportError{
		ErrorCode: 0x100 + TransportErrorCode(tlsAlert),
		error:     err,
	}
}

func (e *TransportError) Error() string {
	str := fmt.Sprintf("%s (%s)", e.ErrorCode.String(), getRole(e.Remote))
	if e.FrameType != 0 {
		str += fmt.Sprintf(" (frame type: %#x)", e.FrameType)
	}
	msg := e.ErrorMessage
	if len(msg) == 0 && e.error != nil {
		msg = e.error.Error()
	}
	if len(msg) == 0 {
		msg = e.ErrorCode.Message()
	}
	if len(msg) == 0 {
		return str
	}
	return str + ": " + msg
}

func (e *TransportError) Is(target error) bool {
	return target == net.ErrClosed
}

func (e *TransportError) Unwrap() error {
	return e.error
}

// An ApplicationErrorCode is an application-defined error code.
type ApplicationErrorCode uint64

func (e *ApplicationError) Is(target error) bool {
	return target == net.ErrClosed
}

// A StreamErrorCode is an error code used to cancel streams.
type StreamErrorCode uint64

type ApplicationError struct {
	Remote       bool
	ErrorCode    ApplicationErrorCode
	ErrorMessage string
}

var _ error = &ApplicationError{}

func (e *ApplicationError) Error() string {
	if len(e.ErrorMessage) == 0 {
		return fmt.Sprintf("Application error %#x (%s)", e.ErrorCode, getRole(e.Remote))
	}
	return fmt.Sprintf("Application error %#x (%s): %s", e.ErrorCode, getRole(e.Remote), e.ErrorMessage)
}

type IdleTimeoutError struct{}

var _ error = &IdleTimeoutError{}

func (e *IdleTimeoutError) Timeout() bool        { return true }
func (e *IdleTimeoutError) Temporary() bool      { return false }
func (e *IdleTimeoutError) Error() string        { return "timeout: no recent network activity" }
func (e *IdleTimeoutError) Is(target error) bool { return target == net.ErrClosed }

type HandshakeTimeoutError struct{}

var _ error = &HandshakeTimeoutError{}

func (e *HandshakeTimeoutError) Timeout() bool        { return true }
func (e *HandshakeTimeoutError) Temporary() bool      { return false }
func (e *HandshakeTimeoutError) Error() string        { return "timeout: handshake did not complete in time" }
func (e *HandshakeTimeoutError) Is(target error) bool { return target == net.ErrClosed }

// A VersionNegotiationError occurs when the client and the server can't agree on a QUIC version.
type VersionNegotiationError struct {
	Ours   []protocol.VersionNumber
	Theirs []protocol.VersionNumber
}

func (e *VersionNegotiationError) Error() string {
	return fmt.Sprintf("no compatible QUIC version found (we support %s, server offered %s)", e.Ours, e.Theirs)
}

func (e *VersionNegotiationError) Is(target error) bool {
	return target == net.ErrClosed
}

// A StatelessResetError occurs when we receive a stateless reset.
type StatelessResetError struct {
	Token protocol.StatelessResetToken
}

var _ net.Error = &StatelessResetError{}

func (e *StatelessResetError) Error() string {
	return fmt.Sprintf("received a stateless reset with token %x", e.Token)
}

func (e *StatelessResetError) Is(target error) bool {
	return target == net.ErrClosed
}

func (e *StatelessResetError) Timeout() bool   { return false }
func (e *StatelessResetError) Temporary() bool { return true }

func getRole(remote bool) string {
	if remote {
		return "remote"
	}
	return "local"
}
