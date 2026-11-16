// Package websocket implements a basic websocket server.
package websocket

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

const requiredVersion = "13"

// Opcode is a websocket OPCODE.
type Opcode uint8

// See the RFC for the set of defined opcodes:
// https://datatracker.ietf.org/doc/html/rfc6455#section-5.2
const (
	OpcodeContinuation Opcode = 0x0
	OpcodeText         Opcode = 0x1
	OpcodeBinary       Opcode = 0x2
	OpcodeClose        Opcode = 0x8
	OpcodePing         Opcode = 0x9
	OpcodePong         Opcode = 0xA
)

// StatusCode is a websocket status code.
type StatusCode uint16

// See the RFC for the set of defined status codes:
// https://datatracker.ietf.org/doc/html/rfc6455#section-7.4.1
const (
	StatusNormalClosure      StatusCode = 1000
	StatusGoingAway          StatusCode = 1001
	StatusProtocolError      StatusCode = 1002
	StatusUnsupported        StatusCode = 1003
	StatusNoStatusRcvd       StatusCode = 1005
	StatusAbnormalClose      StatusCode = 1006
	StatusUnsupportedPayload StatusCode = 1007
	StatusPolicyViolation    StatusCode = 1008
	StatusTooLarge           StatusCode = 1009
	StatusTlSHandshake       StatusCode = 1015
	StatusServerError        StatusCode = 1011
)

// Errors indicating that the client violated the websocket protocol or
// exceeded a configured limit.
var (
	errUnmaskedFrame          = errors.New("received unmasked client frame")
	errUnsupportedRSV         = errors.New("frame has unsupported RSV bits set")
	errInvalidPayloadLength   = errors.New("invalid payload length")
	errFrameTooLarge          = errors.New("frame too large")
	errInvalidControlFrame    = errors.New("invalid control frame")
	errInvalidClosePayload    = errors.New("invalid close frame payload")
	errUnsupportedOpcode      = errors.New("unsupported opcode")
	errContinuationExpected   = errors.New("continuation frame expected")
	errUnexpectedContinuation = errors.New("unexpected continuation frame")
	errInvalidUTF8            = errors.New("invalid UTF-8")
	errMessageTooLarge        = errors.New("message too large")
)

// Frame is a websocket protocol frame.
type Frame struct {
	Fin     bool
	RSV1    bool
	RSV3    bool
	RSV2    bool
	Opcode  Opcode
	Payload []byte
}

// Message is an application-level message from the client, which may be
// constructed from one or more individual protocol frames.
type Message struct {
	Binary  bool
	Payload []byte
}

// Handler handles a single websocket message. If the returned message is
// non-nil, it will be sent to the client. If an error is returned, the
// connection will be closed.
type Handler func(ctx context.Context, msg *Message) (*Message, error)

// EchoHandler is a Handler that echoes each incoming message back to the
// client.
var EchoHandler Handler = func(_ context.Context, msg *Message) (*Message, error) {
	return msg, nil
}

// Limits define the limits imposed on a websocket connection.
type Limits struct {
	MaxDuration     time.Duration
	MaxFragmentSize int
	MaxMessageSize  int
}

// WebSocket is a websocket connection.
type WebSocket struct {
	w               http.ResponseWriter
	r               *http.Request
	maxDuration     time.Duration
	maxFragmentSize int
	maxMessageSize  int
	handshook       bool
}

// New creates a new websocket.
func New(w http.ResponseWriter, r *http.Request, limits Limits) *WebSocket {
	return &WebSocket{
		w:               w,
		r:               r,
		maxDuration:     limits.MaxDuration,
		maxFragmentSize: limits.MaxFragmentSize,
		maxMessageSize:  limits.MaxMessageSize,
	}
}

// Handshake validates the request and performs the WebSocket handshake. If
// Handshake returns nil, only websocket frames should be written to the
// response writer.
func (s *WebSocket) Handshake() error {
	if s.handshook {
		panic("websocket: handshake already completed")
	}

	if strings.ToLower(s.r.Header.Get("Upgrade")) != "websocket" {
		return fmt.Errorf("missing required `Upgrade: websocket` header")
	}
	if v := s.r.Header.Get("Sec-Websocket-Version"); v != requiredVersion {
		return fmt.Errorf("only websocket version %q is supported, got %q", requiredVersion, v)
	}

	clientKey := s.r.Header.Get("Sec-Websocket-Key")
	if clientKey == "" {
		return fmt.Errorf("missing required `Sec-Websocket-Key` header")
	}

	s.w.Header().Set("Connection", "upgrade")
	s.w.Header().Set("Upgrade", "websocket")
	s.w.Header().Set("Sec-Websocket-Accept", acceptKey(clientKey))
	s.w.WriteHeader(http.StatusSwitchingProtocols)

	s.handshook = true
	return nil
}

