/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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
	"sync/atomic"

	"github.com/loadimpact/k6/lib"
)

// BatchParsedHTTPRequest extends the normal parsed HTTP request with a pointer
// to a Response object, so that the batch goroutines can concurrently store the
// responses they receive, without any locking.
type BatchParsedHTTPRequest struct {
	*ParsedHTTPRequest
	Response *Response // this is modified by MakeBatchRequests()
}

// MakeBatchRequests concurrently makes multiple requests. It spawns
// min(reqCount, globalLimit) goroutines that asynchronously process all
// requests coming from the requests channel. Responses are recorded in the
// pointers contained in each BatchParsedHTTPRequest object, so they need to be
// pre-initialized. In addition, each processed request would emit either a nil
// value, or an error, via the returned errors channel. The goroutines exit when
// the requests channel is closed.
// The processResponse callback can be used to modify the response, e.g.
// to replace the body.
func MakeBatchRequests(
	ctx context.Context,
	requests []BatchParsedHTTPRequest,
	reqCount, globalLimit, perHostLimit int,
	processResponse func(context.Context, *Response, ResponseType),
) <-chan error {
	workers := globalLimit
	if reqCount < workers {
		workers = reqCount
	}
	result := make(chan error, reqCount)
	perHostLimiter := lib.NewMultiSlotLimiter(perHostLimit)

	makeRequest := func(req BatchParsedHTTPRequest) {
		if hl := perHostLimiter.Slot(req.URL.GetURL().Host); hl != nil {
			hl.Begin()
			defer hl.End()
		}

		resp, err := MakeRequest(ctx, req.ParsedHTTPRequest)
		if resp != nil {
			processResponse(ctx, resp, req.ParsedHTTPRequest.ResponseType)
			*req.Response = *resp
		}
		result <- err
	}

	counter, i32reqCount := int32(-1), int32(reqCount)
	for i := 0; i < workers; i++ {
		go func() {
			for {
				reqNum := atomic.AddInt32(&counter, 1)
				if reqNum >= i32reqCount {
					return
				}
				makeRequest(requests[reqNum])
			}
		}()
	}

	return result
}
