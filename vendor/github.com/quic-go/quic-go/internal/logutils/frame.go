package logutils

import (
	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/wire"
	"github.com/quic-go/quic-go/logging"
)

// ConvertFrame converts a wire.Frame into a logging.Frame.
// This makes it possible for external packages to access the frames.
// Furthermore, it removes the data slices from CRYPTO and STREAM frames.
func ConvertFrame(frame wire.Frame) logging.Frame {
	switch f := frame.(type) {
	case *wire.AckFrame:
		// We use a pool for ACK frames.
		// Implementations of the tracer interface may hold on to frames, so we need to make a copy here.
		return ConvertAckFrame(f)
	case *wire.CryptoFrame:
		return &logging.CryptoFrame{
			Offset: f.Offset,
			Length: protocol.ByteCount(len(f.Data)),
		}
	case *wire.StreamFrame:
		return &logging.StreamFrame{
			StreamID: f.StreamID,
			Offset:   f.Offset,
			Length:   f.DataLen(),
			Fin:      f.Fin,
		}
	case *wire.DatagramFrame:
		return &logging.DatagramFrame{
			Length: logging.ByteCount(len(f.Data)),
		}
	default:
		return logging.Frame(frame)
	}
}

func ConvertAckFrame(f *wire.AckFrame) *logging.AckFrame {
	ranges := make([]wire.AckRange, 0, len(f.AckRanges))
	ranges = append(ranges, f.AckRanges...)
	ack := &logging.AckFrame{
		AckRanges: ranges,
		DelayTime: f.DelayTime,
		ECNCE:     f.ECNCE,
		ECT0:      f.ECT0,
		ECT1:      f.ECT1,
	}
	return ack
}