// Serve handles a websocket connection after the handshake has been completed.
func (s *WebSocket) Serve(handler Handler) {
	if !s.handshook {
		panic("websocket: serve: handshake not completed")
	}

	hj, ok := s.w.(http.Hijacker)
	if !ok {
		panic("websocket: serve: server does not support hijacking")
	}

	conn, buf, err := hj.Hijack()
	if err != nil {
		panic(fmt.Errorf("websocket: serve: hijack failed: %s", err))
	}
	defer conn.Close()

	// best effort attempt to ensure that our websocket conenctions do not
	// exceed the maximum request duration
	conn.SetDeadline(time.Now().Add(s.maxDuration))

	// errors intentionally ignored here. it's serverLoop's responsibility to
	// properly close the websocket connection with a useful error message, and
	// any unexpected error returned from serverLoop is not actionable.
	_ = s.serveLoop(s.r.Context(), buf, handler)
}

func (s *WebSocket) serveLoop(ctx context.Context, buf *bufio.ReadWriter, handler Handler) error {
	var currentMsg *Message

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		var currentMsgSize int
		if currentMsg != nil {
			currentMsgSize = len(currentMsg.Payload)
		}

		frame, err := nextFrame(buf, s.maxFragmentSize, s.maxMessageSize, currentMsgSize)
		if err != nil {
			return closeWithError(buf, err)
		}

		switch frame.Opcode {
		case OpcodeBinary, OpcodeText:
			if currentMsg != nil {
				return closeWithError(buf, errContinuationExpected)
			}
			if frame.Opcode == OpcodeText && !utf8.Valid(frame.Payload) {
				return closeWithError(buf, errInvalidUTF8)
			}
			currentMsg = &Message{
				Binary:  frame.Opcode == OpcodeBinary,
				Payload: frame.Payload,
			}
		case OpcodeContinuation:
			if currentMsg == nil {
				return closeWithError(buf, errUnexpectedContinuation)
			}
			if !currentMsg.Binary && !utf8.Valid(frame.Payload) {
				return closeWithError(buf, errInvalidUTF8)
			}
			// nextFrame has already ensured that appending this frame's
			// payload will not exceed the maximum message size
			currentMsg.Payload = append(currentMsg.Payload, frame.Payload...)
		case OpcodeClose:
			return writeCloseFrame(buf, StatusNormalClosure, nil)
		case OpcodePing:
			frame.Opcode = OpcodePong
			if err := writeFrame(buf, frame); err != nil {
				return err
			}
			continue
		case OpcodePong:
			continue
		default:
			return closeWithError(buf, fmt.Errorf("%w: %v", errUnsupportedOpcode, frame.Opcode))
		}

		if frame.Fin {
			resp, err := handler(ctx, currentMsg)
			if err != nil {
				return closeWithError(buf, err)
			}
			if resp == nil {
				continue
			}
			for _, respFrame := range frameResponse(resp, s.maxFragmentSize) {
				if err := writeFrame(buf, respFrame); err != nil {
					return err
				}
			}
			currentMsg = nil
		}
	}
}

