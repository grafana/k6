package qlog

import (
	"fmt"

	"github.com/quic-go/quic-go/internal/wire"
	"github.com/quic-go/quic-go/logging"

	"github.com/francoispqt/gojay"
)

type frame struct {
	Frame logging.Frame
}

var _ gojay.MarshalerJSONObject = frame{}

var _ gojay.MarshalerJSONArray = frames{}

func (f frame) MarshalJSONObject(enc *gojay.Encoder) {
	switch frame := f.Frame.(type) {
	case *logging.PingFrame:
		marshalPingFrame(enc, frame)
	case *logging.AckFrame:
		marshalAckFrame(enc, frame)
	case *logging.ResetStreamFrame:
		marshalResetStreamFrame(enc, frame)
	case *logging.StopSendingFrame:
		marshalStopSendingFrame(enc, frame)
	case *logging.CryptoFrame:
		marshalCryptoFrame(enc, frame)
	case *logging.NewTokenFrame:
		marshalNewTokenFrame(enc, frame)
	case *logging.StreamFrame:
		marshalStreamFrame(enc, frame)
	case *logging.MaxDataFrame:
		marshalMaxDataFrame(enc, frame)
	case *logging.MaxStreamDataFrame:
		marshalMaxStreamDataFrame(enc, frame)
	case *logging.MaxStreamsFrame:
		marshalMaxStreamsFrame(enc, frame)
	case *logging.DataBlockedFrame:
		marshalDataBlockedFrame(enc, frame)
	case *logging.StreamDataBlockedFrame:
		marshalStreamDataBlockedFrame(enc, frame)
	case *logging.StreamsBlockedFrame:
		marshalStreamsBlockedFrame(enc, frame)
	case *logging.NewConnectionIDFrame:
		marshalNewConnectionIDFrame(enc, frame)
	case *logging.RetireConnectionIDFrame:
		marshalRetireConnectionIDFrame(enc, frame)
	case *logging.PathChallengeFrame:
		marshalPathChallengeFrame(enc, frame)
	case *logging.PathResponseFrame:
		marshalPathResponseFrame(enc, frame)
	case *logging.ConnectionCloseFrame:
		marshalConnectionCloseFrame(enc, frame)
	case *logging.HandshakeDoneFrame:
		marshalHandshakeDoneFrame(enc, frame)
	case *logging.DatagramFrame:
		marshalDatagramFrame(enc, frame)
	default:
		panic("unknown frame type")
	}
}

func (f frame) IsNil() bool { return false }

type frames []frame

func (fs frames) IsNil() bool { return fs == nil }
func (fs frames) MarshalJSONArray(enc *gojay.Encoder) {
	for _, f := range fs {
		enc.Object(f)
	}
}

func marshalPingFrame(enc *gojay.Encoder, _ *wire.PingFrame) {
	enc.StringKey("frame_type", "ping")
}

type ackRanges []wire.AckRange

func (ars ackRanges) MarshalJSONArray(enc *gojay.Encoder) {
	for _, r := range ars {
		enc.Array(ackRange(r))
	}
}

func (ars ackRanges) IsNil() bool { return false }

type ackRange wire.AckRange

func (ar ackRange) MarshalJSONArray(enc *gojay.Encoder) {
	enc.AddInt64(int64(ar.Smallest))
	if ar.Smallest != ar.Largest {
		enc.AddInt64(int64(ar.Largest))
	}
}

func (ar ackRange) IsNil() bool { return false }

func marshalAckFrame(enc *gojay.Encoder, f *logging.AckFrame) {
	enc.StringKey("frame_type", "ack")
	enc.FloatKeyOmitEmpty("ack_delay", milliseconds(f.DelayTime))
	enc.ArrayKey("acked_ranges", ackRanges(f.AckRanges))
	if hasECN := f.ECT0 > 0 || f.ECT1 > 0 || f.ECNCE > 0; hasECN {
		enc.Uint64Key("ect0", f.ECT0)
		enc.Uint64Key("ect1", f.ECT1)
		enc.Uint64Key("ce", f.ECNCE)
	}
}

func marshalResetStreamFrame(enc *gojay.Encoder, f *logging.ResetStreamFrame) {
	enc.StringKey("frame_type", "reset_stream")
	enc.Int64Key("stream_id", int64(f.StreamID))
	enc.Int64Key("error_code", int64(f.ErrorCode))
	enc.Int64Key("final_size", int64(f.FinalSize))
}

func marshalStopSendingFrame(enc *gojay.Encoder, f *logging.StopSendingFrame) {
	enc.StringKey("frame_type", "stop_sending")
	enc.Int64Key("stream_id", int64(f.StreamID))
	enc.Int64Key("error_code", int64(f.ErrorCode))
}

