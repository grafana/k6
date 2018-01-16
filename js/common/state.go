/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package common

import (
	"net/http"
	"net/http/cookiejar"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/stats"
	"github.com/oxtoacart/bpool"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// Provides volatile state for a VU.
type State struct {
	// Global options.
	Options lib.Options

	// Logger. Avoid using the global logger.
	Logger *log.Logger

	// Current group; all emitted metrics are tagged with this.
	Group *lib.Group

	// Networking equipment.
	HTTPTransport http.RoundTripper
	Dialer        *netext.Dialer
	CookieJar     *cookiejar.Jar

	// Rate limits.
	RPSLimit *rate.Limiter

	// Sample buffer, emitted at the end of the iteration.
	Samples []stats.Sample

	// Bytes sent and received during this iteration. Use `sync/atomic`.
	BytesRead, BytesWritten int64

	// Buffer pool; use instead of allocating fresh buffers when possible.
	BPool *bpool.BufferPool

	Vu, Iteration int64
}