// nextFrame reads the next frame from buf. The frame header is validated
// before its payload is read, so that a client cannot force arbitrarily large
// allocations by claiming a huge payload length. currentMsgSize is the size of
// any in-progress fragmented message, which a continuation frame's payload
// would be appended to. Any returned frame is valid.
func nextFrame(buf *bufio.ReadWriter, maxFragmentSize int, maxMessageSize int, currentMsgSize int) (*Frame, error) {
	bb := make([]byte, 2)
	if _, err := io.ReadFull(buf, bb); err != nil {
		return nil, err
	}

	b0 := bb[0]
	b1 := bb[1]

	var (
		fin    = b0&0b10000000 != 0
		rsv1   = b0&0b01000000 != 0
		rsv2   = b0&0b00100000 != 0
		rsv3   = b0&0b00010000 != 0
		opcode = Opcode(b0 & 0b00001111)
	)

	// Per https://datatracker.ietf.org/doc/html/rfc6455#section-5.2, all
	// client frames must be masked.
	if masked := b1 & 0b10000000; masked == 0 {
		return nil, errUnmaskedFrame
	}

	var payloadLength uint64
	switch {
	case b1-128 <= 125:
		// Payload length is directly represented in the second byte
		payloadLength = uint64(b1 - 128)
	case b1-128 == 126:
		// Payload length is represented in the next 2 bytes (16-bit unsigned integer)
		var l uint16
		if err := binary.Read(buf, binary.BigEndian, &l); err != nil {
			return nil, err
		}
		payloadLength = uint64(l)
	case b1-128 == 127:
		// Payload length is represented in the next 8 bytes (64-bit unsigned integer)
		if err := binary.Read(buf, binary.BigEndian, &payloadLength); err != nil {
			return nil, err
		}
		// Per https://datatracker.ietf.org/doc/html/rfc6455#section-5.2, the
		// most significant bit of a 64-bit payload length MUST be 0.
		if payloadLength&(1<<63) != 0 {
			return nil, fmt.Errorf("%w: most significant bit of 64-bit length must be 0", errInvalidPayloadLength)
		}
	}

	frame := &Frame{
		Fin:    fin,
		RSV1:   rsv1,
		RSV2:   rsv2,
		RSV3:   rsv3,
		Opcode: opcode,
	}
	if err := validateFrameHeader(frame, payloadLength, maxFragmentSize, maxMessageSize, currentMsgSize); err != nil {
		return nil, err
	}

	mask := make([]byte, 4)
	if _, err := io.ReadFull(buf, mask); err != nil {
		return nil, err
	}

	payload := make([]byte, payloadLength)
	if _, err := io.ReadFull(buf, payload); err != nil {
		return nil, err
	}

	for i, b := range payload {
		payload[i] = b ^ mask[i%4]
	}
	frame.Payload = payload

	if frame.Opcode == OpcodeClose {
		if err := validateClosePayload(frame.Payload); err != nil {
			return nil, err
		}
	}

	return frame, nil
}

func writeFrame(dst *bufio.ReadWriter, frame *Frame) error {
	// FIN, RSV1-3, OPCODE
	var b1 byte
	if frame.Fin {
		b1 |= 0b10000000
	}
	if frame.RSV1 {
		b1 |= 0b01000000
	}
	if frame.RSV2 {
		b1 |= 0b00100000
	}
	if frame.RSV3 {
		b1 |= 0b00010000
	}
	b1 |= uint8(frame.Opcode) & 0b00001111
	if err := dst.WriteByte(b1); err != nil {
		return err
	}

	// payload length
	payloadLen := int64(len(frame.Payload))
	switch {
	case payloadLen <= 125:
		if err := dst.WriteByte(byte(payloadLen)); err != nil {
			return err
		}
	case payloadLen <= 65535:
		if err := dst.WriteByte(126); err != nil {
			return err
		}
		if err := binary.Write(dst, binary.BigEndian, uint16(payloadLen)); err != nil {
			return err
		}
	default:
		if err := dst.WriteByte(127); err != nil {
			return err
		}
		if err := binary.Write(dst, binary.BigEndian, payloadLen); err != nil {
			return err
		}
	}

	// payload
	if _, err := dst.Write(frame.Payload); err != nil {
		return err
	}

	return dst.Flush()
}

// writeCloseFrame writes a close frame to the wire, with an optional error
// message.
func writeCloseFrame(dst *bufio.ReadWriter, code StatusCode, err error) error {
	var payload []byte
	payload = binary.BigEndian.AppendUint16(payload, uint16(code))
	if err != nil {
		payload = append(payload, []byte(err.Error())...)
	}
	return writeFrame(dst, &Frame{
		Fin:     true,
		Opcode:  OpcodeClose,
		Payload: payload,
	})
}

// closeStatusForError returns the status code to use when closing the
// connection due to err. Any error not explicitly handled here (e.g. an error
// reading from the connection or returned by a Handler) is a server error.
func closeStatusForError(err error) StatusCode {
	switch {
	case errors.Is(err, errFrameTooLarge),
		errors.Is(err, errMessageTooLarge):
		return StatusTooLarge
	case errors.Is(err, errInvalidUTF8):
		return StatusUnsupportedPayload
	case errors.Is(err, errUnmaskedFrame),
		errors.Is(err, errUnsupportedRSV),
		errors.Is(err, errInvalidPayloadLength),
		errors.Is(err, errInvalidControlFrame),
		errors.Is(err, errInvalidClosePayload),
		errors.Is(err, errUnsupportedOpcode),
		errors.Is(err, errContinuationExpected),
		errors.Is(err, errUnexpectedContinuation):
		return StatusProtocolError
	default:
		return StatusServerError
	}
}

