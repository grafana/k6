/*
 *
 * xk6-browser - a browser automation extension for k6
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

package common

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/dop251/goja"
	"github.com/grafana/xk6-browser/api"
	"github.com/stretchr/testify/assert"
	k6common "go.k6.io/k6/js/common"
)

func TestRequest(t *testing.T) {
	t.Parallel()
	rt := goja.New()
	ctx := k6common.WithRuntime(context.Background(), rt)
	ts := cdp.MonotonicTime(time.Now())
	wt := cdp.TimeSinceEpoch(time.Now())
	headers := map[string]interface{}{"key": "value"}
	evt := &network.EventRequestWillBeSent{
		RequestID: network.RequestID("1234"),
		Request: &network.Request{
			URL:      "https://test/post",
			Method:   "POST",
			Headers:  network.Headers(headers),
			PostData: "hello",
		},
		Timestamp: &ts,
		WallTime:  &wt,
	}
	req := NewRequest(ctx, evt, nil, nil, "intercept", false)

	t.Run("Headers()", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, map[string]string{"key": "value"}, req.Headers())
	})

	t.Run("HeadersArray()", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, []api.HTTPHeader{
			{Name: "key", Value: "value"},
		}, req.HeadersArray())
	})

	t.Run("HeaderValue()", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "value", req.HeaderValue("key").Export())
	})

	t.Run("Size()", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t,
			api.HTTPMessageSize{Headers: int64(33), Body: int64(5)},
			req.Size())
	})
}
