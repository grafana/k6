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

package netext

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/stats"
	"github.com/mccutchen/go-httpbin/httpbin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracer(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(httpbin.NewHTTPBin().Handler())
	defer srv.Close()

	transport, ok := srv.Client().Transport.(*http.Transport)
	assert.True(t, ok)
	transport.DialContext = NewDialer(net.Dialer{}).DialContext

	var prev int64
	assertLaterOrZero := func(t *testing.T, val int64, canBeZero bool) {
		if canBeZero && val == 0 {
			return
		}
		if prev > val {
			_, file, line, _ := runtime.Caller(1)
			t.Errorf("Expected %d to be greater or equal to %d (from %s:%d)", val, prev, file, line)
			return
		}
		prev = val
	}

	for tnum, isReuse := range []bool{false, true, true} {
		t.Run(fmt.Sprintf("Test #%d", tnum), func(t *testing.T) {
			// Do not enable parallel testing, test relies on sequential execution
			tracer := &Tracer{}
			req, err := http.NewRequest("GET", srv.URL+"/get", nil)
			require.NoError(t, err)

			res, err := transport.RoundTrip(req.WithContext(WithTracer(context.Background(), tracer)))
			require.NoError(t, err)

			_, err = io.Copy(ioutil.Discard, res.Body)
			assert.NoError(t, err)
			assert.NoError(t, res.Body.Close())
			samples := tracer.Done().Samples(map[string]string{"tag": "value"})

			assert.Empty(t, tracer.protoErrors)
			assertLaterOrZero(t, tracer.getConn, isReuse)
			assertLaterOrZero(t, tracer.connectStart, isReuse)
			assertLaterOrZero(t, tracer.connectDone, isReuse)
			assertLaterOrZero(t, tracer.tlsHandshakeStart, isReuse)
			assertLaterOrZero(t, tracer.tlsHandshakeDone, isReuse)
			assertLaterOrZero(t, tracer.gotConn, false)
			assertLaterOrZero(t, tracer.wroteRequest, false)
			assertLaterOrZero(t, tracer.gotFirstResponseByte, false)
			assertLaterOrZero(t, now(), false)

			assert.Len(t, samples, 8)
			seenMetrics := map[*stats.Metric]bool{}
			for i, s := range samples {
				assert.NotContains(t, seenMetrics, s.Metric)
				seenMetrics[s.Metric] = true

				assert.False(t, s.Time.IsZero())
				assert.Equal(t, map[string]string{"tag": "value"}, s.Tags)

				switch s.Metric {
				case metrics.HTTPReqs:
					assert.Equal(t, 1.0, s.Value)
					assert.Equal(t, 0, i, "`HTTPReqs` is reported before the other HTTP metrics")
				case metrics.HTTPReqConnecting, metrics.HTTPReqTLSHandshaking:
					if isReuse {
						assert.Equal(t, 0.0, s.Value)
						break
					}
					fallthrough
				case metrics.HTTPReqDuration, metrics.HTTPReqBlocked, metrics.HTTPReqSending, metrics.HTTPReqWaiting, metrics.HTTPReqReceiving:
					assert.True(t, s.Value > 0.0, "%s is <= 0", s.Metric.Name)
				default:
					t.Errorf("unexpected metric: %s", s.Metric.Name)
				}
			}
		})
	}
}

func TestTracerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(httpbin.NewHTTPBin().Handler())
	defer srv.Close()

	tracer := &Tracer{}
	req, err := http.NewRequest("GET", srv.URL+"/get", nil)
	require.NoError(t, err)

	_, err = http.DefaultTransport.RoundTrip(req.WithContext(WithTracer(context.Background(), tracer)))
	assert.Error(t, err)

	assert.Len(t, tracer.protoErrors, 1)
	assert.Error(t, tracer.protoErrors[0])
	assert.Equal(t, tracer.protoErrors, tracer.Done().Errors)
}