// closeWithError closes the connection with the status code appropriate for
// err, using err as the close reason.
func closeWithError(dst *bufio.ReadWriter, err error) error {
	return writeCloseFrame(dst, closeStatusForError(err), err)
}

// frameResponse splits a message into N frames with payloads of at most
// fragmentSize bytes.
func frameResponse(msg *Message, fragmentSize int) []*Frame {
	var result []*Frame

	fin := false
	opcode := OpcodeText
	if msg.Binary {
		opcode = OpcodeBinary
	}

	offset := 0
	dataLen := len(msg.Payload)
	for {
		if offset > 0 {
			opcode = OpcodeContinuation
		}
		end := offset + fragmentSize
		if end >= dataLen {
			fin = true
			end = dataLen
		}
		result = append(result, &Frame{
			Fin:     fin,
			Opcode:  opcode,
			Payload: msg.Payload[offset:end],
		})
		if fin {
			break
		}
	}
	return result
}

var reservedStatusCodes = map[uint16]bool{
	// Explicitly reserved by RFC section 7.4.1 Defined Status Codes:
	// https://datatracker.ietf.org/doc/html/rfc6455#section-7.4.1
	1004: true,
	1005: true,
	1006: true,
	1015: true,
	// Apparently reserved, according to the autobahn testsuite's fuzzingclient
	// tests, though it's not clear to me why, based on the RFC.
	//
	// See: https://github.com/crossbario/autobahn-testsuite
	1016: true,
	1100: true,
	2000: true,
	2999: true,
}

// validateFrameHeader validates a frame before its payload has been read.
func validateFrameHeader(frame *Frame, payloadLength uint64, maxFragmentSize int, maxMessageSize int, currentMsgSize int) error {
	// We do not support any extensions, per the spec all RSV bits must be 0:
	// https://datatracker.ietf.org/doc/html/rfc6455#section-5.2
	if frame.RSV1 || frame.RSV2 || frame.RSV3 {
		return errUnsupportedRSV
	}

	switch frame.Opcode {
	case OpcodeContinuation, OpcodeText, OpcodeBinary:
		if payloadLength > uint64(maxFragmentSize) {
			return fmt.Errorf("%w: payload size %d exceeds maximum of %d bytes", errFrameTooLarge, payloadLength, maxFragmentSize)
		}
		// A continuation frame's payload will be appended to the current
		// message, so we must ensure the resulting message will not exceed
		// the maximum message size before reading the payload.
		if frame.Opcode == OpcodeContinuation {
			if msgSize := uint64(currentMsgSize) + payloadLength; msgSize > uint64(maxMessageSize) {
				return fmt.Errorf("%w: size %d exceeds maximum of %d bytes", errMessageTooLarge, msgSize, maxMessageSize)
			}
		}
	case OpcodeClose, OpcodePing, OpcodePong:
		// All control frames MUST have a payload length of 125 bytes or less
		// and MUST NOT be fragmented.
		// https://datatracker.ietf.org/doc/html/rfc6455#section-5.5
		if payloadLength > 125 {
			return fmt.Errorf("%w: payload size %d exceeds 125 bytes", errInvalidControlFrame, payloadLength)
		}
		if !frame.Fin {
			return fmt.Errorf("%w: control frame %v must not be fragmented", errInvalidControlFrame, frame.Opcode)
		}
	}

	return nil
}

// validateClosePayload validates the status code and reason in a close
// frame's payload.
func validateClosePayload(payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	if len(payload) == 1 {
		return fmt.Errorf("%w: must be at least 2 bytes", errInvalidClosePayload)
	}

	code := binary.BigEndian.Uint16(payload[:2])
	if code < 1000 || code >= 5000 {
		return fmt.Errorf("%w: status code %d out of range", errInvalidClosePayload, code)
	}
	if reservedStatusCodes[code] {
		return fmt.Errorf("%w: status code %d is reserved", errInvalidClosePayload, code)
	}

	if len(payload) > 2 {
		if !utf8.Valid(payload[2:]) {
			return fmt.Errorf("%w: reason must be valid UTF-8", errInvalidClosePayload)
		}
	}

	return nil
}

func acceptKey(clientKey string) string {
	// Magic value comes from RFC 6455 section 1.3: Opening Handshake
	// https://www.rfc-editor.org/rfc/rfc6455#section-1.3
	h := sha1.New()
	io.WriteString(h, clientKey+"258EAFA5-E914-47DA-95CA-C5AB0DC85B11")
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
