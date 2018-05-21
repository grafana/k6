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
	"github.com/loadimpact/k6/stats"
)

//TODO: refactor this, using non thread-safe global variables seems like a bad idea for various reasons...

var (
	// Engine-emitted.
	VUs               = stats.New("vus", stats.Gauge)
	VUsMax            = stats.New("vus_max", stats.Gauge)
	Iterations        = stats.New("iterations", stats.Counter)
	IterationDuration = stats.New("iteration_duration", stats.Trend, stats.Time)
	Errors            = stats.New("errors", stats.Counter)

	// Runner-emitted.
	Checks        = stats.New("checks", stats.Rate)
	GroupDuration = stats.New("group_duration", stats.Trend, stats.Time)

	// HTTP-related.
	HTTPReqs              = stats.New("http_reqs", stats.Counter)
	HTTPReqDuration       = stats.New("http_req_duration", stats.Trend, stats.Time)
	HTTPReqBlocked        = stats.New("http_req_blocked", stats.Trend, stats.Time)
	HTTPReqConnecting     = stats.New("http_req_connecting", stats.Trend, stats.Time)
	HTTPReqTLSHandshaking = stats.New("http_req_tls_handshaking", stats.Trend, stats.Time)
	HTTPReqSending        = stats.New("http_req_sending", stats.Trend, stats.Time)
	HTTPReqWaiting        = stats.New("http_req_waiting", stats.Trend, stats.Time)
	HTTPReqReceiving      = stats.New("http_req_receiving", stats.Trend, stats.Time)

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
