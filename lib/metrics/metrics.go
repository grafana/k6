/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package metrics

import (
	"go.k6.io/k6/stats"
)

const (
	VUsName               = "vus" //nolint:golint
	VUsMaxName            = "vus_max"
	IterationsName        = "iterations"
	IterationDurationName = "iteration_duration"
	DroppedIterationsName = "dropped_iterations"
	ErrorsName            = "errors"

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
	VUs               *stats.Metric
	VUsMax            *stats.Metric
	Iterations        *stats.Metric
	IterationDuration *stats.Metric
	DroppedIterations *stats.Metric
	Errors            *stats.Metric

	// Runner-emitted.
	Checks        *stats.Metric
	GroupDuration *stats.Metric

	// HTTP-related.
	HTTPReqs              *stats.Metric
	HTTPReqFailed         *stats.Metric
	HTTPReqDuration       *stats.Metric
	HTTPReqBlocked        *stats.Metric
	HTTPReqConnecting     *stats.Metric
	HTTPReqTLSHandshaking *stats.Metric
	HTTPReqSending        *stats.Metric
	HTTPReqWaiting        *stats.Metric
	HTTPReqReceiving      *stats.Metric

	// Websocket-related
	WSSessions         *stats.Metric
	WSMessagesSent     *stats.Metric
	WSMessagesReceived *stats.Metric
	WSPing             *stats.Metric
	WSSessionDuration  *stats.Metric
	WSConnecting       *stats.Metric

	// gRPC-related
	GRPCReqDuration *stats.Metric

	// Network-related; used for future protocols as well.
	DataSent     *stats.Metric
	DataReceived *stats.Metric
}

// RegisterBuiltinMetrics register and returns the builtin metrics in the provided registry
func RegisterBuiltinMetrics(registry *Registry) *BuiltinMetrics {
	return &BuiltinMetrics{
		VUs:               registry.MustNewMetric(VUsName, stats.Gauge),
		VUsMax:            registry.MustNewMetric(VUsMaxName, stats.Gauge),
		Iterations:        registry.MustNewMetric(IterationsName, stats.Counter),
		IterationDuration: registry.MustNewMetric(IterationDurationName, stats.Trend, stats.Time),
		DroppedIterations: registry.MustNewMetric(DroppedIterationsName, stats.Counter),
		Errors:            registry.MustNewMetric(ErrorsName, stats.Counter),

		Checks:        registry.MustNewMetric(ChecksName, stats.Rate),
		GroupDuration: registry.MustNewMetric(GroupDurationName, stats.Trend, stats.Time),

		HTTPReqs:              registry.MustNewMetric(HTTPReqsName, stats.Counter),
		HTTPReqFailed:         registry.MustNewMetric(HTTPReqFailedName, stats.Rate),
		HTTPReqDuration:       registry.MustNewMetric(HTTPReqDurationName, stats.Trend, stats.Time),
		HTTPReqBlocked:        registry.MustNewMetric(HTTPReqBlockedName, stats.Trend, stats.Time),
		HTTPReqConnecting:     registry.MustNewMetric(HTTPReqConnectingName, stats.Trend, stats.Time),
		HTTPReqTLSHandshaking: registry.MustNewMetric(HTTPReqTLSHandshakingName, stats.Trend, stats.Time),
		HTTPReqSending:        registry.MustNewMetric(HTTPReqSendingName, stats.Trend, stats.Time),
		HTTPReqWaiting:        registry.MustNewMetric(HTTPReqWaitingName, stats.Trend, stats.Time),
		HTTPReqReceiving:      registry.MustNewMetric(HTTPReqReceivingName, stats.Trend, stats.Time),

		WSSessions:         registry.MustNewMetric(WSSessionsName, stats.Counter),
		WSMessagesSent:     registry.MustNewMetric(WSMessagesSentName, stats.Counter),
		WSMessagesReceived: registry.MustNewMetric(WSMessagesReceivedName, stats.Counter),
		WSPing:             registry.MustNewMetric(WSPingName, stats.Trend, stats.Time),
		WSSessionDuration:  registry.MustNewMetric(WSSessionDurationName, stats.Trend, stats.Time),
		WSConnecting:       registry.MustNewMetric(WSConnectingName, stats.Trend, stats.Time),

		GRPCReqDuration: registry.MustNewMetric(GRPCReqDurationName, stats.Trend, stats.Time),

		DataSent:     registry.MustNewMetric(DataSentName, stats.Counter, stats.Data),
		DataReceived: registry.MustNewMetric(DataReceivedName, stats.Counter, stats.Data),
	}
}
