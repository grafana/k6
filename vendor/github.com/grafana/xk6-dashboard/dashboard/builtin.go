package dashboard

import (
	"sort"

	"go.k6.io/k6/metrics"
)

const (
	// xk6-dashboard's time
	keyTime = "time"

	// from k6 metrics/builtin.go
	keyVUsName               = metrics.VUsName
	keyVUsMaxName            = metrics.VUsMaxName
	keyIterationsName        = metrics.IterationsName
	keyIterationDurationName = metrics.IterationDurationName
	keyDroppedIterationsName = metrics.DroppedIterationsName

	keyChecksName        = metrics.ChecksName
	keyGroupDurationName = metrics.GroupDurationName

	keyHTTPReqsName              = metrics.HTTPReqsName
	keyHTTPReqFailedName         = metrics.HTTPReqFailedName
	keyHTTPReqDurationName       = metrics.HTTPReqDurationName
	keyHTTPReqBlockedName        = metrics.HTTPReqBlockedName
	keyHTTPReqConnectingName     = metrics.HTTPReqConnectingName
	keyHTTPReqTLSHandshakingName = metrics.HTTPReqTLSHandshakingName
	keyHTTPReqSendingName        = metrics.HTTPReqSendingName
	keyHTTPReqWaitingName        = metrics.HTTPReqWaitingName
	keyHTTPReqReceivingName      = metrics.HTTPReqReceivingName

	keyWSSessionsName         = metrics.WSSessionsName
	keyWSMessagesSentName     = metrics.WSMessagesSentName
	keyWSMessagesReceivedName = metrics.WSMessagesReceivedName
	keyWSPingName             = metrics.WSPingName
	keyWSSessionDurationName  = metrics.WSSessionDurationName
	keyWSConnectingName       = metrics.WSConnectingName

	keyGRPCReqDurationName = metrics.GRPCReqDurationName

	keyDataSentName     = metrics.DataSentName
	keyDataReceivedName = metrics.DataReceivedName

	// from xk6-browser

	keyfidName  = "browser_web_vital_fid"
	keyttfbName = "browser_web_vital_ttfb"
	keylcpName  = "browser_web_vital_lcp"
	keyclsName  = "browser_web_vital_cls"
	keyinpName  = "browser_web_vital_inp"
	keyfcpName  = "browser_web_vital_fcp"

	keybrowserDataSentName        = "browser_data_sent"
	keybrowserDataReceivedName    = "browser_data_received"
	keybrowserHTTPReqDurationName = "browser_http_req_duration"
	keybrowserHTTPReqFailedName   = "browser_http_req_failed"

	// from k6/grpc

	keyGRPCStreamsName             = "grpc_streams"
	keyGRPCStreamsMsgsReceivedName = "grpc_streams_msgs_received"
	keyGRPCStreamsMsgsSentName     = "grpc_streams_msgs_sent"
)

var builtinNames = []string{ //nolint:gochecknoglobals
	keyTime,

	keyVUsName,
	keyVUsMaxName,
	keyIterationsName,
	keyIterationDurationName,
	keyDroppedIterationsName,

	keyChecksName,
	keyGroupDurationName,

	keyHTTPReqsName,
	keyHTTPReqFailedName,
	keyHTTPReqDurationName,
	keyHTTPReqBlockedName,
	keyHTTPReqConnectingName,
	keyHTTPReqTLSHandshakingName,
	keyHTTPReqSendingName,
	keyHTTPReqWaitingName,
	keyHTTPReqReceivingName,

	keyWSSessionsName,
	keyWSMessagesSentName,
	keyWSMessagesReceivedName,
	keyWSPingName,
	keyWSSessionDurationName,
	keyWSConnectingName,

	keyGRPCReqDurationName,

	keyDataSentName,
	keyDataReceivedName,

	keyfidName,
	keyttfbName,
	keylcpName,
	keyclsName,
	keyinpName,
	keyfcpName,

	keybrowserDataSentName,
	keybrowserDataReceivedName,
	keybrowserHTTPReqDurationName,
	keybrowserHTTPReqFailedName,

	keyGRPCStreamsName,
	keyGRPCStreamsMsgsReceivedName,
	keyGRPCStreamsMsgsSentName,
}

func init() {
	sort.Strings(builtinNames)
}

func isBuiltin(name string) bool {
	idx := sort.SearchStrings(builtinNames, name)

	return idx < len(builtinNames) && builtinNames[idx] == name
}
