package ackhandler

import (
	"errors"
	"fmt"
	"time"

	"github.com/quic-go/quic-go/internal/congestion"
	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/qerr"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/internal/wire"
	"github.com/quic-go/quic-go/logging"
)

const (
	// Maximum reordering in time space before time based loss detection considers a packet lost.
	// Specified as an RTT multiplier.
	timeThreshold = 9.0 / 8
	// Maximum reordering in packets before packet threshold loss detection considers a packet lost.
	packetThreshold = 3
	// Before validating the client's address, the server won't send more than 3x bytes than it received.
	amplificationFactor = 3
	// We use Retry packets to derive an RTT estimate. Make sure we don't set the RTT to a super low value yet.
	minRTTAfterRetry = 5 * time.Millisecond
	// The PTO duration uses exponential backoff, but is truncated to a maximum value, as allowed by RFC 8961, section 4.4.
	maxPTODuration = 60 * time.Second
)

type packetNumberSpace struct {
	history *sentPacketHistory
	pns     packetNumberGenerator

	lossTime                   time.Time
	lastAckElicitingPacketTime time.Time

	largestAcked protocol.PacketNumber
	largestSent  protocol.PacketNumber
}

func newPacketNumberSpace(initialPN protocol.PacketNumber, skipPNs bool) *packetNumberSpace {
	var pns packetNumberGenerator
	if skipPNs {
		pns = newSkippingPacketNumberGenerator(initialPN, protocol.SkipPacketInitialPeriod, protocol.SkipPacketMaxPeriod)
	} else {
		pns = newSequentialPacketNumberGenerator(initialPN)
	}
	return &packetNumberSpace{
		history:      newSentPacketHistory(),
		pns:          pns,
		largestSent:  protocol.InvalidPacketNumber,
		largestAcked: protocol.InvalidPacketNumber,
	}
}

type sentPacketHandler struct {
	initialPackets   *packetNumberSpace
	handshakePackets *packetNumberSpace
	appDataPackets   *packetNumberSpace

	// Do we know that the peer completed address validation yet?
	// Always true for the server.
	peerCompletedAddressValidation bool
	bytesReceived                  protocol.ByteCount
	bytesSent                      protocol.ByteCount
	// Have we validated the peer's address yet?
	// Always true for the client.
	peerAddressValidated bool

	handshakeConfirmed bool

	// lowestNotConfirmedAcked is the lowest packet number that we sent an ACK for, but haven't received confirmation, that this ACK actually arrived
	// example: we send an ACK for packets 90-100 with packet number 20
	// once we receive an ACK from the peer for packet 20, the lowestNotConfirmedAcked is 101
	// Only applies to the application-data packet number space.
	lowestNotConfirmedAcked protocol.PacketNumber

	ackedPackets []*packet // to avoid allocations in detectAndRemoveAckedPackets

	bytesInFlight protocol.ByteCount

	congestion congestion.SendAlgorithmWithDebugInfos
	rttStats   *utils.RTTStats

	// The number of times a PTO has been sent without receiving an ack.
	ptoCount uint32
	ptoMode  SendMode
	// The number of PTO probe packets that should be sent.
	// Only applies to the application-data packet number space.
	numProbesToSend int

	// The alarm timeout
	alarm time.Time

	enableECN  bool
	ecnTracker ecnHandler

	perspective protocol.Perspective

	tracer *logging.ConnectionTracer
	logger utils.Logger
}

var (
	_ SentPacketHandler = &sentPacketHandler{}
	_ sentPacketTracker = &sentPacketHandler{}
)

// clientAddressValidated indicates whether the address was validated beforehand by an address validation token.
// If the address was validated, the amplification limit doesn't apply. It has no effect for a client.
func newSentPacketHandler(
	initialPN protocol.PacketNumber,
	initialMaxDatagramSize protocol.ByteCount,
	rttStats *utils.RTTStats,
	clientAddressValidated bool,
	enableECN bool,
	pers protocol.Perspective,
	tracer *logging.ConnectionTracer,
	logger utils.Logger,
) *sentPacketHandler {
	congestion := congestion.NewCubicSender(
		congestion.DefaultClock{},
		rttStats,
		initialMaxDatagramSize,
		true, // use Reno
		tracer,
	)

	h := &sentPacketHandler{
		peerCompletedAddressValidation: pers == protocol.PerspectiveServer,
		peerAddressValidated:           pers == protocol.PerspectiveClient || clientAddressValidated,
		initialPackets:                 newPacketNumberSpace(initialPN, false),
		handshakePackets:               newPacketNumberSpace(0, false),
		appDataPackets:                 newPacketNumberSpace(0, true),
		rttStats:                       rttStats,
		congestion:                     congestion,
		perspective:                    pers,
		tracer:                         tracer,
		logger:                         logger,
	}
	if enableECN {
		h.enableECN = true
		h.ecnTracker = newECNTracker(logger, tracer)
	}
	return h
}

