package qlog

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/logging"

	"github.com/francoispqt/gojay"
)

func milliseconds(dur time.Duration) float64 { return float64(dur.Nanoseconds()) / 1e6 }

type eventDetails interface {
	Category() category
	Name() string
	gojay.MarshalerJSONObject
}

type event struct {
	RelativeTime time.Duration
	eventDetails
}

var _ gojay.MarshalerJSONObject = event{}

func (e event) IsNil() bool { return false }
func (e event) MarshalJSONObject(enc *gojay.Encoder) {
	enc.Float64Key("time", milliseconds(e.RelativeTime))
	enc.StringKey("name", e.Category().String()+":"+e.Name())
	enc.ObjectKey("data", e.eventDetails)
}

type versions []versionNumber

func (v versions) IsNil() bool { return false }
func (v versions) MarshalJSONArray(enc *gojay.Encoder) {
	for _, e := range v {
		enc.AddString(e.String())
	}
}

type rawInfo struct {
	Length        logging.ByteCount // full packet length, including header and AEAD authentication tag
	PayloadLength logging.ByteCount // length of the packet payload, excluding AEAD tag
}

func (i rawInfo) IsNil() bool { return false }
func (i rawInfo) MarshalJSONObject(enc *gojay.Encoder) {
	enc.Uint64Key("length", uint64(i.Length))
	enc.Uint64KeyOmitEmpty("payload_length", uint64(i.PayloadLength))
}

type eventConnectionStarted struct {
	SrcAddr  *net.UDPAddr
	DestAddr *net.UDPAddr

	SrcConnectionID  protocol.ConnectionID
	DestConnectionID protocol.ConnectionID
}

var _ eventDetails = &eventConnectionStarted{}

func (e eventConnectionStarted) Category() category { return categoryTransport }
func (e eventConnectionStarted) Name() string       { return "connection_started" }
func (e eventConnectionStarted) IsNil() bool        { return false }

func (e eventConnectionStarted) MarshalJSONObject(enc *gojay.Encoder) {
	if utils.IsIPv4(e.SrcAddr.IP) {
		enc.StringKey("ip_version", "ipv4")
	} else {
		enc.StringKey("ip_version", "ipv6")
	}
	enc.StringKey("src_ip", e.SrcAddr.IP.String())
	enc.IntKey("src_port", e.SrcAddr.Port)
	enc.StringKey("dst_ip", e.DestAddr.IP.String())
	enc.IntKey("dst_port", e.DestAddr.Port)
	enc.StringKey("src_cid", e.SrcConnectionID.String())
	enc.StringKey("dst_cid", e.DestConnectionID.String())
}

type eventVersionNegotiated struct {
	clientVersions, serverVersions []versionNumber
	chosenVersion                  versionNumber
}

func (e eventVersionNegotiated) Category() category { return categoryTransport }
func (e eventVersionNegotiated) Name() string       { return "version_information" }
func (e eventVersionNegotiated) IsNil() bool        { return false }

func (e eventVersionNegotiated) MarshalJSONObject(enc *gojay.Encoder) {
	if len(e.clientVersions) > 0 {
		enc.ArrayKey("client_versions", versions(e.clientVersions))
	}
	if len(e.serverVersions) > 0 {
		enc.ArrayKey("server_versions", versions(e.serverVersions))
	}
	enc.StringKey("chosen_version", e.chosenVersion.String())
}

type eventConnectionClosed struct {
	e error
}

func (e eventConnectionClosed) Category() category { return categoryTransport }
func (e eventConnectionClosed) Name() string       { return "connection_closed" }
func (e eventConnectionClosed) IsNil() bool        { return false }

