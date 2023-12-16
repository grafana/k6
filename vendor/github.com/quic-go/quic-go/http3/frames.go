package http3

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/quicvarint"
)

// FrameType is the frame type of a HTTP/3 frame
type FrameType uint64

type unknownFrameHandlerFunc func(FrameType, error) (processed bool, err error)

type frame interface{}

var errHijacked = errors.New("hijacked")

func parseNextFrame(r io.Reader, unknownFrameHandler unknownFrameHandlerFunc) (frame, error) {
	qr := quicvarint.NewReader(r)
	for {
		t, err := quicvarint.Read(qr)
		if err != nil {
			if unknownFrameHandler != nil {
				hijacked, err := unknownFrameHandler(0, err)
				if err != nil {
					return nil, err
				}
				if hijacked {
					return nil, errHijacked
				}
			}
			return nil, err
		}
		// Call the unknownFrameHandler for frames not defined in the HTTP/3 spec
		if t > 0xd && unknownFrameHandler != nil {
			hijacked, err := unknownFrameHandler(FrameType(t), nil)
			if err != nil {
				return nil, err
			}
			if hijacked {
				return nil, errHijacked
			}
			// If the unknownFrameHandler didn't process the frame, it is our responsibility to skip it.
		}
		l, err := quicvarint.Read(qr)
		if err != nil {
			return nil, err
		}

		switch t {
		case 0x0:
			return &dataFrame{Length: l}, nil
		case 0x1:
			return &headersFrame{Length: l}, nil
		case 0x4:
			return parseSettingsFrame(r, l)
		case 0x3: // CANCEL_PUSH
		case 0x5: // PUSH_PROMISE
		case 0x7: // GOAWAY
		case 0xd: // MAX_PUSH_ID
		}
		// skip over unknown frames
		if _, err := io.CopyN(io.Discard, qr, int64(l)); err != nil {
			return nil, err
		}
	}
}

type dataFrame struct {
	Length uint64
}

func (f *dataFrame) Append(b []byte) []byte {
	b = quicvarint.Append(b, 0x0)
	return quicvarint.Append(b, f.Length)
}

type headersFrame struct {
	Length uint64
}

func (f *headersFrame) Append(b []byte) []byte {
	b = quicvarint.Append(b, 0x1)
	return quicvarint.Append(b, f.Length)
}

const settingDatagram = 0x33

type settingsFrame struct {
	Datagram bool
	Other    map[uint64]uint64 // all settings that we don't explicitly recognize
}

func parseSettingsFrame(r io.Reader, l uint64) (*settingsFrame, error) {
	if l > 8*(1<<10) {
		return nil, fmt.Errorf("unexpected size for SETTINGS frame: %d", l)
	}
	buf := make([]byte, l)
	if _, err := io.ReadFull(r, buf); err != nil {
		if err == io.ErrUnexpectedEOF {
			return nil, io.EOF
		}
		return nil, err
	}
	frame := &settingsFrame{}
	b := bytes.NewReader(buf)
	var readDatagram bool
	for b.Len() > 0 {
		id, err := quicvarint.Read(b)
		if err != nil { // should not happen. We allocated the whole frame already.
			return nil, err
		}
		val, err := quicvarint.Read(b)
		if err != nil { // should not happen. We allocated the whole frame already.
			return nil, err
		}

		switch id {
		case settingDatagram:
			if readDatagram {
				return nil, fmt.Errorf("duplicate setting: %d", id)
			}
			readDatagram = true
			if val != 0 && val != 1 {
				return nil, fmt.Errorf("invalid value for H3_DATAGRAM: %d", val)
			}
			frame.Datagram = val == 1
		default:
			if _, ok := frame.Other[id]; ok {
				return nil, fmt.Errorf("duplicate setting: %d", id)
			}
			if frame.Other == nil {
				frame.Other = make(map[uint64]uint64)
			}
			frame.Other[id] = val
		}
	}
	return frame, nil
}

func (f *settingsFrame) Append(b []byte) []byte {
	b = quicvarint.Append(b, 0x4)
	var l protocol.ByteCount
	for id, val := range f.Other {
		l += quicvarint.Len(id) + quicvarint.Len(val)
	}
	if f.Datagram {
		l += quicvarint.Len(settingDatagram) + quicvarint.Len(1)
	}
	b = quicvarint.Append(b, uint64(l))
	if f.Datagram {
		b = quicvarint.Append(b, settingDatagram)
		b = quicvarint.Append(b, 1)
	}
	for id, val := range f.Other {
		b = quicvarint.Append(b, id)
		b = quicvarint.Append(b, val)
	}
	return b
}