func (h *sentPacketHandler) removeFromBytesInFlight(p *packet) {
	if p.includedInBytesInFlight {
		if p.Length > h.bytesInFlight {
			panic("negative bytes_in_flight")
		}
		h.bytesInFlight -= p.Length
		p.includedInBytesInFlight = false
	}
}

func (h *sentPacketHandler) DropPackets(encLevel protocol.EncryptionLevel) {
	// The server won't await address validation after the handshake is confirmed.
	// This applies even if we didn't receive an ACK for a Handshake packet.
	if h.perspective == protocol.PerspectiveClient && encLevel == protocol.EncryptionHandshake {
		h.peerCompletedAddressValidation = true
	}
	// remove outstanding packets from bytes_in_flight
	if encLevel == protocol.EncryptionInitial || encLevel == protocol.EncryptionHandshake {
		pnSpace := h.getPacketNumberSpace(encLevel)
		// We might already have dropped this packet number space.
		if pnSpace == nil {
			return
		}
		pnSpace.history.Iterate(func(p *packet) (bool, error) {
			h.removeFromBytesInFlight(p)
			return true, nil
		})
	}
	// drop the packet history
	//nolint:exhaustive // Not every packet number space can be dropped.
	switch encLevel {
	case protocol.EncryptionInitial:
		h.initialPackets = nil
	case protocol.EncryptionHandshake:
		h.handshakePackets = nil
	case protocol.Encryption0RTT:
		// This function is only called when 0-RTT is rejected,
		// and not when the client drops 0-RTT keys when the handshake completes.
		// When 0-RTT is rejected, all application data sent so far becomes invalid.
		// Delete the packets from the history and remove them from bytes_in_flight.
		h.appDataPackets.history.Iterate(func(p *packet) (bool, error) {
			if p.EncryptionLevel != protocol.Encryption0RTT && !p.skippedPacket {
				return false, nil
			}
			h.removeFromBytesInFlight(p)
			h.appDataPackets.history.Remove(p.PacketNumber)
			return true, nil
		})
	default:
		panic(fmt.Sprintf("Cannot drop keys for encryption level %s", encLevel))
	}
	if h.tracer != nil && h.tracer.UpdatedPTOCount != nil && h.ptoCount != 0 {
		h.tracer.UpdatedPTOCount(0)
	}
	h.ptoCount = 0
	h.numProbesToSend = 0
	h.ptoMode = SendNone
	h.setLossDetectionTimer()
}

func (h *sentPacketHandler) ReceivedBytes(n protocol.ByteCount) {
	wasAmplificationLimit := h.isAmplificationLimited()
	h.bytesReceived += n
	if wasAmplificationLimit && !h.isAmplificationLimited() {
		h.setLossDetectionTimer()
	}
}

func (h *sentPacketHandler) ReceivedPacket(l protocol.EncryptionLevel) {
	if h.perspective == protocol.PerspectiveServer && l == protocol.EncryptionHandshake && !h.peerAddressValidated {
		h.peerAddressValidated = true
		h.setLossDetectionTimer()
	}
}

func (h *sentPacketHandler) packetsInFlight() int {
	packetsInFlight := h.appDataPackets.history.Len()
	if h.handshakePackets != nil {
		packetsInFlight += h.handshakePackets.history.Len()
	}
	if h.initialPackets != nil {
		packetsInFlight += h.initialPackets.history.Len()
	}
	return packetsInFlight
}

