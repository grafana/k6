package ackhandler

import (
	"fmt"
	"time"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/internal/wire"
)

type receivedPacketHandler struct {
	sentPackets sentPacketTracker

	initialPackets   *receivedPacketTracker
	handshakePackets *receivedPacketTracker
	appDataPackets   *receivedPacketTracker

	lowest1RTTPacket protocol.PacketNumber
}

var _ ReceivedPacketHandler = &receivedPacketHandler{}

func newReceivedPacketHandler(
	sentPackets sentPacketTracker,
	rttStats *utils.RTTStats,
	logger utils.Logger,
) ReceivedPacketHandler {
	return &receivedPacketHandler{
		sentPackets:      sentPackets,
		initialPackets:   newReceivedPacketTracker(rttStats, logger),
		handshakePackets: newReceivedPacketTracker(rttStats, logger),
		appDataPackets:   newReceivedPacketTracker(rttStats, logger),
		lowest1RTTPacket: protocol.InvalidPacketNumber,
	}
}

func (h *receivedPacketHandler) ReceivedPacket(
	pn protocol.PacketNumber,
	ecn protocol.ECN,
	encLevel protocol.EncryptionLevel,
	rcvTime time.Time,
	ackEliciting bool,
) error {
	h.sentPackets.ReceivedPacket(encLevel)
	switch encLevel {
	case protocol.EncryptionInitial:
		return h.initialPackets.ReceivedPacket(pn, ecn, rcvTime, ackEliciting)
	case protocol.EncryptionHandshake:
		// The Handshake packet number space might already have been dropped as a result
		// of processing the CRYPTO frame that was contained in this packet.
		if h.handshakePackets == nil {
			return nil
		}
		return h.handshakePackets.ReceivedPacket(pn, ecn, rcvTime, ackEliciting)
	case protocol.Encryption0RTT:
		if h.lowest1RTTPacket != protocol.InvalidPacketNumber && pn > h.lowest1RTTPacket {
			return fmt.Errorf("received packet number %d on a 0-RTT packet after receiving %d on a 1-RTT packet", pn, h.lowest1RTTPacket)
		}
		return h.appDataPackets.ReceivedPacket(pn, ecn, rcvTime, ackEliciting)
	case protocol.Encryption1RTT:
		if h.lowest1RTTPacket == protocol.InvalidPacketNumber || pn < h.lowest1RTTPacket {
			h.lowest1RTTPacket = pn
		}
		if err := h.appDataPackets.ReceivedPacket(pn, ecn, rcvTime, ackEliciting); err != nil {
			return err
		}
		h.appDataPackets.IgnoreBelow(h.sentPackets.GetLowestPacketNotConfirmedAcked())
		return nil
	default:
		panic(fmt.Sprintf("received packet with unknown encryption level: %s", encLevel))
	}
}

func (h *receivedPacketHandler) DropPackets(encLevel protocol.EncryptionLevel) {
	//nolint:exhaustive // 1-RTT packet number space is never dropped.
	switch encLevel {
	case protocol.EncryptionInitial:
		h.initialPackets = nil
	case protocol.EncryptionHandshake:
		h.handshakePackets = nil
	case protocol.Encryption0RTT:
		// Nothing to do here.
		// If we are rejecting 0-RTT, no 0-RTT packets will have been decrypted.
	default:
		panic(fmt.Sprintf("Cannot drop keys for encryption level %s", encLevel))
	}
}

func (h *receivedPacketHandler) GetAlarmTimeout() time.Time {
	var initialAlarm, handshakeAlarm time.Time
	if h.initialPackets != nil {
		initialAlarm = h.initialPackets.GetAlarmTimeout()
	}
	if h.handshakePackets != nil {
		handshakeAlarm = h.handshakePackets.GetAlarmTimeout()
	}
	oneRTTAlarm := h.appDataPackets.GetAlarmTimeout()
	return utils.MinNonZeroTime(utils.MinNonZeroTime(initialAlarm, handshakeAlarm), oneRTTAlarm)
}

func (h *receivedPacketHandler) GetAckFrame(encLevel protocol.EncryptionLevel, onlyIfQueued bool) *wire.AckFrame {
	var ack *wire.AckFrame
	//nolint:exhaustive // 0-RTT packets can't contain ACK frames.
	switch encLevel {
	case protocol.EncryptionInitial:
		if h.initialPackets != nil {
			ack = h.initialPackets.GetAckFrame(onlyIfQueued)
		}
	case protocol.EncryptionHandshake:
		if h.handshakePackets != nil {
			ack = h.handshakePackets.GetAckFrame(onlyIfQueued)
		}
	case protocol.Encryption1RTT:
		// 0-RTT packets can't contain ACK frames
		return h.appDataPackets.GetAckFrame(onlyIfQueued)
	default:
		return nil
	}
	// For Initial and Handshake ACKs, the delay time is ignored by the receiver.
	// Set it to 0 in order to save bytes.
	if ack != nil {
		ack.DelayTime = 0
	}
	return ack
}

func (h *receivedPacketHandler) IsPotentiallyDuplicate(pn protocol.PacketNumber, encLevel protocol.EncryptionLevel) bool {
	switch encLevel {
	case protocol.EncryptionInitial:
		if h.initialPackets != nil {
			return h.initialPackets.IsPotentiallyDuplicate(pn)
		}
	case protocol.EncryptionHandshake:
		if h.handshakePackets != nil {
			return h.handshakePackets.IsPotentiallyDuplicate(pn)
		}
	case protocol.Encryption0RTT, protocol.Encryption1RTT:
		return h.appDataPackets.IsPotentiallyDuplicate(pn)
	}
	panic("unexpected encryption level")
}