func (e eventConnectionClosed) MarshalJSONObject(enc *gojay.Encoder) {
	var (
		statelessResetErr     *quic.StatelessResetError
		handshakeTimeoutErr   *quic.HandshakeTimeoutError
		idleTimeoutErr        *quic.IdleTimeoutError
		applicationErr        *quic.ApplicationError
		transportErr          *quic.TransportError
		versionNegotiationErr *quic.VersionNegotiationError
	)
	switch {
	case errors.As(e.e, &statelessResetErr):
		enc.StringKey("owner", ownerRemote.String())
		enc.StringKey("trigger", "stateless_reset")
		enc.StringKey("stateless_reset_token", fmt.Sprintf("%x", statelessResetErr.Token))
	case errors.As(e.e, &handshakeTimeoutErr):
		enc.StringKey("owner", ownerLocal.String())
		enc.StringKey("trigger", "handshake_timeout")
	case errors.As(e.e, &idleTimeoutErr):
		enc.StringKey("owner", ownerLocal.String())
		enc.StringKey("trigger", "idle_timeout")
	case errors.As(e.e, &applicationErr):
		owner := ownerLocal
		if applicationErr.Remote {
			owner = ownerRemote
		}
		enc.StringKey("owner", owner.String())
		enc.Uint64Key("application_code", uint64(applicationErr.ErrorCode))
		enc.StringKey("reason", applicationErr.ErrorMessage)
	case errors.As(e.e, &transportErr):
		owner := ownerLocal
		if transportErr.Remote {
			owner = ownerRemote
		}
		enc.StringKey("owner", owner.String())
		enc.StringKey("connection_code", transportError(transportErr.ErrorCode).String())
		enc.StringKey("reason", transportErr.ErrorMessage)
	case errors.As(e.e, &versionNegotiationErr):
		enc.StringKey("trigger", "version_mismatch")
	}
}

type eventPacketSent struct {
	Header        gojay.MarshalerJSONObject // either a shortHeader or a packetHeader
	Length        logging.ByteCount
	PayloadLength logging.ByteCount
	Frames        frames
	IsCoalesced   bool
	ECN           logging.ECN
	Trigger       string
}

var _ eventDetails = eventPacketSent{}

func (e eventPacketSent) Category() category { return categoryTransport }
func (e eventPacketSent) Name() string       { return "packet_sent" }
func (e eventPacketSent) IsNil() bool        { return false }

func (e eventPacketSent) MarshalJSONObject(enc *gojay.Encoder) {
	enc.ObjectKey("header", e.Header)
	enc.ObjectKey("raw", rawInfo{Length: e.Length, PayloadLength: e.PayloadLength})
	enc.ArrayKeyOmitEmpty("frames", e.Frames)
	enc.BoolKeyOmitEmpty("is_coalesced", e.IsCoalesced)
	if e.ECN != logging.ECNUnsupported {
		enc.StringKey("ecn", ecn(e.ECN).String())
	}
	enc.StringKeyOmitEmpty("trigger", e.Trigger)
}

type eventPacketReceived struct {
	Header        gojay.MarshalerJSONObject // either a shortHeader or a packetHeader
	Length        logging.ByteCount
	PayloadLength logging.ByteCount
	Frames        frames
	ECN           logging.ECN
	IsCoalesced   bool
	Trigger       string
}

var _ eventDetails = eventPacketReceived{}

func (e eventPacketReceived) Category() category { return categoryTransport }
func (e eventPacketReceived) Name() string       { return "packet_received" }
func (e eventPacketReceived) IsNil() bool        { return false }

func (e eventPacketReceived) MarshalJSONObject(enc *gojay.Encoder) {
	enc.ObjectKey("header", e.Header)
	enc.ObjectKey("raw", rawInfo{Length: e.Length, PayloadLength: e.PayloadLength})
	enc.ArrayKeyOmitEmpty("frames", e.Frames)
	enc.BoolKeyOmitEmpty("is_coalesced", e.IsCoalesced)
	if e.ECN != logging.ECNUnsupported {
		enc.StringKey("ecn", ecn(e.ECN).String())
	}
	enc.StringKeyOmitEmpty("trigger", e.Trigger)
}

type eventRetryReceived struct {
	Header packetHeader
}

func (e eventRetryReceived) Category() category { return categoryTransport }
func (e eventRetryReceived) Name() string       { return "packet_received" }
func (e eventRetryReceived) IsNil() bool        { return false }

func (e eventRetryReceived) MarshalJSONObject(enc *gojay.Encoder) {
	enc.ObjectKey("header", e.Header)
}

type eventVersionNegotiationReceived struct {
	Header            packetHeaderVersionNegotiation
	SupportedVersions []versionNumber
}

func (e eventVersionNegotiationReceived) Category() category { return categoryTransport }
func (e eventVersionNegotiationReceived) Name() string       { return "packet_received" }
func (e eventVersionNegotiationReceived) IsNil() bool        { return false }

func (e eventVersionNegotiationReceived) MarshalJSONObject(enc *gojay.Encoder) {
	enc.ObjectKey("header", e.Header)
	enc.ArrayKey("supported_versions", versions(e.SupportedVersions))
}