func (h *sentPacketHandler) SentPacket(
	t time.Time,
	pn, largestAcked protocol.PacketNumber,
	streamFrames []StreamFrame,
	frames []Frame,
	encLevel protocol.EncryptionLevel,
	ecn protocol.ECN,
	size protocol.ByteCount,
	isPathMTUProbePacket bool,
) {
	h.bytesSent += size

	pnSpace := h.getPacketNumberSpace(encLevel)
	if h.logger.Debug() && pnSpace.history.HasOutstandingPackets() {
		for p := utils.Max(0, pnSpace.largestSent+1); p < pn; p++ {
			h.logger.Debugf("Skipping packet number %d", p)
		}
	}

	pnSpace.largestSent = pn
	isAckEliciting := len(streamFrames) > 0 || len(frames) > 0

	if isAckEliciting {
		pnSpace.lastAckElicitingPacketTime = t
		h.bytesInFlight += size
		if h.numProbesToSend > 0 {
			h.numProbesToSend--
		}
	}
	h.congestion.OnPacketSent(t, h.bytesInFlight, pn, size, isAckEliciting)

	if encLevel == protocol.Encryption1RTT && h.ecnTracker != nil {
		h.ecnTracker.SentPacket(pn, ecn)
	}

	if !isAckEliciting {
		pnSpace.history.SentNonAckElicitingPacket(pn)
		if !h.peerCompletedAddressValidation {
			h.setLossDetectionTimer()
		}
		return
	}

	p := getPacket()
	p.SendTime = t
	p.PacketNumber = pn
	p.EncryptionLevel = encLevel
	p.Length = size
	p.LargestAcked = largestAcked
	p.StreamFrames = streamFrames
	p.Frames = frames
	p.IsPathMTUProbePacket = isPathMTUProbePacket
	p.includedInBytesInFlight = true

	pnSpace.history.SentAckElicitingPacket(p)
	if h.tracer != nil && h.tracer.UpdatedMetrics != nil {
		h.tracer.UpdatedMetrics(h.rttStats, h.congestion.GetCongestionWindow(), h.bytesInFlight, h.packetsInFlight())
	}
	h.setLossDetectionTimer()
}

func (h *sentPacketHandler) getPacketNumberSpace(encLevel protocol.EncryptionLevel) *packetNumberSpace {
	switch encLevel {
	case protocol.EncryptionInitial:
		return h.initialPackets
	case protocol.EncryptionHandshake:
		return h.handshakePackets
	case protocol.Encryption0RTT, protocol.Encryption1RTT:
		return h.appDataPackets
	default:
		panic("invalid packet number space")
	}
}

func (h *sentPacketHandler) ReceivedAck(ack *wire.AckFrame, encLevel protocol.EncryptionLevel, rcvTime time.Time) (bool /* contained 1-RTT packet */, error) {
	pnSpace := h.getPacketNumberSpace(encLevel)

	largestAcked := ack.LargestAcked()
	if largestAcked > pnSpace.largestSent {
		return false, &qerr.TransportError{
			ErrorCode:    qerr.ProtocolViolation,
			ErrorMessage: "received ACK for an unsent packet",
		}
	}

	// Servers complete address validation when a protected packet is received.
	if h.perspective == protocol.PerspectiveClient && !h.peerCompletedAddressValidation &&
		(encLevel == protocol.EncryptionHandshake || encLevel == protocol.Encryption1RTT) {
		h.peerCompletedAddressValidation = true
		h.logger.Debugf("Peer doesn't await address validation any longer.")
		// Make sure that the timer is reset, even if this ACK doesn't acknowledge any (ack-eliciting) packets.
		h.setLossDetectionTimer()
	}

	priorInFlight := h.bytesInFlight
	ackedPackets, err := h.detectAndRemoveAckedPackets(ack, encLevel)
	if err != nil || len(ackedPackets) == 0 {
		return false, err
	}
	// update the RTT, if the largest acked is newly acknowledged
	if len(ackedPackets) > 0 {
		if p := ackedPackets[len(ackedPackets)-1]; p.PacketNumber == ack.LargestAcked() {
			// don't use the ack delay for Initial and Handshake packets
			var ackDelay time.Duration
			if encLevel == protocol.Encryption1RTT {
				ackDelay = utils.Min(ack.DelayTime, h.rttStats.MaxAckDelay())
			}
			h.rttStats.UpdateRTT(rcvTime.Sub(p.SendTime), ackDelay, rcvTime)
			if h.logger.Debug() {
				h.logger.Debugf("\tupdated RTT: %s (σ: %s)", h.rttStats.SmoothedRTT(), h.rttStats.MeanDeviation())
			}
			h.congestion.MaybeExitSlowStart()
		}
	}

	// Only inform the ECN tracker about new 1-RTT ACKs if the ACK increases the largest acked.
	if encLevel == protocol.Encryption1RTT && h.ecnTracker != nil && largestAcked > pnSpace.largestAcked {
		congested := h.ecnTracker.HandleNewlyAcked(ackedPackets, int64(ack.ECT0), int64(ack.ECT1), int64(ack.ECNCE))
		if congested {
			h.congestion.OnCongestionEvent(largestAcked, 0, priorInFlight)
		}
	}

	pnSpace.largestAcked = utils.Max(pnSpace.largestAcked, largestAcked)

	if err := h.detectLostPackets(rcvTime, encLevel); err != nil {
		return false, err
	}
	var acked1RTTPacket bool
	for _, p := range ackedPackets {
		if p.includedInBytesInFlight && !p.declaredLost {
			h.congestion.OnPacketAcked(p.PacketNumber, p.Length, priorInFlight, rcvTime)
		}
		if p.EncryptionLevel == protocol.Encryption1RTT {
			acked1RTTPacket = true
		}
		h.removeFromBytesInFlight(p)
		putPacket(p)
	}
	// After this point, we must not use ackedPackets any longer!
	// We've already returned the buffers.
	ackedPackets = nil //nolint:ineffassign // This is just to be on the safe side.

	// Reset the pto_count unless the client is unsure if the server has validated the client's address.
	if h.peerCompletedAddressValidation {
		if h.tracer != nil && h.tracer.UpdatedPTOCount != nil && h.ptoCount != 0 {
			h.tracer.UpdatedPTOCount(0)
		}
		h.ptoCount = 0
	}
	h.numProbesToSend = 0

	if h.tracer != nil && h.tracer.UpdatedMetrics != nil {
		h.tracer.UpdatedMetrics(h.rttStats, h.congestion.GetCongestionWindow(), h.bytesInFlight, h.packetsInFlight())
	}

	h.setLossDetectionTimer()
	return acked1RTTPacket, nil
}

