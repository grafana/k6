package wire

import (
	"bytes"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/quicvarint"
)

// A MaxDataFrame carries flow control information for the connection
type MaxDataFrame struct {
	MaximumData protocol.ByteCount
}

// parseMaxDataFrame parses a MAX_DATA frame
func parseMaxDataFrame(r *bytes.Reader, _ protocol.VersionNumber) (*MaxDataFrame, error) {
	frame := &MaxDataFrame{}
	byteOffset, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	frame.MaximumData = protocol.ByteCount(byteOffset)
	return frame, nil
}

func (f *MaxDataFrame) Append(b []byte, _ protocol.VersionNumber) ([]byte, error) {
	b = append(b, maxDataFrameType)
	b = quicvarint.Append(b, uint64(f.MaximumData))
	return b, nil
}

// Length of a written frame
func (f *MaxDataFrame) Length(_ protocol.VersionNumber) protocol.ByteCount {
	return 1 + quicvarint.Len(uint64(f.MaximumData))
}