type eventPacketBuffered struct {
	PacketType logging.PacketType
	PacketSize protocol.ByteCount
}

func (e eventPacketBuffered) Category() category { return categoryTransport }
func (e eventPacketBuffered) Name() string       { return "packet_buffered" }
func (e eventPacketBuffered) IsNil() bool        { return false }

func (e eventPacketBuffered) MarshalJSONObject(enc *gojay.Encoder) {
	//nolint:gosimple
	enc.ObjectKey("header", packetHeaderWithType{PacketType: e.PacketType})
	enc.ObjectKey("raw", rawInfo{Length: e.PacketSize})
	enc.StringKey("trigger", "keys_unavailable")
}

type eventPacketDropped struct {
	PacketType logging.PacketType
	PacketSize protocol.ByteCount
	Trigger    packetDropReason
}

func (e eventPacketDropped) Category() category { return categoryTransport }
func (e eventPacketDropped) Name() string       { return "packet_dropped" }
func (e eventPacketDropped) IsNil() bool        { return false }

func (e eventPacketDropped) MarshalJSONObject(enc *gojay.Encoder) {
	enc.ObjectKey("header", packetHeaderWithType{PacketType: e.PacketType})
	enc.ObjectKey("raw", rawInfo{Length: e.PacketSize})
	enc.StringKey("trigger", e.Trigger.String())
}

type metrics struct {
	MinRTT      time.Duration
	SmoothedRTT time.Duration
	LatestRTT   time.Duration
	RTTVariance time.Duration

	CongestionWindow protocol.ByteCount
	BytesInFlight    protocol.ByteCount
	PacketsInFlight  int
}

type eventMetricsUpdated struct {
	Last    *metrics
	Current *metrics
}

func (e eventMetricsUpdated) Category() category { return categoryRecovery }
func (e eventMetricsUpdated) Name() string       { return "metrics_updated" }
func (e eventMetricsUpdated) IsNil() bool        { return false }

func (e eventMetricsUpdated) MarshalJSONObject(enc *gojay.Encoder) {
	if e.Last == nil || e.Last.MinRTT != e.Current.MinRTT {
		enc.FloatKey("min_rtt", milliseconds(e.Current.MinRTT))
	}
	if e.Last == nil || e.Last.SmoothedRTT != e.Current.SmoothedRTT {
		enc.FloatKey("smoothed_rtt", milliseconds(e.Current.SmoothedRTT))
	}
	if e.Last == nil || e.Last.LatestRTT != e.Current.LatestRTT {
		enc.FloatKey("latest_rtt", milliseconds(e.Current.LatestRTT))
	}
	if e.Last == nil || e.Last.RTTVariance != e.Current.RTTVariance {
		enc.FloatKey("rtt_variance", milliseconds(e.Current.RTTVariance))
	}

	if e.Last == nil || e.Last.CongestionWindow != e.Current.CongestionWindow {
		enc.Uint64Key("congestion_window", uint64(e.Current.CongestionWindow))
	}
	if e.Last == nil || e.Last.BytesInFlight != e.Current.BytesInFlight {
		enc.Uint64Key("bytes_in_flight", uint64(e.Current.BytesInFlight))
	}
	if e.Last == nil || e.Last.PacketsInFlight != e.Current.PacketsInFlight {
		enc.Uint64KeyOmitEmpty("packets_in_flight", uint64(e.Current.PacketsInFlight))
	}
}

type eventUpdatedPTO struct {
	Value uint32
}

func (e eventUpdatedPTO) Category() category { return categoryRecovery }
func (e eventUpdatedPTO) Name() string       { return "metrics_updated" }
func (e eventUpdatedPTO) IsNil() bool        { return false }

func (e eventUpdatedPTO) MarshalJSONObject(enc *gojay.Encoder) {
	enc.Uint32Key("pto_count", e.Value)
}

type eventPacketLost struct {
	PacketType   logging.PacketType
	PacketNumber protocol.PacketNumber
	Trigger      packetLossReason
}

func (e eventPacketLost) Category() category { return categoryRecovery }
func (e eventPacketLost) Name() string       { return "packet_lost" }
func (e eventPacketLost) IsNil() bool        { return false }

func (e eventPacketLost) MarshalJSONObject(enc *gojay.Encoder) {
	enc.ObjectKey("header", packetHeaderWithTypeAndPacketNumber{
		PacketType:   e.PacketType,
		PacketNumber: e.PacketNumber,
	})
	enc.StringKey("trigger", e.Trigger.String())
}