func (h *sentPacketHandler) GetLowestPacketNotConfirmedAcked() protocol.PacketNumber {
	return h.lowestNotConfirmedAcked
}

// Packets are returned in ascending packet number order.
func (h *sentPacketHandler) detectAndRemoveAckedPackets(ack *wire.AckFrame, encLevel protocol.EncryptionLevel) ([]*packet, error) {
	pnSpace := h.getPacketNumberSpace(encLevel)
	h.ackedPackets = h.ackedPackets[:0]
	ackRangeIndex := 0
	lowestAcked := ack.LowestAcked()
	largestAcked := ack.LargestAcked()
	err := pnSpace.history.Iterate(func(p *packet) (bool, error) {
		// Ignore packets below the lowest acked
		if p.PacketNumber < lowestAcked {
			return true, nil
		}
		// Break after largest acked is reached
		if p.PacketNumber > largestAcked {
			return false, nil
		}

		if ack.HasMissingRanges() {
			ackRange := ack.AckRanges[len(ack.AckRanges)-1-ackRangeIndex]

			for p.PacketNumber > ackRange.Largest && ackRangeIndex < len(ack.AckRanges)-1 {
				ackRangeIndex++
				ackRange = ack.AckRanges[len(ack.AckRanges)-1-ackRangeIndex]
			}

			if p.PacketNumber < ackRange.Smallest { // packet not contained in ACK range
				return true, nil
			}
			if p.PacketNumber > ackRange.Largest {
				return false, fmt.Errorf("BUG: ackhandler would have acked wrong packet %d, while evaluating range %d -> %d", p.PacketNumber, ackRange.Smallest, ackRange.Largest)
			}
		}
		if p.skippedPacket {
			return false, &qerr.TransportError{
				ErrorCode:    qerr.ProtocolViolation,
				ErrorMessage: fmt.Sprintf("received an ACK for skipped packet number: %d (%s)", p.PacketNumber, encLevel),
			}
		}
		h.ackedPackets = append(h.ackedPackets, p)
		return true, nil
	})
	if h.logger.Debug() && len(h.ackedPackets) > 0 {
		pns := make([]protocol.PacketNumber, len(h.ackedPackets))
		for i, p := range h.ackedPackets {
			pns[i] = p.PacketNumber
		}
		h.logger.Debugf("\tnewly acked packets (%d): %d", len(pns), pns)
	}

	for _, p := range h.ackedPackets {
		if p.LargestAcked != protocol.InvalidPacketNumber && encLevel == protocol.Encryption1RTT {
			h.lowestNotConfirmedAcked = utils.Max(h.lowestNotConfirmedAcked, p.LargestAcked+1)
		}

		for _, f := range p.Frames {
			if f.Handler != nil {
				f.Handler.OnAcked(f.Frame)
			}
		}
		for _, f := range p.StreamFrames {
			if f.Handler != nil {
				f.Handler.OnAcked(f.Frame)
			}
		}
		if err := pnSpace.history.Remove(p.PacketNumber); err != nil {
			return nil, err
		}
		if h.tracer != nil && h.tracer.AcknowledgedPacket != nil {
			h.tracer.AcknowledgedPacket(encLevel, p.PacketNumber)
		}
	}

	return h.ackedPackets, err
}