func marshalCryptoFrame(enc *gojay.Encoder, f *logging.CryptoFrame) {
	enc.StringKey("frame_type", "crypto")
	enc.Int64Key("offset", int64(f.Offset))
	enc.Int64Key("length", int64(f.Length))
}

func marshalNewTokenFrame(enc *gojay.Encoder, f *logging.NewTokenFrame) {
	enc.StringKey("frame_type", "new_token")
	enc.ObjectKey("token", &token{Raw: f.Token})
}

func marshalStreamFrame(enc *gojay.Encoder, f *logging.StreamFrame) {
	enc.StringKey("frame_type", "stream")
	enc.Int64Key("stream_id", int64(f.StreamID))
	enc.Int64Key("offset", int64(f.Offset))
	enc.IntKey("length", int(f.Length))
	enc.BoolKeyOmitEmpty("fin", f.Fin)
}

func marshalMaxDataFrame(enc *gojay.Encoder, f *logging.MaxDataFrame) {
	enc.StringKey("frame_type", "max_data")
	enc.Int64Key("maximum", int64(f.MaximumData))
}

func marshalMaxStreamDataFrame(enc *gojay.Encoder, f *logging.MaxStreamDataFrame) {
	enc.StringKey("frame_type", "max_stream_data")
	enc.Int64Key("stream_id", int64(f.StreamID))
	enc.Int64Key("maximum", int64(f.MaximumStreamData))
}

func marshalMaxStreamsFrame(enc *gojay.Encoder, f *logging.MaxStreamsFrame) {
	enc.StringKey("frame_type", "max_streams")
	enc.StringKey("stream_type", streamType(f.Type).String())
	enc.Int64Key("maximum", int64(f.MaxStreamNum))
}

func marshalDataBlockedFrame(enc *gojay.Encoder, f *logging.DataBlockedFrame) {
	enc.StringKey("frame_type", "data_blocked")
	enc.Int64Key("limit", int64(f.MaximumData))
}

func marshalStreamDataBlockedFrame(enc *gojay.Encoder, f *logging.StreamDataBlockedFrame) {
	enc.StringKey("frame_type", "stream_data_blocked")
	enc.Int64Key("stream_id", int64(f.StreamID))
	enc.Int64Key("limit", int64(f.MaximumStreamData))
}

func marshalStreamsBlockedFrame(enc *gojay.Encoder, f *logging.StreamsBlockedFrame) {
	enc.StringKey("frame_type", "streams_blocked")
	enc.StringKey("stream_type", streamType(f.Type).String())
	enc.Int64Key("limit", int64(f.StreamLimit))
}

func marshalNewConnectionIDFrame(enc *gojay.Encoder, f *logging.NewConnectionIDFrame) {
	enc.StringKey("frame_type", "new_connection_id")
	enc.Int64Key("sequence_number", int64(f.SequenceNumber))
	enc.Int64Key("retire_prior_to", int64(f.RetirePriorTo))
	enc.IntKey("length", f.ConnectionID.Len())
	enc.StringKey("connection_id", f.ConnectionID.String())
	enc.StringKey("stateless_reset_token", fmt.Sprintf("%x", f.StatelessResetToken))
}

func marshalRetireConnectionIDFrame(enc *gojay.Encoder, f *logging.RetireConnectionIDFrame) {
	enc.StringKey("frame_type", "retire_connection_id")
	enc.Int64Key("sequence_number", int64(f.SequenceNumber))
}

func marshalPathChallengeFrame(enc *gojay.Encoder, f *logging.PathChallengeFrame) {
	enc.StringKey("frame_type", "path_challenge")
	enc.StringKey("data", fmt.Sprintf("%x", f.Data[:]))
}

func marshalPathResponseFrame(enc *gojay.Encoder, f *logging.PathResponseFrame) {
	enc.StringKey("frame_type", "path_response")
	enc.StringKey("data", fmt.Sprintf("%x", f.Data[:]))
}

func marshalConnectionCloseFrame(enc *gojay.Encoder, f *logging.ConnectionCloseFrame) {
	errorSpace := "transport"
	if f.IsApplicationError {
		errorSpace = "application"
	}
	enc.StringKey("frame_type", "connection_close")
	enc.StringKey("error_space", errorSpace)
	if errName := transportError(f.ErrorCode).String(); len(errName) > 0 {
		enc.StringKey("error_code", errName)
	} else {
		enc.Uint64Key("error_code", f.ErrorCode)
	}
	enc.Uint64Key("raw_error_code", f.ErrorCode)
	enc.StringKey("reason", f.ReasonPhrase)
}

func marshalHandshakeDoneFrame(enc *gojay.Encoder, _ *logging.HandshakeDoneFrame) {
	enc.StringKey("frame_type", "handshake_done")
}

func marshalDatagramFrame(enc *gojay.Encoder, f *logging.DatagramFrame) {
	enc.StringKey("frame_type", "datagram")
	enc.Int64Key("length", int64(f.Length))
}