type eventKeyUpdated struct {
	Trigger    keyUpdateTrigger
	KeyType    keyType
	Generation protocol.KeyPhase
	// we don't log the keys here, so we don't need `old` and `new`.
}

func (e eventKeyUpdated) Category() category { return categorySecurity }
func (e eventKeyUpdated) Name() string       { return "key_updated" }
func (e eventKeyUpdated) IsNil() bool        { return false }

func (e eventKeyUpdated) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("trigger", e.Trigger.String())
	enc.StringKey("key_type", e.KeyType.String())
	if e.KeyType == keyTypeClient1RTT || e.KeyType == keyTypeServer1RTT {
		enc.Uint64Key("generation", uint64(e.Generation))
	}
}

type eventKeyDiscarded struct {
	KeyType    keyType
	Generation protocol.KeyPhase
}

func (e eventKeyDiscarded) Category() category { return categorySecurity }
func (e eventKeyDiscarded) Name() string       { return "key_discarded" }
func (e eventKeyDiscarded) IsNil() bool        { return false }

func (e eventKeyDiscarded) MarshalJSONObject(enc *gojay.Encoder) {
	if e.KeyType != keyTypeClient1RTT && e.KeyType != keyTypeServer1RTT {
		enc.StringKey("trigger", "tls")
	}
	enc.StringKey("key_type", e.KeyType.String())
	if e.KeyType == keyTypeClient1RTT || e.KeyType == keyTypeServer1RTT {
		enc.Uint64Key("generation", uint64(e.Generation))
	}
}

type eventTransportParameters struct {
	Restore bool
	Owner   owner
	SentBy  protocol.Perspective

	OriginalDestinationConnectionID protocol.ConnectionID
	InitialSourceConnectionID       protocol.ConnectionID
	RetrySourceConnectionID         *protocol.ConnectionID

	StatelessResetToken     *protocol.StatelessResetToken
	DisableActiveMigration  bool
	MaxIdleTimeout          time.Duration
	MaxUDPPayloadSize       protocol.ByteCount
	AckDelayExponent        uint8
	MaxAckDelay             time.Duration
	ActiveConnectionIDLimit uint64

	InitialMaxData                 protocol.ByteCount
	InitialMaxStreamDataBidiLocal  protocol.ByteCount
	InitialMaxStreamDataBidiRemote protocol.ByteCount
	InitialMaxStreamDataUni        protocol.ByteCount
	InitialMaxStreamsBidi          int64
	InitialMaxStreamsUni           int64

	PreferredAddress *preferredAddress

	MaxDatagramFrameSize protocol.ByteCount
}

func (e eventTransportParameters) Category() category { return categoryTransport }
func (e eventTransportParameters) Name() string {
	if e.Restore {
		return "parameters_restored"
	}
	return "parameters_set"
}
func (e eventTransportParameters) IsNil() bool { return false }

func (e eventTransportParameters) MarshalJSONObject(enc *gojay.Encoder) {
	if !e.Restore {
		enc.StringKey("owner", e.Owner.String())
		if e.SentBy == protocol.PerspectiveServer {
			enc.StringKey("original_destination_connection_id", e.OriginalDestinationConnectionID.String())
			if e.StatelessResetToken != nil {
				enc.StringKey("stateless_reset_token", fmt.Sprintf("%x", e.StatelessResetToken[:]))
			}
			if e.RetrySourceConnectionID != nil {
				enc.StringKey("retry_source_connection_id", (*e.RetrySourceConnectionID).String())
			}
		}
		enc.StringKey("initial_source_connection_id", e.InitialSourceConnectionID.String())
	}
	enc.BoolKey("disable_active_migration", e.DisableActiveMigration)
	enc.FloatKeyOmitEmpty("max_idle_timeout", milliseconds(e.MaxIdleTimeout))
	enc.Int64KeyNullEmpty("max_udp_payload_size", int64(e.MaxUDPPayloadSize))
	enc.Uint8KeyOmitEmpty("ack_delay_exponent", e.AckDelayExponent)
	enc.FloatKeyOmitEmpty("max_ack_delay", milliseconds(e.MaxAckDelay))
	enc.Uint64KeyOmitEmpty("active_connection_id_limit", e.ActiveConnectionIDLimit)

	enc.Int64KeyOmitEmpty("initial_max_data", int64(e.InitialMaxData))
	enc.Int64KeyOmitEmpty("initial_max_stream_data_bidi_local", int64(e.InitialMaxStreamDataBidiLocal))
	enc.Int64KeyOmitEmpty("initial_max_stream_data_bidi_remote", int64(e.InitialMaxStreamDataBidiRemote))
	enc.Int64KeyOmitEmpty("initial_max_stream_data_uni", int64(e.InitialMaxStreamDataUni))
	enc.Int64KeyOmitEmpty("initial_max_streams_bidi", e.InitialMaxStreamsBidi)
	enc.Int64KeyOmitEmpty("initial_max_streams_uni", e.InitialMaxStreamsUni)

	if e.PreferredAddress != nil {
		enc.ObjectKey("preferred_address", e.PreferredAddress)
	}
	if e.MaxDatagramFrameSize != protocol.InvalidByteCount {
		enc.Int64Key("max_datagram_frame_size", int64(e.MaxDatagramFrameSize))
	}
}