func (h *sentPacketHandler) getLossTimeAndSpace() (time.Time, protocol.EncryptionLevel) {
	var encLevel protocol.EncryptionLevel
	var lossTime time.Time

	if h.initialPackets != nil {
		lossTime = h.initialPackets.lossTime
		encLevel = protocol.EncryptionInitial
	}
	if h.handshakePackets != nil && (lossTime.IsZero() || (!h.handshakePackets.lossTime.IsZero() && h.handshakePackets.lossTime.Before(lossTime))) {
		lossTime = h.handshakePackets.lossTime
		encLevel = protocol.EncryptionHandshake
	}
	if lossTime.IsZero() || (!h.appDataPackets.lossTime.IsZero() && h.appDataPackets.lossTime.Before(lossTime)) {
		lossTime = h.appDataPackets.lossTime
		encLevel = protocol.Encryption1RTT
	}
	return lossTime, encLevel
}

func (h *sentPacketHandler) getScaledPTO(includeMaxAckDelay bool) time.Duration {
	pto := h.rttStats.PTO(includeMaxAckDelay) << h.ptoCount
	if pto > maxPTODuration || pto <= 0 {
		return maxPTODuration
	}
	return pto
}

// same logic as getLossTimeAndSpace, but for lastAckElicitingPacketTime instead of lossTime
func (h *sentPacketHandler) getPTOTimeAndSpace() (pto time.Time, encLevel protocol.EncryptionLevel, ok bool) {
	// We only send application data probe packets once the handshake is confirmed,
	// because before that, we don't have the keys to decrypt ACKs sent in 1-RTT packets.
	if !h.handshakeConfirmed && !h.hasOutstandingCryptoPackets() {
		if h.peerCompletedAddressValidation {
			return
		}
		t := time.Now().Add(h.getScaledPTO(false))
		if h.initialPackets != nil {
			return t, protocol.EncryptionInitial, true
		}
		return t, protocol.EncryptionHandshake, true
	}

	if h.initialPackets != nil {
		encLevel = protocol.EncryptionInitial
		if t := h.initialPackets.lastAckElicitingPacketTime; !t.IsZero() {
			pto = t.Add(h.getScaledPTO(false))
		}
	}
	if h.handshakePackets != nil && !h.handshakePackets.lastAckElicitingPacketTime.IsZero() {
		t := h.handshakePackets.lastAckElicitingPacketTime.Add(h.getScaledPTO(false))
		if pto.IsZero() || (!t.IsZero() && t.Before(pto)) {
			pto = t
			encLevel = protocol.EncryptionHandshake
		}
	}
	if h.handshakeConfirmed && !h.appDataPackets.lastAckElicitingPacketTime.IsZero() {
		t := h.appDataPackets.lastAckElicitingPacketTime.Add(h.getScaledPTO(true))
		if pto.IsZero() || (!t.IsZero() && t.Before(pto)) {
			pto = t
			encLevel = protocol.Encryption1RTT
		}
	}
	return pto, encLevel, true
}

func (h *sentPacketHandler) hasOutstandingCryptoPackets() bool {
	if h.initialPackets != nil && h.initialPackets.history.HasOutstandingPackets() {
		return true
	}
	if h.handshakePackets != nil && h.handshakePackets.history.HasOutstandingPackets() {
		return true
	}
	return false
}

func (h *sentPacketHandler) hasOutstandingPackets() bool {
	return h.appDataPackets.history.HasOutstandingPackets() || h.hasOutstandingCryptoPackets()
}

func (h *sentPacketHandler) setLossDetectionTimer() {
	oldAlarm := h.alarm // only needed in case tracing is enabled
	lossTime, encLevel := h.getLossTimeAndSpace()
	if !lossTime.IsZero() {
		// Early retransmit timer or time loss detection.
		h.alarm = lossTime
		if h.tracer != nil && h.tracer.SetLossTimer != nil && h.alarm != oldAlarm {
			h.tracer.SetLossTimer(logging.TimerTypeACK, encLevel, h.alarm)
		}
		return
	}

	// Cancel the alarm if amplification limited.
	if h.isAmplificationLimited() {
		h.alarm = time.Time{}
		if !oldAlarm.IsZero() {
			h.logger.Debugf("Canceling loss detection timer. Amplification limited.")
			if h.tracer != nil && h.tracer.LossTimerCanceled != nil {
				h.tracer.LossTimerCanceled()
			}
		}
		return
	}

	// Cancel the alarm if no packets are outstanding
	if !h.hasOutstandingPackets() && h.peerCompletedAddressValidation {
		h.alarm = time.Time{}
		if !oldAlarm.IsZero() {
			h.logger.Debugf("Canceling loss detection timer. No packets in flight.")
			if h.tracer != nil && h.tracer.LossTimerCanceled != nil {
				h.tracer.LossTimerCanceled()
			}
		}
		return
	}

	// PTO alarm
	ptoTime, encLevel, ok := h.getPTOTimeAndSpace()
	if !ok {
		if !oldAlarm.IsZero() {
			h.alarm = time.Time{}
			h.logger.Debugf("Canceling loss detection timer. No PTO needed..")
			if h.tracer != nil && h.tracer.LossTimerCanceled != nil {
				h.tracer.LossTimerCanceled()
			}
		}
		return
	}
	h.alarm = ptoTime
	if h.tracer != nil && h.tracer.SetLossTimer != nil && h.alarm != oldAlarm {
		h.tracer.SetLossTimer(logging.TimerTypePTO, encLevel, h.alarm)
	}
}

