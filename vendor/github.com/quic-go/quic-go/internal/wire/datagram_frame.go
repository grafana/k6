package wire

import (
	"bytes"
	"io"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/quicvarint"
)

// A DatagramFrame is a DATAGRAM frame
type DatagramFrame struct {
	DataLenPresent bool
	Data           []byte
}

func parseDatagramFrame(r *bytes.Reader, typ uint64, _ protocol.VersionNumber) (*DatagramFrame, error) {
	f := &DatagramFrame{}
	f.DataLenPresent = typ&0x1 > 0

	var length uint64
	if f.DataLenPresent {
		var err error
		len, err := quicvarint.Read(r)
		if err != nil {
			return nil, err
		}
		if len > uint64(r.Len()) {
			return nil, io.EOF
		}
		length = len
	} else {
		length = uint64(r.Len())
	}
	f.Data = make([]byte, length)
	if _, err := io.ReadFull(r, f.Data); err != nil {
		return nil, err
	}
	return f, nil
}

func (f *DatagramFrame) Append(b []byte, _ protocol.VersionNumber) ([]byte, error) {
	typ := uint8(0x30)
	if f.DataLenPresent {
		typ ^= 0b1
	}
	b = append(b, typ)
	if f.DataLenPresent {
		b = quicvarint.Append(b, uint64(len(f.Data)))
	}
	b = append(b, f.Data...)
	return b, nil
}

// MaxDataLen returns the maximum data length
func (f *DatagramFrame) MaxDataLen(maxSize protocol.ByteCount, version protocol.VersionNumber) protocol.ByteCount {
	headerLen := protocol.ByteCount(1)
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

// Length of a written frame
func (f *DatagramFrame) Length(_ protocol.VersionNumber) protocol.ByteCount {
	length := 1 + protocol.ByteCount(len(f.Data))
	if f.DataLenPresent {
		length += quicvarint.Len(uint64(len(f.Data)))
	}
	return length
}
