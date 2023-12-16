package logging

import (
	"net"
	"time"
)

// A ConnectionTracer records events.
type ConnectionTracer struct {
	StartedConnection                func(local, remote net.Addr, srcConnID, destConnID ConnectionID)
	NegotiatedVersion                func(chosen VersionNumber, clientVersions, serverVersions []VersionNumber)
	ClosedConnection                 func(error)
	SentTransportParameters          func(*TransportParameters)
	ReceivedTransportParameters      func(*TransportParameters)
	RestoredTransportParameters      func(parameters *TransportParameters) // for 0-RTT
	SentLongHeaderPacket             func(*ExtendedHeader, ByteCount, ECN, *AckFrame, []Frame)
	SentShortHeaderPacket            func(*ShortHeader, ByteCount, ECN, *AckFrame, []Frame)
	ReceivedVersionNegotiationPacket func(dest, src ArbitraryLenConnectionID, _ []VersionNumber)
	ReceivedRetry                    func(*Header)
	ReceivedLongHeaderPacket         func(*ExtendedHeader, ByteCount, ECN, []Frame)
	ReceivedShortHeaderPacket        func(*ShortHeader, ByteCount, ECN, []Frame)
	BufferedPacket                   func(PacketType, ByteCount)
	DroppedPacket                    func(PacketType, ByteCount, PacketDropReason)
	UpdatedMetrics                   func(rttStats *RTTStats, cwnd, bytesInFlight ByteCount, packetsInFlight int)
	AcknowledgedPacket               func(EncryptionLevel, PacketNumber)
	LostPacket                       func(EncryptionLevel, PacketNumber, PacketLossReason)
	UpdatedCongestionState           func(CongestionState)
	UpdatedPTOCount                  func(value uint32)
	UpdatedKeyFromTLS                func(EncryptionLevel, Perspective)
	UpdatedKey                       func(generation KeyPhase, remote bool)
	DroppedEncryptionLevel           func(EncryptionLevel)
	DroppedKey                       func(generation KeyPhase)
	SetLossTimer                     func(TimerType, EncryptionLevel, time.Time)
	LossTimerExpired                 func(TimerType, EncryptionLevel)
	LossTimerCanceled                func()
	ECNStateUpdated                  func(state ECNState, trigger ECNStateTrigger)
	// Close is called when the connection is closed.
	Close func()
	Debug func(name, msg string)
}

