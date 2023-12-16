package wire

import (
	"bytes"
	"fmt"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/quicvarint"
)

// A MaxStreamsFrame is a MAX_STREAMS frame
type MaxStreamsFrame struct {
	Type         protocol.StreamType
	MaxStreamNum protocol.StreamNum
}

func parseMaxStreamsFrame(r *bytes.Reader, typ uint64, _ protocol.VersionNumber) (*MaxStreamsFrame, error) {
	f := &MaxStreamsFrame{}
	switch typ {
	case bidiMaxStreamsFrameType:
		f.Type = protocol.StreamTypeBidi
	case uniMaxStreamsFrameType:
		f.Type = protocol.StreamTypeUni
	}
	streamID, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	f.MaxStreamNum = protocol.StreamNum(streamID)
	if f.MaxStreamNum > protocol.MaxStreamCount {
		return nil, fmt.Errorf("%d exceeds the maximum stream count", f.MaxStreamNum)
	}
	return f, nil
}

func (f *MaxStreamsFrame) Append(b []byte, _ protocol.VersionNumber) ([]byte, error) {
	switch f.Type {
	case protocol.StreamTypeBidi:
		b = append(b, bidiMaxStreamsFrameType)
	case protocol.StreamTypeUni:
		b = append(b, uniMaxStreamsFrameType)
	}
	b = quicvarint.Append(b, uint64(f.MaxStreamNum))
	return b, nil
}

// Length of a written frame
func (f *MaxStreamsFrame) Length(protocol.VersionNumber) protocol.ByteCount {
	return 1 + quicvarint.Len(uint64(f.MaxStreamNum))
}
