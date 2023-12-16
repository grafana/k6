package wire

import (
	"bytes"
	"fmt"
	"io"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/quicvarint"
)

// A NewConnectionIDFrame is a NEW_CONNECTION_ID frame
type NewConnectionIDFrame struct {
	SequenceNumber      uint64
	RetirePriorTo       uint64
	ConnectionID        protocol.ConnectionID
	StatelessResetToken protocol.StatelessResetToken
}

func parseNewConnectionIDFrame(r *bytes.Reader, _ protocol.VersionNumber) (*NewConnectionIDFrame, error) {
	seq, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	ret, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	if ret > seq {
		//nolint:stylecheck
		return nil, fmt.Errorf("Retire Prior To value (%d) larger than Sequence Number (%d)", ret, seq)
	}
	connIDLen, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	connID, err := protocol.ReadConnectionID(r, int(connIDLen))
	if err != nil {
		return nil, err
	}
	frame := &NewConnectionIDFrame{
		SequenceNumber: seq,
		RetirePriorTo:  ret,
		ConnectionID:   connID,
	}
	if _, err := io.ReadFull(r, frame.StatelessResetToken[:]); err != nil {
		if err == io.ErrUnexpectedEOF {
			return nil, io.EOF
		}
		return nil, err
	}

	return frame, nil
}

func (f *NewConnectionIDFrame) Append(b []byte, _ protocol.VersionNumber) ([]byte, error) {
	b = append(b, newConnectionIDFrameType)
	b = quicvarint.Append(b, f.SequenceNumber)
	b = quicvarint.Append(b, f.RetirePriorTo)
	connIDLen := f.ConnectionID.Len()
	if connIDLen > protocol.MaxConnIDLen {
		return nil, fmt.Errorf("invalid connection ID length: %d", connIDLen)
	}
	b = append(b, uint8(connIDLen))
	b = append(b, f.ConnectionID.Bytes()...)
	b = append(b, f.StatelessResetToken[:]...)
	return b, nil
}

// Length of a written frame
func (f *NewConnectionIDFrame) Length(protocol.VersionNumber) protocol.ByteCount {
	return 1 + quicvarint.Len(f.SequenceNumber) + quicvarint.Len(f.RetirePriorTo) + 1 /* connection ID length */ + protocol.ByteCount(f.ConnectionID.Len()) + 16
}
