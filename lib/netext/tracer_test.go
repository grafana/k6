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
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"testing"

	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
)

func TestTracer(t *testing.T) {
	client := &http.Client{
		Transport: &http.Transport{DialContext: NewDialer(net.Dialer{}).DialContext},
	}

	for _, isReuse := range []bool{false, true} {
		name := "First"
		if isReuse {
			name = "Reuse"
		}
		t.Run(name, func(t *testing.T) {
			tracer := &Tracer{}
			req, err := http.NewRequest("GET", "http://httpbin.org/get", nil)
			if !assert.NoError(t, err) {
				return
			}
			res, err := client.Do(req.WithContext(WithTracer(context.Background(), tracer)))
			if !assert.NoError(t, err) {
				return
			}
			_, err = io.Copy(ioutil.Discard, res.Body)
			assert.NoError(t, err)
			assert.NoError(t, res.Body.Close())
			samples := tracer.Done().Samples(map[string]string{"tag": "value"})

			assert.Len(t, samples, 9)
			seenMetrics := map[*stats.Metric]bool{}
			for _, s := range samples {
				assert.NotContains(t, seenMetrics, s.Metric)
				seenMetrics[s.Metric] = true

				assert.False(t, s.Time.IsZero())
				assert.Equal(t, map[string]string{"tag": "value"}, s.Tags)

				switch s.Metric {
				case metrics.HTTPReqs:
					assert.Equal(t, 1.0, s.Value)
				case metrics.HTTPReqConnecting:
					if isReuse {
						assert.Equal(t, 0.0, s.Value)
						break
					}
					fallthrough
				case metrics.HTTPReqDuration, metrics.HTTPReqBlocked, metrics.HTTPReqSending, metrics.HTTPReqWaiting, metrics.HTTPReqReceiving, metrics.DataSent, metrics.DataReceived:
					assert.True(t, s.Value > 0.0, "%s is <= 0", s.Metric.Name)
				default:
					t.Errorf("unexpected metric: %s", s.Metric.Name)
				}
			}
		})
	}
}