type preferredAddress struct {
	IPv4, IPv6          net.IP
	PortV4, PortV6      uint16
	ConnectionID        protocol.ConnectionID
	StatelessResetToken protocol.StatelessResetToken
}

var _ gojay.MarshalerJSONObject = &preferredAddress{}

func (a preferredAddress) IsNil() bool { return false }
func (a preferredAddress) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("ip_v4", a.IPv4.String())
	enc.Uint16Key("port_v4", a.PortV4)
	enc.StringKey("ip_v6", a.IPv6.String())
	enc.Uint16Key("port_v6", a.PortV6)
	enc.StringKey("connection_id", a.ConnectionID.String())
	enc.StringKey("stateless_reset_token", fmt.Sprintf("%x", a.StatelessResetToken))
}

type eventLossTimerSet struct {
	TimerType timerType
	EncLevel  protocol.EncryptionLevel
	Delta     time.Duration
}

func (e eventLossTimerSet) Category() category { return categoryRecovery }
func (e eventLossTimerSet) Name() string       { return "loss_timer_updated" }
func (e eventLossTimerSet) IsNil() bool        { return false }

func (e eventLossTimerSet) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("event_type", "set")
	enc.StringKey("timer_type", e.TimerType.String())
	enc.StringKey("packet_number_space", encLevelToPacketNumberSpace(e.EncLevel))
	enc.Float64Key("delta", milliseconds(e.Delta))
}

type eventLossTimerExpired struct {
	TimerType timerType
	EncLevel  protocol.EncryptionLevel
}

func (e eventLossTimerExpired) Category() category { return categoryRecovery }
func (e eventLossTimerExpired) Name() string       { return "loss_timer_updated" }
func (e eventLossTimerExpired) IsNil() bool        { return false }

func (e eventLossTimerExpired) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("event_type", "expired")
	enc.StringKey("timer_type", e.TimerType.String())
	enc.StringKey("packet_number_space", encLevelToPacketNumberSpace(e.EncLevel))
}

type eventLossTimerCanceled struct{}

func (e eventLossTimerCanceled) Category() category { return categoryRecovery }
func (e eventLossTimerCanceled) Name() string       { return "loss_timer_updated" }
func (e eventLossTimerCanceled) IsNil() bool        { return false }

func (e eventLossTimerCanceled) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("event_type", "cancelled")
}

type eventCongestionStateUpdated struct {
	state congestionState
}

func (e eventCongestionStateUpdated) Category() category { return categoryRecovery }
func (e eventCongestionStateUpdated) Name() string       { return "congestion_state_updated" }
func (e eventCongestionStateUpdated) IsNil() bool        { return false }

func (e eventCongestionStateUpdated) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("new", e.state.String())
}

type eventECNStateUpdated struct {
	state   logging.ECNState
	trigger logging.ECNStateTrigger
}

func (e eventECNStateUpdated) Category() category { return categoryRecovery }
func (e eventECNStateUpdated) Name() string       { return "ecn_state_updated" }
func (e eventECNStateUpdated) IsNil() bool        { return false }

func (e eventECNStateUpdated) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("new", ecnState(e.state).String())
	enc.StringKeyOmitEmpty("trigger", ecnStateTrigger(e.trigger).String())
}

type eventGeneric struct {
	name string
	msg  string
}

func (e eventGeneric) Category() category { return categoryTransport }
func (e eventGeneric) Name() string       { return e.name }
func (e eventGeneric) IsNil() bool        { return false }

func (e eventGeneric) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("details", e.msg)
}
