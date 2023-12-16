package wire

import (
	"bytes"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/quicvarint"
)

// A DataBlockedFrame is a DATA_BLOCKED frame
type DataBlockedFrame struct {
	MaximumData protocol.ByteCount
}

func parseDataBlockedFrame(r *bytes.Reader, _ protocol.VersionNumber) (*DataBlockedFrame, error) {
	offset, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	return &DataBlockedFrame{MaximumData: protocol.ByteCount(offset)}, nil
}

func (f *DataBlockedFrame) Append(b []byte, version protocol.VersionNumber) ([]byte, error) {
	b = append(b, dataBlockedFrameType)
	return quicvarint.Append(b, uint64(f.MaximumData)), nil
}

// Length of a written frame
func (f *DataBlockedFrame) Length(version protocol.VersionNumber) protocol.ByteCount {
	return 1 + quicvarint.Len(uint64(f.MaximumData))
}
