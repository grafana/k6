package quic

import (
	"fmt"

	"github.com/quic-go/quic-go/internal/ackhandler"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/wire"
)

type retransmissionQueue struct {
	initial           []wire.Frame
	initialCryptoData []*wire.CryptoFrame

	handshake           []wire.Frame
	handshakeCryptoData []*wire.CryptoFrame

	appData []wire.Frame
}

func newRetransmissionQueue() *retransmissionQueue {
	return &retransmissionQueue{}
}

// AddPing queues a ping.
// It is used when a probe packet needs to be sent
func (q *retransmissionQueue) AddPing(encLevel protocol.EncryptionLevel) {
	//nolint:exhaustive // Cannot send probe packets for 0-RTT.
	switch encLevel {
	case protocol.EncryptionInitial:
		q.addInitial(&wire.PingFrame{})
	case protocol.EncryptionHandshake:
		q.addHandshake(&wire.PingFrame{})
	case protocol.Encryption1RTT:
		q.addAppData(&wire.PingFrame{})
	default:
		panic("unexpected encryption level")
	}
}

func (q *retransmissionQueue) addInitial(f wire.Frame) {
	if cf, ok := f.(*wire.CryptoFrame); ok {
		q.initialCryptoData = append(q.initialCryptoData, cf)
		return
	}
	q.initial = append(q.initial, f)
}

func (q *retransmissionQueue) addHandshake(f wire.Frame) {
	if cf, ok := f.(*wire.CryptoFrame); ok {
		q.handshakeCryptoData = append(q.handshakeCryptoData, cf)
		return
	}
	q.handshake = append(q.handshake, f)
}

func (q *retransmissionQueue) HasInitialData() bool {
	return len(q.initialCryptoData) > 0 || len(q.initial) > 0
}

func (q *retransmissionQueue) HasHandshakeData() bool {
	return len(q.handshakeCryptoData) > 0 || len(q.handshake) > 0
}

func (q *retransmissionQueue) HasAppData() bool {
	return len(q.appData) > 0
}

func (q *retransmissionQueue) addAppData(f wire.Frame) {
	if _, ok := f.(*wire.StreamFrame); ok {
		panic("STREAM frames are handled with their respective streams.")
	}
	q.appData = append(q.appData, f)
}

func (q *retransmissionQueue) GetInitialFrame(maxLen protocol.ByteCount, v protocol.VersionNumber) wire.Frame {
	if len(q.initialCryptoData) > 0 {
		f := q.initialCryptoData[0]
		newFrame, needsSplit := f.MaybeSplitOffFrame(maxLen, v)
		if newFrame == nil && !needsSplit { // the whole frame fits
			q.initialCryptoData = q.initialCryptoData[1:]
			return f
		}
		if newFrame != nil { // frame was split. Leave the original frame in the queue.
			return newFrame
		}
	}
	if len(q.initial) == 0 {
		return nil
	}
	f := q.initial[0]
	if f.Length(v) > maxLen {
		return nil
	}
	q.initial = q.initial[1:]
	return f
}

func (q *retransmissionQueue) GetHandshakeFrame(maxLen protocol.ByteCount, v protocol.VersionNumber) wire.Frame {
	if len(q.handshakeCryptoData) > 0 {
		f := q.handshakeCryptoData[0]
		newFrame, needsSplit := f.MaybeSplitOffFrame(maxLen, v)
		if newFrame == nil && !needsSplit { // the whole frame fits
			q.handshakeCryptoData = q.handshakeCryptoData[1:]
			return f
		}
		if newFrame != nil { // frame was split. Leave the original frame in the queue.
			return newFrame
		}
	}
	if len(q.handshake) == 0 {
		return nil
	}
	f := q.handshake[0]
	if f.Length(v) > maxLen {
		return nil
	}
	q.handshake = q.handshake[1:]
	return f
}

func (q *retransmissionQueue) GetAppDataFrame(maxLen protocol.ByteCount, v protocol.VersionNumber) wire.Frame {
	if len(q.appData) == 0 {
		return nil
	}
	f := q.appData[0]
	if f.Length(v) > maxLen {
		return nil
	}
	q.appData = q.appData[1:]
	return f
}

func (q *retransmissionQueue) DropPackets(encLevel protocol.EncryptionLevel) {
	//nolint:exhaustive // Can only drop Initial and Handshake packet number space.
	switch encLevel {
	case protocol.EncryptionInitial:
		q.initial = nil
		q.initialCryptoData = nil
	case protocol.EncryptionHandshake:
		q.handshake = nil
		q.handshakeCryptoData = nil
	default:
		panic(fmt.Sprintf("unexpected encryption level: %s", encLevel))
	}
}

func (q *retransmissionQueue) InitialAckHandler() ackhandler.FrameHandler {
	return (*retransmissionQueueInitialAckHandler)(q)
}

func (q *retransmissionQueue) HandshakeAckHandler() ackhandler.FrameHandler {
	return (*retransmissionQueueHandshakeAckHandler)(q)
}

func (q *retransmissionQueue) AppDataAckHandler() ackhandler.FrameHandler {
	return (*retransmissionQueueAppDataAckHandler)(q)
}

type retransmissionQueueInitialAckHandler retransmissionQueue

func (q *retransmissionQueueInitialAckHandler) OnAcked(wire.Frame) {}
func (q *retransmissionQueueInitialAckHandler) OnLost(f wire.Frame) {
	(*retransmissionQueue)(q).addInitial(f)
}

type retransmissionQueueHandshakeAckHandler retransmissionQueue

func (q *retransmissionQueueHandshakeAckHandler) OnAcked(wire.Frame) {}
func (q *retransmissionQueueHandshakeAckHandler) OnLost(f wire.Frame) {
	(*retransmissionQueue)(q).addHandshake(f)
}

type retransmissionQueueAppDataAckHandler retransmissionQueue

func (q *retransmissionQueueAppDataAckHandler) OnAcked(wire.Frame) {}
func (q *retransmissionQueueAppDataAckHandler) OnLost(f wire.Frame) {
	(*retransmissionQueue)(q).addAppData(f)
}
