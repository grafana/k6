package wire

import (
	"bytes"
	"io"

	"github.com/quic-go/quic-go/internal/protocol"
)

// A PathResponseFrame is a PATH_RESPONSE frame
type PathResponseFrame struct {
	Data [8]byte
}

func parsePathResponseFrame(r *bytes.Reader, _ protocol.VersionNumber) (*PathResponseFrame, error) {
	frame := &PathResponseFrame{}
	if _, err := io.ReadFull(r, frame.Data[:]); err != nil {
		if err == io.ErrUnexpectedEOF {
			return nil, io.EOF
		}
		return nil, err
	}
	return frame, nil
}

func (f *PathResponseFrame) Append(b []byte, _ protocol.VersionNumber) ([]byte, error) {
	b = append(b, pathResponseFrameType)
	b = append(b, f.Data[:]...)
	return b, nil
}

// Length of a written frame
func (f *PathResponseFrame) Length(_ protocol.VersionNumber) protocol.ByteCount {
	return 1 + 8
}
