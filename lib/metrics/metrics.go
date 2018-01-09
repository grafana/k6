package metrics

import (
	"github.com/loadimpact/k6/stats"
)

var (
	// Engine-emitted.
	VUs        = stats.New("vus", stats.Gauge)
	VUsMax     = stats.New("vus_max", stats.Gauge)
	Iterations = stats.New("iterations", stats.Counter)
	Errors     = stats.New("errors", stats.Counter)

	// Runner-emitted.
	Checks        = stats.New("checks", stats.Rate)
	GroupDuration = stats.New("group_duration", stats.Trend, stats.Time)

	// HTTP-related.
	HTTPReqs          = stats.New("http_reqs", stats.Counter)
	HTTPReqDuration   = stats.New("http_req_duration", stats.Trend, stats.Time)
	HTTPReqBlocked    = stats.New("http_req_blocked", stats.Trend, stats.Time)
	HTTPReqConnecting = stats.New("http_req_connecting", stats.Trend, stats.Time)
	HTTPReqSending    = stats.New("http_req_sending", stats.Trend, stats.Time)
	HTTPReqWaiting    = stats.New("http_req_waiting", stats.Trend, stats.Time)
	HTTPReqReceiving  = stats.New("http_req_receiving", stats.Trend, stats.Time)
	HTTPReqTLSShaking = stats.New("http_req_tls_shaking", stats.Trend, stats.Time)

	// Websocket-related
	WSSessions         = stats.New("ws_sessions", stats.Counter)
	WSMessagesSent     = stats.New("ws_msgs_sent", stats.Counter)
	WSMessagesReceived = stats.New("ws_msgs_received", stats.Counter)
	WSPing             = stats.New("ws_ping", stats.Trend)
	WSSessionDuration  = stats.New("ws_session_duration", stats.Trend, stats.Time)
	WSConnecting       = stats.New("ws_connecting", stats.Trend, stats.Time)

	// Network-related; used for future protocols as well.
	DataSent     = stats.New("data_sent", stats.Counter, stats.Data)
	DataReceived = stats.New("data_received", stats.Counter, stats.Data)
)