// NewMultiplexedConnectionTracer creates a new connection tracer that multiplexes events to multiple tracers.
func NewMultiplexedConnectionTracer(tracers ...*ConnectionTracer) *ConnectionTracer {
	if len(tracers) == 0 {
		return nil
	}
	if len(tracers) == 1 {
		return tracers[0]
	}
	return &ConnectionTracer{
		StartedConnection: func(local, remote net.Addr, srcConnID, destConnID ConnectionID) {
			for _, t := range tracers {
				if t.StartedConnection != nil {
					t.StartedConnection(local, remote, srcConnID, destConnID)
				}
			}
		},
		NegotiatedVersion: func(chosen VersionNumber, clientVersions, serverVersions []VersionNumber) {
			for _, t := range tracers {
				if t.NegotiatedVersion != nil {
					t.NegotiatedVersion(chosen, clientVersions, serverVersions)
				}
			}
		},
		ClosedConnection: func(e error) {
			for _, t := range tracers {
				if t.ClosedConnection != nil {
					t.ClosedConnection(e)
				}
			}
		},
		SentTransportParameters: func(tp *TransportParameters) {
			for _, t := range tracers {
				if t.SentTransportParameters != nil {
					t.SentTransportParameters(tp)
				}
			}
		},
		ReceivedTransportParameters: func(tp *TransportParameters) {
			for _, t := range tracers {
				if t.ReceivedTransportParameters != nil {
					t.ReceivedTransportParameters(tp)
				}
			}
		},
		RestoredTransportParameters: func(tp *TransportParameters) {
			for _, t := range tracers {
				if t.RestoredTransportParameters != nil {
					t.RestoredTransportParameters(tp)
				}
			}
		},
		SentLongHeaderPacket: func(hdr *ExtendedHeader, size ByteCount, ecn ECN, ack *AckFrame, frames []Frame) {
			for _, t := range tracers {
				if t.SentLongHeaderPacket != nil {
					t.SentLongHeaderPacket(hdr, size, ecn, ack, frames)
				}
			}
		},
		SentShortHeaderPacket: func(hdr *ShortHeader, size ByteCount, ecn ECN, ack *AckFrame, frames []Frame) {
			for _, t := range tracers {
				if t.SentShortHeaderPacket != nil {
					t.SentShortHeaderPacket(hdr, size, ecn, ack, frames)
				}
			}
		},
		ReceivedVersionNegotiationPacket: func(dest, src ArbitraryLenConnectionID, versions []VersionNumber) {
			for _, t := range tracers {
				if t.ReceivedVersionNegotiationPacket != nil {
					t.ReceivedVersionNegotiationPacket(dest, src, versions)
				}
			}
		},
		ReceivedRetry: func(hdr *Header) {
			for _, t := range tracers {
				if t.ReceivedRetry != nil {
					t.ReceivedRetry(hdr)
				}
			}
		},
		ReceivedLongHeaderPacket: func(hdr *ExtendedHeader, size ByteCount, ecn ECN, frames []Frame) {
			for _, t := range tracers {
				if t.ReceivedLongHeaderPacket != nil {
					t.ReceivedLongHeaderPacket(hdr, size, ecn, frames)
				}
			}
		},
		ReceivedShortHeaderPacket: func(hdr *ShortHeader, size ByteCount, ecn ECN, frames []Frame) {
			for _, t := range tracers {
				if t.ReceivedShortHeaderPacket != nil {
					t.ReceivedShortHeaderPacket(hdr, size, ecn, frames)
				}
			}
		},
		BufferedPacket: func(typ PacketType, size ByteCount) {
			for _, t := range tracers {
				if t.BufferedPacket != nil {
					t.BufferedPacket(typ, size)
				}
			}
		},
		DroppedPacket: func(typ PacketType, size ByteCount, reason PacketDropReason) {
			for _, t := range tracers {
				if t.DroppedPacket != nil {
					t.DroppedPacket(typ, size, reason)
				}
			}
		},
		UpdatedMetrics: func(rttStats *RTTStats, cwnd, bytesInFlight ByteCount, packetsInFlight int) {
			for _, t := range tracers {
				if t.UpdatedMetrics != nil {
					t.UpdatedMetrics(rttStats, cwnd, bytesInFlight, packetsInFlight)
				}
			}
		},
		AcknowledgedPacket: func(encLevel EncryptionLevel, pn PacketNumber) {
			for _, t := range tracers {
				if t.AcknowledgedPacket != nil {
					t.AcknowledgedPacket(encLevel, pn)
				}
			}
		},
		LostPacket: func(encLevel EncryptionLevel, pn PacketNumber, reason PacketLossReason) {
			for _, t := range tracers {
				if t.LostPacket != nil {
					t.LostPacket(encLevel, pn, reason)
				}
			}
		},
		UpdatedCongestionState: func(state CongestionState) {
			for _, t := range tracers {
				if t.UpdatedCongestionState != nil {
					t.UpdatedCongestionState(state)
				}
			}
		},
		UpdatedPTOCount: func(value uint32) {
			for _, t := range tracers {
				if t.UpdatedPTOCount != nil {
					t.UpdatedPTOCount(value)
				}
			}
		},
		UpdatedKeyFromTLS: func(encLevel EncryptionLevel, perspective Perspective) {
			for _, t := range tracers {
				if t.UpdatedKeyFromTLS != nil {
					t.UpdatedKeyFromTLS(encLevel, perspective)
				}
			}
		},
		UpdatedKey: func(generation KeyPhase, remote bool) {
			for _, t := range tracers {
				if t.UpdatedKey != nil {
					t.UpdatedKey(generation, remote)
				}
			}
		},
		DroppedEncryptionLevel: func(encLevel EncryptionLevel) {
			for _, t := range tracers {
				if t.DroppedEncryptionLevel != nil {
					t.DroppedEncryptionLevel(encLevel)
				}
			}
		},
		DroppedKey: func(generation KeyPhase) {
			for _, t := range tracers {
				if t.DroppedKey != nil {
					t.DroppedKey(generation)
				}
			}
		},
		SetLossTimer: func(typ TimerType, encLevel EncryptionLevel, exp time.Time) {
			for _, t := range tracers {
				if t.SetLossTimer != nil {
					t.SetLossTimer(typ, encLevel, exp)
				}
			}
		},
		LossTimerExpired: func(typ TimerType, encLevel EncryptionLevel) {
			for _, t := range tracers {
				if t.LossTimerExpired != nil {
					t.LossTimerExpired(typ, encLevel)
				}
			}
		},
		LossTimerCanceled: func() {
			for _, t := range tracers {
				if t.LossTimerCanceled != nil {
					t.LossTimerCanceled()
				}
			}
		},
		ECNStateUpdated: func(state ECNState, trigger ECNStateTrigger) {
			for _, t := range tracers {
				if t.ECNStateUpdated != nil {
					t.ECNStateUpdated(state, trigger)
				}
			}
		},
		Close: func() {
			for _, t := range tracers {
				if t.Close != nil {
					t.Close()
				}
			}
		},
		Debug: func(name, msg string) {
			for _, t := range tracers {
				if t.Debug != nil {
					t.Debug(name, msg)
				}
			}
		},
	}
}
