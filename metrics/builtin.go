package metrics

const (
	VUsName               = "vus" //nolint:revive
	VUsMaxName            = "vus_max"
	IterationsName        = "iterations"
	IterationDurationName = "iteration_duration"
	DroppedIterationsName = "dropped_iterations"

	ChecksName        = "checks"
	GroupDurationName = "group_duration"

	HTTPReqsName              = "http_reqs"
	HTTPReqFailedName         = "http_req_failed"
	HTTPReqDurationName       = "http_req_duration"
	HTTPReqBlockedName        = "http_req_blocked"
	HTTPReqConnectingName     = "http_req_connecting"
	HTTPReqTLSHandshakingName = "http_req_tls_handshaking"
	HTTPReqSendingName        = "http_req_sending"
	HTTPReqWaitingName        = "http_req_waiting"
	HTTPReqReceivingName      = "http_req_receiving"

	WSSessionsName         = "ws_sessions"
	WSMessagesSentName     = "ws_msgs_sent"
	WSMessagesReceivedName = "ws_msgs_received"
	WSPingName             = "ws_ping"
	WSSessionDurationName  = "ws_session_duration"
	WSConnectingName       = "ws_connecting"

	GRPCReqDurationName = "grpc_req_duration"

	DataSentName     = "data_sent"
	DataReceivedName = "data_received"
)

// BuiltinMetrics represent all the builtin metrics of k6
type BuiltinMetrics struct {
	VUs               *Metric
	VUsMax            *Metric
	Iterations        *Metric
	IterationDuration *Metric
	DroppedIterations *Metric

	// Runner-emitted.
	Checks        *Metric
	GroupDuration *Metric

	// HTTP-related.
	HTTPReqs              *Metric
	HTTPReqFailed         *Metric
	HTTPReqDuration       *Metric
	HTTPReqBlocked        *Metric
	HTTPReqConnecting     *Metric
	HTTPReqTLSHandshaking *Metric
	HTTPReqSending        *Metric
	HTTPReqWaiting        *Metric
	HTTPReqReceiving      *Metric

	// Websocket-related
	WSSessions         *Metric
	WSMessagesSent     *Metric
	WSMessagesReceived *Metric
	WSPing             *Metric
	WSSessionDuration  *Metric
	WSConnecting       *Metric

	// gRPC-related
	GRPCReqDuration *Metric

	// Network-related; used for future protocols as well.
	DataSent     *Metric
	DataReceived *Metric
}

// RegisterBuiltinMetrics register and returns the builtin metrics in the provided registry
func RegisterBuiltinMetrics(registry *Registry) *BuiltinMetrics {
	return &BuiltinMetrics{
		VUs:               registry.MustNewMetric(VUsName, Gauge),
		VUsMax:            registry.MustNewMetric(VUsMaxName, Gauge),
		Iterations:        registry.MustNewMetric(IterationsName, Counter),
		IterationDuration: registry.MustNewMetric(IterationDurationName, Trend, Time),
		DroppedIterations: registry.MustNewMetric(DroppedIterationsName, Counter),

		Checks:        registry.MustNewMetric(ChecksName, Rate),
		GroupDuration: registry.MustNewMetric(GroupDurationName, Trend, Time),

		HTTPReqs:              registry.MustNewMetric(HTTPReqsName, Counter),
		HTTPReqFailed:         registry.MustNewMetric(HTTPReqFailedName, Rate),
		HTTPReqDuration:       registry.MustNewMetric(HTTPReqDurationName, Trend, Time),
		HTTPReqBlocked:        registry.MustNewMetric(HTTPReqBlockedName, Trend, Time),
		HTTPReqConnecting:     registry.MustNewMetric(HTTPReqConnectingName, Trend, Time),
		HTTPReqTLSHandshaking: registry.MustNewMetric(HTTPReqTLSHandshakingName, Trend, Time),
		HTTPReqSending:        registry.MustNewMetric(HTTPReqSendingName, Trend, Time),
		HTTPReqWaiting:        registry.MustNewMetric(HTTPReqWaitingName, Trend, Time),
		HTTPReqReceiving:      registry.MustNewMetric(HTTPReqReceivingName, Trend, Time),

		WSSessions:         registry.MustNewMetric(WSSessionsName, Counter),
		WSMessagesSent:     registry.MustNewMetric(WSMessagesSentName, Counter),
		WSMessagesReceived: registry.MustNewMetric(WSMessagesReceivedName, Counter),
		WSPing:             registry.MustNewMetric(WSPingName, Trend, Time),
		WSSessionDuration:  registry.MustNewMetric(WSSessionDurationName, Trend, Time),
		WSConnecting:       registry.MustNewMetric(WSConnectingName, Trend, Time),

		GRPCReqDuration: registry.MustNewMetric(GRPCReqDurationName, Trend, Time),

		DataSent:     registry.MustNewMetric(DataSentName, Counter, Data),
		DataReceived: registry.MustNewMetric(DataReceivedName, Counter, Data),
	}
}