func (h *sentPacketHandler) detectLostPackets(now time.Time, encLevel protocol.EncryptionLevel) error {
	pnSpace := h.getPacketNumberSpace(encLevel)
	pnSpace.lossTime = time.Time{}

	maxRTT := float64(utils.Max(h.rttStats.LatestRTT(), h.rttStats.SmoothedRTT()))
	lossDelay := time.Duration(timeThreshold * maxRTT)

	// Minimum time of granularity before packets are deemed lost.
	lossDelay = utils.Max(lossDelay, protocol.TimerGranularity)

	// Packets sent before this time are deemed lost.
	lostSendTime := now.Add(-lossDelay)

	priorInFlight := h.bytesInFlight
	return pnSpace.history.Iterate(func(p *packet) (bool, error) {
		if p.PacketNumber > pnSpace.largestAcked {
			return false, nil
		}

		var packetLost bool
		if p.SendTime.Before(lostSendTime) {
			packetLost = true
			if !p.skippedPacket {
				if h.logger.Debug() {
					h.logger.Debugf("\tlost packet %d (time threshold)", p.PacketNumber)
				}
				if h.tracer != nil && h.tracer.LostPacket != nil {
					h.tracer.LostPacket(p.EncryptionLevel, p.PacketNumber, logging.PacketLossTimeThreshold)
				}
			}
		} else if pnSpace.largestAcked >= p.PacketNumber+packetThreshold {
			packetLost = true
			if !p.skippedPacket {
				if h.logger.Debug() {
					h.logger.Debugf("\tlost packet %d (reordering threshold)", p.PacketNumber)
				}
				if h.tracer != nil && h.tracer.LostPacket != nil {
					h.tracer.LostPacket(p.EncryptionLevel, p.PacketNumber, logging.PacketLossReorderingThreshold)
				}
			}
		} else if pnSpace.lossTime.IsZero() {
			// Note: This conditional is only entered once per call
			lossTime := p.SendTime.Add(lossDelay)
			if h.logger.Debug() {
				h.logger.Debugf("\tsetting loss timer for packet %d (%s) to %s (in %s)", p.PacketNumber, encLevel, lossDelay, lossTime)
			}
			pnSpace.lossTime = lossTime
		}
		if packetLost {
			pnSpace.history.DeclareLost(p.PacketNumber)
			if !p.skippedPacket {
				// the bytes in flight need to be reduced no matter if the frames in this packet will be retransmitted
				h.removeFromBytesInFlight(p)
				h.queueFramesForRetransmission(p)
				if !p.IsPathMTUProbePacket {
					h.congestion.OnCongestionEvent(p.PacketNumber, p.Length, priorInFlight)
				}
				if encLevel == protocol.Encryption1RTT && h.ecnTracker != nil {
					h.ecnTracker.LostPacket(p.PacketNumber)
				}
			}
		}
		return true, nil
	})
}

