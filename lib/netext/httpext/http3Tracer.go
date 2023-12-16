package httpext

import (
	"context"
	"net"
	"time"

	"github.com/quic-go/quic-go/logging"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"
)

type streamMetrics struct {
	requestStart  time.Time
	requestFin    time.Time
	responseStart time.Time
	responseFin   time.Time
	receivedBytes int
	sentBytes     int
}

type connectionMetrics struct {
	connectionStart time.Time
	handshakeStart  time.Time
	handshakeDone   time.Time
	dataReceived    int
	dataSent        int
}
type metricHandler struct {
	vu                modules.VU
	metrics           *HTTP3Metrics
	connectionMetrics *connectionMetrics
	streams           map[int64]*streamMetrics
}

func (mh *metricHandler) sendConnectionMetrics(ctx context.Context) {
	handshakeTime := mh.connectionMetrics.handshakeDone.Sub(mh.connectionMetrics.handshakeStart)
	connectTime := mh.connectionMetrics.handshakeStart.Sub(mh.connectionMetrics.connectionStart)

	samples := metrics.ConnectedSamples{
		Samples: []metrics.Sample{
			{
				TimeSeries: metrics.TimeSeries{
					Metric: mh.metrics.HTTP3ReqTLSHandshaking,
					Tags:   nil,
				},
				Metadata: nil,
				Time:     mh.connectionMetrics.handshakeDone,
				Value:    metrics.D(handshakeTime),
			},
			{
				TimeSeries: metrics.TimeSeries{
					Metric: mh.metrics.HTTP3ReqConnecting,
					Tags:   nil,
				},
				Metadata: nil,
				Time:     mh.connectionMetrics.handshakeStart,
				Value:    metrics.D(connectTime),
			},
		},
	}
	mh.vu.State().Samples <- samples
	mh.sendDataMetrics(ctx)
}

func (mh *metricHandler) sendDataMetrics(ctx context.Context) {
	samples := metrics.ConnectedSamples{
		Samples: []metrics.Sample{
			{
				TimeSeries: metrics.TimeSeries{
					Metric: mh.vu.State().BuiltinMetrics.DataReceived,
					Tags:   mh.vu.State().Tags.GetCurrentValues().Tags,
				},
				Time:     time.Now(),
				Value:    float64(mh.connectionMetrics.dataReceived),
				Metadata: mh.vu.State().Tags.GetCurrentValues().Metadata,
			},
			{
				TimeSeries: metrics.TimeSeries{
					Metric: mh.vu.State().BuiltinMetrics.DataSent,
					Tags:   mh.vu.State().Tags.GetCurrentValues().Tags,
				},
				Time:     time.Now(),
				Value:    float64(mh.connectionMetrics.dataSent),
				Metadata: mh.vu.State().Tags.GetCurrentValues().Metadata,
			},
		},
	}
	metrics.PushIfNotDone(ctx, mh.vu.State().Samples, samples)
	mh.connectionMetrics.dataReceived = 0
	mh.connectionMetrics.dataSent = 0
}

func (mh *metricHandler) sendStreamMetrics(ctx context.Context, streamID int64) {
	streamMetrics := mh.getStreamMetrics(streamID, false)
	if streamMetrics == nil {
		return
	}
	delete(mh.streams, streamID)
	http3ReqDuration := streamMetrics.responseFin.Sub(streamMetrics.requestStart)
	http3ReqSending := streamMetrics.requestFin.Sub(streamMetrics.requestStart)
	http3ReqWaiting := streamMetrics.responseStart.Sub(streamMetrics.requestFin)
	http3ReqReceiving := streamMetrics.responseFin.Sub(streamMetrics.responseStart)

	tags := mh.vu.State().Tags.GetCurrentValues().Tags
	metadata := mh.vu.State().Tags.GetCurrentValues().Metadata

	samples := metrics.ConnectedSamples{
		Samples: []metrics.Sample{
			{
				TimeSeries: metrics.TimeSeries{
					Metric: mh.metrics.HTTP3ReqDuration,
					Tags:   tags,
				},
				Metadata: metadata,
				Time:     streamMetrics.responseFin,
				Value:    metrics.D(http3ReqDuration),
			},
			{
				TimeSeries: metrics.TimeSeries{
					Metric: mh.metrics.HTTP3ReqSending,
					Tags:   tags,
				},
				Metadata: metadata,
				Time:     streamMetrics.requestFin,
				Value:    metrics.D(http3ReqSending),
			},
			{
				TimeSeries: metrics.TimeSeries{
					Metric: mh.metrics.HTTP3ReqWaiting,
					Tags:   tags,
				},
				Metadata: metadata,
				Time:     streamMetrics.responseStart,
				Value:    metrics.D(http3ReqWaiting),
			},
			{
				TimeSeries: metrics.TimeSeries{
					Metric: mh.metrics.HTTP3ReqReceiving,
					Tags:   tags,
				},
				Metadata: metadata,
				Time:     streamMetrics.responseFin,
				Value:    metrics.D(http3ReqReceiving),
			},
			{
				TimeSeries: metrics.TimeSeries{
					Metric: mh.metrics.HTTP3Reqs,
					Tags:   tags,
				},
				Metadata: metadata,
				Time:     streamMetrics.responseFin,
				Value:    1,
			},
		},
	}
	mh.vu.State().Samples <- samples
	mh.sendDataMetrics(ctx)
}

