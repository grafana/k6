package ackhandler

import (
	"fmt"
	"time"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/internal/wire"
)

// number of ack-eliciting packets received before sending an ack.
const packetsBeforeAck = 2

type receivedPacketTracker struct {
	largestObserved         protocol.PacketNumber
	ignoreBelow             protocol.PacketNumber
	largestObservedRcvdTime time.Time
	ect0, ect1, ecnce       uint64

	packetHistory *receivedPacketHistory

	maxAckDelay time.Duration
	rttStats    *utils.RTTStats

	hasNewAck bool // true as soon as we received an ack-eliciting new packet
	ackQueued bool // true once we received more than 2 (or later in the connection 10) ack-eliciting packets

	ackElicitingPacketsReceivedSinceLastAck int
	ackAlarm                                time.Time
	lastAck                                 *wire.AckFrame

	logger utils.Logger
}

func newReceivedPacketTracker(
	rttStats *utils.RTTStats,
	logger utils.Logger,
) *receivedPacketTracker {
	return &receivedPacketTracker{
		packetHistory: newReceivedPacketHistory(),
		maxAckDelay:   protocol.MaxAckDelay,
		rttStats:      rttStats,
		logger:        logger,
	}
}

func (h *receivedPacketTracker) ReceivedPacket(pn protocol.PacketNumber, ecn protocol.ECN, rcvTime time.Time, ackEliciting bool) error {
	if isNew := h.packetHistory.ReceivedPacket(pn); !isNew {
		return fmt.Errorf("recevedPacketTracker BUG: ReceivedPacket called for old / duplicate packet %d", pn)
	}

	isMissing := h.isMissing(pn)
	if pn >= h.largestObserved {
		h.largestObserved = pn
		h.largestObservedRcvdTime = rcvTime
	}

	if ackEliciting {
		h.hasNewAck = true
	}
	if ackEliciting {
		h.maybeQueueACK(pn, rcvTime, isMissing)
	}
	//nolint:exhaustive // Only need to count ECT(0), ECT(1) and ECNCE.
	switch ecn {
	case protocol.ECT0:
		h.ect0++
	case protocol.ECT1:
		h.ect1++
	case protocol.ECNCE:
		h.ecnce++
	}
	return nil
}

// IgnoreBelow sets a lower limit for acknowledging packets.
// Packets with packet numbers smaller than p will not be acked.
func (h *receivedPacketTracker) IgnoreBelow(pn protocol.PacketNumber) {
	if pn <= h.ignoreBelow {
		return
	}
	h.ignoreBelow = pn
	h.packetHistory.DeleteBelow(pn)
	if h.logger.Debug() {
		h.logger.Debugf("\tIgnoring all packets below %d.", pn)
	}
}

// isMissing says if a packet was reported missing in the last ACK.
func (h *receivedPacketTracker) isMissing(p protocol.PacketNumber) bool {
	if h.lastAck == nil || p < h.ignoreBelow {
		return false
	}
	return p < h.lastAck.LargestAcked() && !h.lastAck.AcksPacket(p)
}

func (h *receivedPacketTracker) hasNewMissingPackets() bool {
	if h.lastAck == nil {
		return false
	}
	highestRange := h.packetHistory.GetHighestAckRange()
	return highestRange.Smallest > h.lastAck.LargestAcked()+1 && highestRange.Len() == 1
}

// maybeQueueACK queues an ACK, if necessary.
func (h *receivedPacketTracker) maybeQueueACK(pn protocol.PacketNumber, rcvTime time.Time, wasMissing bool) {
	// always acknowledge the first packet
	if h.lastAck == nil {
		if !h.ackQueued {
			h.logger.Debugf("\tQueueing ACK because the first packet should be acknowledged.")
		}
		h.ackQueued = true
		return
	}

	if h.ackQueued {
		return
	}

	h.ackElicitingPacketsReceivedSinceLastAck++

	// Send an ACK if this packet was reported missing in an ACK sent before.
	// Ack decimation with reordering relies on the timer to send an ACK, but if
	// missing packets we reported in the previous ack, send an ACK immediately.
	if wasMissing {
		if h.logger.Debug() {
			h.logger.Debugf("\tQueueing ACK because packet %d was missing before.", pn)
		}
		h.ackQueued = true
	}

	// send an ACK every 2 ack-eliciting packets
	if h.ackElicitingPacketsReceivedSinceLastAck >= packetsBeforeAck {
		if h.logger.Debug() {
			h.logger.Debugf("\tQueueing ACK because packet %d packets were received after the last ACK (using initial threshold: %d).", h.ackElicitingPacketsReceivedSinceLastAck, packetsBeforeAck)
		}
		h.ackQueued = true
	} else if h.ackAlarm.IsZero() {
		if h.logger.Debug() {
			h.logger.Debugf("\tSetting ACK timer to max ack delay: %s", h.maxAckDelay)
		}
		h.ackAlarm = rcvTime.Add(h.maxAckDelay)
	}

	// Queue an ACK if there are new missing packets to report.
	if h.hasNewMissingPackets() {
		h.logger.Debugf("\tQueuing ACK because there's a new missing packet to report.")
		h.ackQueued = true
	}

	if h.ackQueued {
		// cancel the ack alarm
		h.ackAlarm = time.Time{}
	}
}

func (h *receivedPacketTracker) GetAckFrame(onlyIfQueued bool) *wire.AckFrame {
	if !h.hasNewAck {
		return nil
	}
	now := time.Now()
	if onlyIfQueued {
		if !h.ackQueued && (h.ackAlarm.IsZero() || h.ackAlarm.After(now)) {
			return nil
		}
		if h.logger.Debug() && !h.ackQueued && !h.ackAlarm.IsZero() {
			h.logger.Debugf("Sending ACK because the ACK timer expired.")
		}
	}

	// This function always returns the same ACK frame struct, filled with the most recent values.
	ack := h.lastAck
	if ack == nil {
		ack = &wire.AckFrame{}
	}
	ack.Reset()
	ack.DelayTime = utils.Max(0, now.Sub(h.largestObservedRcvdTime))
	ack.ECT0 = h.ect0
	ack.ECT1 = h.ect1
	ack.ECNCE = h.ecnce
	ack.AckRanges = h.packetHistory.AppendAckRanges(ack.AckRanges)

	h.lastAck = ack
	h.ackAlarm = time.Time{}
	h.ackQueued = false
	h.hasNewAck = false
	h.ackElicitingPacketsReceivedSinceLastAck = 0
	return ack
}

func (h *receivedPacketTracker) GetAlarmTimeout() time.Time { return h.ackAlarm }

func (h *receivedPacketTracker) IsPotentiallyDuplicate(pn protocol.PacketNumber) bool {
	return h.packetHistory.IsPotentiallyDuplicate(pn)
}