func (h *sentPacketHandler) OnLossDetectionTimeout() error {
	defer h.setLossDetectionTimer()
	earliestLossTime, encLevel := h.getLossTimeAndSpace()
	if !earliestLossTime.IsZero() {
		if h.logger.Debug() {
			h.logger.Debugf("Loss detection alarm fired in loss timer mode. Loss time: %s", earliestLossTime)
		}
		if h.tracer != nil && h.tracer.LossTimerExpired != nil {
			h.tracer.LossTimerExpired(logging.TimerTypeACK, encLevel)
		}
		// Early retransmit or time loss detection
		return h.detectLostPackets(time.Now(), encLevel)
	}

	// PTO
	// When all outstanding are acknowledged, the alarm is canceled in
	// setLossDetectionTimer. This doesn't reset the timer in the session though.
	// When OnAlarm is called, we therefore need to make sure that there are
	// actually packets outstanding.
	if h.bytesInFlight == 0 && !h.peerCompletedAddressValidation {
		h.ptoCount++
		h.numProbesToSend++
		if h.initialPackets != nil {
			h.ptoMode = SendPTOInitial
		} else if h.handshakePackets != nil {
			h.ptoMode = SendPTOHandshake
		} else {
			return errors.New("sentPacketHandler BUG: PTO fired, but bytes_in_flight is 0 and Initial and Handshake already dropped")
		}
		return nil
	}

	_, encLevel, ok := h.getPTOTimeAndSpace()
	if !ok {
		return nil
	}
	if ps := h.getPacketNumberSpace(encLevel); !ps.history.HasOutstandingPackets() && !h.peerCompletedAddressValidation {
		return nil
	}
	h.ptoCount++
	if h.logger.Debug() {
		h.logger.Debugf("Loss detection alarm for %s fired in PTO mode. PTO count: %d", encLevel, h.ptoCount)
	}
	if h.tracer != nil {
		if h.tracer.LossTimerExpired != nil {
			h.tracer.LossTimerExpired(logging.TimerTypePTO, encLevel)
		}
		if h.tracer.UpdatedPTOCount != nil {
			h.tracer.UpdatedPTOCount(h.ptoCount)
		}
	}
	h.numProbesToSend += 2
	//nolint:exhaustive // We never arm a PTO timer for 0-RTT packets.
	switch encLevel {
	case protocol.EncryptionInitial:
		h.ptoMode = SendPTOInitial
	case protocol.EncryptionHandshake:
		h.ptoMode = SendPTOHandshake
	case protocol.Encryption1RTT:
		// skip a packet number in order to elicit an immediate ACK
		pn := h.PopPacketNumber(protocol.Encryption1RTT)
		h.getPacketNumberSpace(protocol.Encryption1RTT).history.SkippedPacket(pn)
		h.ptoMode = SendPTOAppData
	default:
		return fmt.Errorf("PTO timer in unexpected encryption level: %s", encLevel)
	}
	return nil
}

func (h *sentPacketHandler) GetLossDetectionTimeout() time.Time {
	return h.alarm
}

func (h *sentPacketHandler) ECNMode(isShortHeaderPacket bool) protocol.ECN {
	if !h.enableECN {
		return protocol.ECNUnsupported
	}
	if !isShortHeaderPacket {
		return protocol.ECNNon
	}
	return h.ecnTracker.Mode()
}

func (h *sentPacketHandler) PeekPacketNumber(encLevel protocol.EncryptionLevel) (protocol.PacketNumber, protocol.PacketNumberLen) {
	pnSpace := h.getPacketNumberSpace(encLevel)
	pn := pnSpace.pns.Peek()
	// See section 17.1 of RFC 9000.
	return pn, protocol.GetPacketNumberLengthForHeader(pn, pnSpace.largestAcked)
}

func (h *sentPacketHandler) PopPacketNumber(encLevel protocol.EncryptionLevel) protocol.PacketNumber {
	pnSpace := h.getPacketNumberSpace(encLevel)
	skipped, pn := pnSpace.pns.Pop()
	if skipped {
		skippedPN := pn - 1
		pnSpace.history.SkippedPacket(skippedPN)
		if h.logger.Debug() {
			h.logger.Debugf("Skipping packet number %d", skippedPN)
		}
	}
	return pn
}

func (h *sentPacketHandler) SendMode(now time.Time) SendMode {
	numTrackedPackets := h.appDataPackets.history.Len()
	if h.initialPackets != nil {
		numTrackedPackets += h.initialPackets.history.Len()
	}
	if h.handshakePackets != nil {
		numTrackedPackets += h.handshakePackets.history.Len()
	}

	if h.isAmplificationLimited() {
		h.logger.Debugf("Amplification window limited. Received %d bytes, already sent out %d bytes", h.bytesReceived, h.bytesSent)
		return SendNone
	}
	// Don't send any packets if we're keeping track of the maximum number of packets.
	// Note that since MaxOutstandingSentPackets is smaller than MaxTrackedSentPackets,
	// we will stop sending out new data when reaching MaxOutstandingSentPackets,
	// but still allow sending of retransmissions and ACKs.
	if numTrackedPackets >= protocol.MaxTrackedSentPackets {
		if h.logger.Debug() {
			h.logger.Debugf("Limited by the number of tracked packets: tracking %d packets, maximum %d", numTrackedPackets, protocol.MaxTrackedSentPackets)
		}
		return SendNone
	}
	if h.numProbesToSend > 0 {
		return h.ptoMode
	}
	// Only send ACKs if we're congestion limited.
	if !h.congestion.CanSend(h.bytesInFlight) {
		if h.logger.Debug() {
			h.logger.Debugf("Congestion limited: bytes in flight %d, window %d", h.bytesInFlight, h.congestion.GetCongestionWindow())
		}
		return SendAck
	}
	if numTrackedPackets >= protocol.MaxOutstandingSentPackets {
		if h.logger.Debug() {
			h.logger.Debugf("Max outstanding limited: tracking %d packets, maximum: %d", numTrackedPackets, protocol.MaxOutstandingSentPackets)
		}
		return SendAck
	}
	if !h.congestion.HasPacingBudget(now) {
		return SendPacingLimited
	}
	return SendAny
}