func (mh *metricHandler) handleFramesReceived(ctx context.Context, frames []logging.Frame) {
	for _, frame := range frames {
		switch f := frame.(type) {
		case *logging.HandshakeDoneFrame:
			{
				mh.connectionMetrics.handshakeDone = time.Now()
				mh.sendConnectionMetrics(ctx)
			}
		case *logging.StreamFrame:
			{
				streamID := int64(f.StreamID)
				streamMetrics := mh.getStreamMetrics(streamID, false)
				if streamMetrics == nil {
					return
				}
				streamMetrics.receivedBytes += int(f.Length)
				if streamMetrics.responseStart.IsZero() {
					streamMetrics.responseStart = time.Now()
				}
				if f.Fin {
					streamMetrics.responseFin = time.Now()
					mh.sendStreamMetrics(ctx, streamID)
				}
			}
		}
	}
}

func (mh *metricHandler) handleFramesSent(frames []logging.Frame) {
	for _, frame := range frames {
		if f, ok := frame.(*logging.StreamFrame); ok {
			streamID := int64(f.StreamID)
			streamMetrics := mh.getStreamMetrics(streamID, true)
			streamMetrics.sentBytes += int(f.Length)
			if f.Fin {
				streamMetrics.requestFin = time.Now()
			}
		}
	}
}

func (mh *metricHandler) packetReceived(ctx context.Context, bc logging.ByteCount, f []logging.Frame) {
	mh.handleFramesReceived(ctx, f)
	mh.connectionMetrics.dataReceived += int(bc)
}

func (mh *metricHandler) packetSent(bc logging.ByteCount, f []logging.Frame) {
	mh.handleFramesSent(f)
	mh.connectionMetrics.dataSent += int(bc)
}

func (mh *metricHandler) getStreamMetrics(streamID int64, createIfNotFound bool) *streamMetrics {
	now := time.Now()
	if m, ok := mh.streams[streamID]; ok || !createIfNotFound {
		return m
	}
	m := &streamMetrics{
		requestStart: now,
	}
	mh.streams[streamID] = m
	return m
}

// NewTracer creates a new connection tracer for the given VU, that will measure and emit different metrics
func NewTracer(ctx context.Context, vu modules.VU, http3Metrics *HTTP3Metrics) *logging.ConnectionTracer {
	mh := &metricHandler{
		vu:                vu,
		metrics:           http3Metrics,
		connectionMetrics: &connectionMetrics{},
		streams:           make(map[int64]*streamMetrics),
	}

	return &logging.ConnectionTracer{
		StartedConnection: func(local, remote net.Addr, srcConnID, destConnID logging.ConnectionID) {
			mh.connectionMetrics.connectionStart = time.Now()
		},
		ReceivedLongHeaderPacket: func(eh *logging.ExtendedHeader, bc logging.ByteCount, e logging.ECN, f []logging.Frame) {
			if mh.connectionMetrics.handshakeStart.IsZero() && eh.Type == 3 /*protocol.PacketTypeHandshake*/ {
				mh.connectionMetrics.handshakeStart = time.Now()
			}
			mh.packetReceived(ctx, bc, f)
		},
		ReceivedShortHeaderPacket: func(sh *logging.ShortHeader, bc logging.ByteCount, e logging.ECN, f []logging.Frame) {
			mh.packetReceived(ctx, bc, f)
		},
		SentLongHeaderPacket: func(eh *logging.ExtendedHeader,
			bc logging.ByteCount,
			e logging.ECN,
			af *logging.AckFrame,
			f []logging.Frame,
		) {
			mh.packetSent(bc, f)
		},
		SentShortHeaderPacket: func(sh *logging.ShortHeader,
			bc logging.ByteCount,
			e logging.ECN,
			af *logging.AckFrame,
			f []logging.Frame,
		) {
			mh.packetSent(bc, f)
		},
	}
}
