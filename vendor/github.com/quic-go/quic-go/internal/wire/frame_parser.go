package wire

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/qerr"
	"github.com/quic-go/quic-go/quicvarint"
)

const (
	pingFrameType               = 0x1
	ackFrameType                = 0x2
	ackECNFrameType             = 0x3
	resetStreamFrameType        = 0x4
	stopSendingFrameType        = 0x5
	cryptoFrameType             = 0x6
	newTokenFrameType           = 0x7
	maxDataFrameType            = 0x10
	maxStreamDataFrameType      = 0x11
	bidiMaxStreamsFrameType     = 0x12
	uniMaxStreamsFrameType      = 0x13
	dataBlockedFrameType        = 0x14
	streamDataBlockedFrameType  = 0x15
	bidiStreamBlockedFrameType  = 0x16
	uniStreamBlockedFrameType   = 0x17
	newConnectionIDFrameType    = 0x18
	retireConnectionIDFrameType = 0x19
	pathChallengeFrameType      = 0x1a
	pathResponseFrameType       = 0x1b
	connectionCloseFrameType    = 0x1c
	applicationCloseFrameType   = 0x1d
	handshakeDoneFrameType      = 0x1e
)

type frameParser struct {
	r bytes.Reader // cached bytes.Reader, so we don't have to repeatedly allocate them

	ackDelayExponent  uint8
	supportsDatagrams bool

	// To avoid allocating when parsing, keep a single ACK frame struct.
	// It is used over and over again.
	ackFrame *AckFrame
}

var _ FrameParser = &frameParser{}

// NewFrameParser creates a new frame parser.
func NewFrameParser(supportsDatagrams bool) *frameParser {
	return &frameParser{
		r:                 *bytes.NewReader(nil),
		supportsDatagrams: supportsDatagrams,
		ackFrame:          &AckFrame{},
	}
}

// ParseNext parses the next frame.
// It skips PADDING frames.
func (p *frameParser) ParseNext(data []byte, encLevel protocol.EncryptionLevel, v protocol.VersionNumber) (int, Frame, error) {
	startLen := len(data)
	p.r.Reset(data)
	frame, err := p.parseNext(&p.r, encLevel, v)
	n := startLen - p.r.Len()
	p.r.Reset(nil)
	return n, frame, err
}

func (p *frameParser) parseNext(r *bytes.Reader, encLevel protocol.EncryptionLevel, v protocol.VersionNumber) (Frame, error) {
	for r.Len() != 0 {
		typ, err := quicvarint.Read(r)
		if err != nil {
			return nil, &qerr.TransportError{
				ErrorCode:    qerr.FrameEncodingError,
				ErrorMessage: err.Error(),
			}
		}
		if typ == 0x0 { // skip PADDING frames
			continue
		}

		f, err := p.parseFrame(r, typ, encLevel, v)
		if err != nil {
			return nil, &qerr.TransportError{
				FrameType:    typ,
				ErrorCode:    qerr.FrameEncodingError,
				ErrorMessage: err.Error(),
			}
		}
		return f, nil
	}
	return nil, nil
}

func (p *frameParser) parseFrame(r *bytes.Reader, typ uint64, encLevel protocol.EncryptionLevel, v protocol.VersionNumber) (Frame, error) {
	var frame Frame
	var err error
	if typ&0xf8 == 0x8 {
		frame, err = parseStreamFrame(r, typ, v)
	} else {
		switch typ {
		case pingFrameType:
			frame = &PingFrame{}
		case ackFrameType, ackECNFrameType:
			ackDelayExponent := p.ackDelayExponent
			if encLevel != protocol.Encryption1RTT {
				ackDelayExponent = protocol.DefaultAckDelayExponent
			}
			p.ackFrame.Reset()
			err = parseAckFrame(p.ackFrame, r, typ, ackDelayExponent, v)
			frame = p.ackFrame
		case resetStreamFrameType:
			frame, err = parseResetStreamFrame(r, v)
		case stopSendingFrameType:
			frame, err = parseStopSendingFrame(r, v)
		case cryptoFrameType:
			frame, err = parseCryptoFrame(r, v)
		case newTokenFrameType:
			frame, err = parseNewTokenFrame(r, v)
		case maxDataFrameType:
			frame, err = parseMaxDataFrame(r, v)
		case maxStreamDataFrameType:
			frame, err = parseMaxStreamDataFrame(r, v)
		case bidiMaxStreamsFrameType, uniMaxStreamsFrameType:
			frame, err = parseMaxStreamsFrame(r, typ, v)
		case dataBlockedFrameType:
			frame, err = parseDataBlockedFrame(r, v)
		case streamDataBlockedFrameType:
			frame, err = parseStreamDataBlockedFrame(r, v)
		case bidiStreamBlockedFrameType, uniStreamBlockedFrameType:
			frame, err = parseStreamsBlockedFrame(r, typ, v)
		case newConnectionIDFrameType:
			frame, err = parseNewConnectionIDFrame(r, v)
		case retireConnectionIDFrameType:
			frame, err = parseRetireConnectionIDFrame(r, v)
		case pathChallengeFrameType:
			frame, err = parsePathChallengeFrame(r, v)
		case pathResponseFrameType:
			frame, err = parsePathResponseFrame(r, v)
		case connectionCloseFrameType, applicationCloseFrameType:
			frame, err = parseConnectionCloseFrame(r, typ, v)
		case handshakeDoneFrameType:
			frame = &HandshakeDoneFrame{}
		case 0x30, 0x31:
			if p.supportsDatagrams {
				frame, err = parseDatagramFrame(r, typ, v)
				break
			}
			fallthrough
		default:
			err = errors.New("unknown frame type")
		}
	}
	if err != nil {
		return nil, err
	}
	if !p.isAllowedAtEncLevel(frame, encLevel) {
		return nil, fmt.Errorf("%s not allowed at encryption level %s", reflect.TypeOf(frame).Elem().Name(), encLevel)
	}
	return frame, nil
}

func (p *frameParser) isAllowedAtEncLevel(f Frame, encLevel protocol.EncryptionLevel) bool {
	switch encLevel {
	case protocol.EncryptionInitial, protocol.EncryptionHandshake:
		switch f.(type) {
		case *CryptoFrame, *AckFrame, *ConnectionCloseFrame, *PingFrame:
			return true
		default:
			return false
		}
	case protocol.Encryption0RTT:
		switch f.(type) {
		case *CryptoFrame, *AckFrame, *ConnectionCloseFrame, *NewTokenFrame, *PathResponseFrame, *RetireConnectionIDFrame:
			return false
		default:
			return true
		}
	case protocol.Encryption1RTT:
		return true
	default:
		panic("unknown encryption level")
	}
}

func (p *frameParser) SetAckDelayExponent(exp uint8) {
	p.ackDelayExponent = exp
}
