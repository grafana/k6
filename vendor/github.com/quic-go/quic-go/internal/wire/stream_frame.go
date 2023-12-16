package wire

import (
	"bytes"
	"errors"
	"io"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/quicvarint"
)

// A StreamFrame of QUIC
type StreamFrame struct {
	StreamID       protocol.StreamID
	Offset         protocol.ByteCount
	Data           []byte
	Fin            bool
	DataLenPresent bool

	fromPool bool
}

func parseStreamFrame(r *bytes.Reader, typ uint64, _ protocol.VersionNumber) (*StreamFrame, error) {
	hasOffset := typ&0b100 > 0
	fin := typ&0b1 > 0
	hasDataLen := typ&0b10 > 0

	streamID, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	var offset uint64
	if hasOffset {
		offset, err = quicvarint.Read(r)
		if err != nil {
			return nil, err
		}
	}

	var dataLen uint64
	if hasDataLen {
		var err error
		dataLen, err = quicvarint.Read(r)
		if err != nil {
			return nil, err
		}
	} else {
		// The rest of the packet is data
		dataLen = uint64(r.Len())
	}

	var frame *StreamFrame
	if dataLen < protocol.MinStreamFrameBufferSize {
		frame = &StreamFrame{Data: make([]byte, dataLen)}
	} else {
		frame = GetStreamFrame()
		// The STREAM frame can't be larger than the StreamFrame we obtained from the buffer,
		// since those StreamFrames have a buffer length of the maximum packet size.
		if dataLen > uint64(cap(frame.Data)) {
			return nil, io.EOF
		}
		frame.Data = frame.Data[:dataLen]
	}

	frame.StreamID = protocol.StreamID(streamID)
	frame.Offset = protocol.ByteCount(offset)
	frame.Fin = fin
	frame.DataLenPresent = hasDataLen

	if dataLen != 0 {
		if _, err := io.ReadFull(r, frame.Data); err != nil {
			return nil, err
		}
	}
	if frame.Offset+frame.DataLen() > protocol.MaxByteCount {
		return nil, errors.New("stream data overflows maximum offset")
	}
	return frame, nil
}

// Write writes a STREAM frame
func (f *StreamFrame) Append(b []byte, _ protocol.VersionNumber) ([]byte, error) {
	if len(f.Data) == 0 && !f.Fin {
		return nil, errors.New("StreamFrame: attempting to write empty frame without FIN")
	}

	typ := byte(0x8)
	if f.Fin {
		typ ^= 0b1
	}
	hasOffset := f.Offset != 0
	if f.DataLenPresent {
		typ ^= 0b10
	}
	if hasOffset {
		typ ^= 0b100
	}
	b = append(b, typ)
	b = quicvarint.Append(b, uint64(f.StreamID))
	if hasOffset {
		b = quicvarint.Append(b, uint64(f.Offset))
	}
	if f.DataLenPresent {
		b = quicvarint.Append(b, uint64(f.DataLen()))
	}
	b = append(b, f.Data...)
	return b, nil
}

// Length returns the total length of the STREAM frame
func (f *StreamFrame) Length(version protocol.VersionNumber) protocol.ByteCount {
	length := 1 + quicvarint.Len(uint64(f.StreamID))
	if f.Offset != 0 {
		length += quicvarint.Len(uint64(f.Offset))
	}
	if f.DataLenPresent {
		length += quicvarint.Len(uint64(f.DataLen()))
	}
	return length + f.DataLen()
}

// DataLen gives the length of data in bytes
func (f *StreamFrame) DataLen() protocol.ByteCount {
	return protocol.ByteCount(len(f.Data))
}

// MaxDataLen returns the maximum data length
// If 0 is returned, writing will fail (a STREAM frame must contain at least 1 byte of data).
func (f *StreamFrame) MaxDataLen(maxSize protocol.ByteCount, version protocol.VersionNumber) protocol.ByteCount {
	headerLen := 1 + quicvarint.Len(uint64(f.StreamID))
	if f.Offset != 0 {
		headerLen += quicvarint.Len(uint64(f.Offset))
	}
	if f.DataLenPresent {
		// pretend that the data size will be 1 bytes
		// if it turns out that varint encoding the length will consume 2 bytes, we need to adjust the data length afterwards
		headerLen++
	}
	if headerLen > maxSize {
		return 0
	}
	maxDataLen := maxSize - headerLen
	if f.DataLenPresent && quicvarint.Len(uint64(maxDataLen)) != 1 {
		maxDataLen--
	}
	return maxDataLen
}

// MaybeSplitOffFrame splits a frame such that it is not bigger than n bytes.
// It returns if the frame was actually split.
// The frame might not be split if:
// * the size is large enough to fit the whole frame
// * the size is too small to fit even a 1-byte frame. In that case, the frame returned is nil.
func (f *StreamFrame) MaybeSplitOffFrame(maxSize protocol.ByteCount, version protocol.VersionNumber) (*StreamFrame, bool /* was splitting required */) {
	if maxSize >= f.Length(version) {
		return nil, false
	}

	n := f.MaxDataLen(maxSize, version)
	if n == 0 {
		return nil, true
	}

	new := GetStreamFrame()
	new.StreamID = f.StreamID
	new.Offset = f.Offset
	new.Fin = false
	new.DataLenPresent = f.DataLenPresent

	// swap the data slices
	new.Data, f.Data = f.Data, new.Data
	new.fromPool, f.fromPool = f.fromPool, new.fromPool

	f.Data = f.Data[:protocol.ByteCount(len(new.Data))-n]
	copy(f.Data, new.Data[n:])
	new.Data = new.Data[:n]
	f.Offset += n

	return new, true
}

func (f *StreamFrame) PutBack() {
	putStreamFrame(f)
}
