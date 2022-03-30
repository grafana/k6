/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
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

package httpext

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

func BenchmarkMeasureAndEmitMetrics(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	samples := make(chan metrics.SampleContainer, 10)
	defer close(samples)
	go func() {
		for range samples {
		}
	}()
	logger := logrus.New()
	logger.Level = logrus.DebugLevel

	state := &lib.State{
		Options: lib.Options{
			RunTags:    &metrics.SampleTags{},
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		Samples: samples,
		Logger:  logger,
	}
	t := transport{
		state: state,
		ctx:   ctx,
	}

	b.ResetTimer()
	unfRequest := &unfinishedRequest{
		tracer: &Tracer{},
		response: &http.Response{
			StatusCode: 200,
		},
		request: &http.Request{
			URL: &url.URL{
				Host:   "example.com",
				Scheme: "https",
			},
		},
	}

	b.Run("no responseCallback", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			t.measureAndEmitMetrics(unfRequest)
		}
	})

	t.responseCallback = func(n int) bool { return true }

	b.Run("responseCallback", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			t.measureAndEmitMetrics(unfRequest)
		}
	})
}