func (h *sentPacketHandler) TimeUntilSend() time.Time {
	return h.congestion.TimeUntilSend(h.bytesInFlight)
}

func (h *sentPacketHandler) SetMaxDatagramSize(s protocol.ByteCount) {
	h.congestion.SetMaxDatagramSize(s)
}

func (h *sentPacketHandler) isAmplificationLimited() bool {
	if h.peerAddressValidated {
		return false
	}
	return h.bytesSent >= amplificationFactor*h.bytesReceived
}

func (h *sentPacketHandler) QueueProbePacket(encLevel protocol.EncryptionLevel) bool {
	pnSpace := h.getPacketNumberSpace(encLevel)
	p := pnSpace.history.FirstOutstanding()
	if p == nil {
		return false
	}
	h.queueFramesForRetransmission(p)
	// TODO: don't declare the packet lost here.
	// Keep track of acknowledged frames instead.
	h.removeFromBytesInFlight(p)
	pnSpace.history.DeclareLost(p.PacketNumber)
	return true
}

func (h *sentPacketHandler) queueFramesForRetransmission(p *packet) {
	if len(p.Frames) == 0 && len(p.StreamFrames) == 0 {
		panic("no frames")
	}
	for _, f := range p.Frames {
		if f.Handler != nil {
			f.Handler.OnLost(f.Frame)
		}
	}
	for _, f := range p.StreamFrames {
		if f.Handler != nil {
			f.Handler.OnLost(f.Frame)
		}
	}
	p.StreamFrames = nil
	p.Frames = nil
}

func (h *sentPacketHandler) ResetForRetry(now time.Time) error {
	h.bytesInFlight = 0
	var firstPacketSendTime time.Time
	h.initialPackets.history.Iterate(func(p *packet) (bool, error) {
		if firstPacketSendTime.IsZero() {
			firstPacketSendTime = p.SendTime
		}
		if p.declaredLost || p.skippedPacket {
			return true, nil
		}
		h.queueFramesForRetransmission(p)
		return true, nil
	})
	// All application data packets sent at this point are 0-RTT packets.
	// In the case of a Retry, we can assume that the server dropped all of them.
	h.appDataPackets.history.Iterate(func(p *packet) (bool, error) {
		if !p.declaredLost && !p.skippedPacket {
			h.queueFramesForRetransmission(p)
		}
		return true, nil
	})

	// Only use the Retry to estimate the RTT if we didn't send any retransmission for the Initial.
	// Otherwise, we don't know which Initial the Retry was sent in response to.
	if h.ptoCount == 0 {
		// Don't set the RTT to a value lower than 5ms here.
		h.rttStats.UpdateRTT(utils.Max(minRTTAfterRetry, now.Sub(firstPacketSendTime)), 0, now)
		if h.logger.Debug() {
			h.logger.Debugf("\tupdated RTT: %s (σ: %s)", h.rttStats.SmoothedRTT(), h.rttStats.MeanDeviation())
		}
		if h.tracer != nil && h.tracer.UpdatedMetrics != nil {
			h.tracer.UpdatedMetrics(h.rttStats, h.congestion.GetCongestionWindow(), h.bytesInFlight, h.packetsInFlight())
		}
	}
	h.initialPackets = newPacketNumberSpace(h.initialPackets.pns.Peek(), false)
	h.appDataPackets = newPacketNumberSpace(h.appDataPackets.pns.Peek(), true)
	oldAlarm := h.alarm
	h.alarm = time.Time{}
	if h.tracer != nil {
		if h.tracer.UpdatedPTOCount != nil {
			h.tracer.UpdatedPTOCount(0)
		}
		if !oldAlarm.IsZero() && h.tracer.LossTimerCanceled != nil {
			h.tracer.LossTimerCanceled()
		}
	}
	h.ptoCount = 0
	return nil
}

func (h *sentPacketHandler) SetHandshakeConfirmed() {
	if h.initialPackets != nil {
		panic("didn't drop initial correctly")
	}
	if h.handshakePackets != nil {
		panic("didn't drop handshake correctly")
	}
	h.handshakeConfirmed = true
	// We don't send PTOs for application data packets before the handshake completes.
	// Make sure the timer is armed now, if necessary.
	h.setLossDetectionTimer()
}
