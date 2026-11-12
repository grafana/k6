package httpext

import (
	"context"
	"fmt"
	"sync/atomic"

	"go.k6.io/k6/v2/lib"
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
//
// If globalLimit is non-positive and there are requests to run, no workers would
// be started and callers waiting on the errors channel would hang forever.
// That case is rejected with a buffered error per request instead.
func MakeBatchRequests(
	ctx context.Context, state *lib.State,
	requests []BatchParsedHTTPRequest,
	reqCount, globalLimit, perHostLimit int,
) <-chan error {
	result := make(chan error, reqCount)
	if reqCount > 0 && globalLimit < 1 {
		err := fmt.Errorf("batch concurrency must be a positive number but is %d", globalLimit)
		for range reqCount {
			result <- err
		}
		return result
	}

	workers := min(reqCount, globalLimit)
	perHostLimiter := lib.NewMultiSlotLimiter(perHostLimit)

	makeRequest := func(req BatchParsedHTTPRequest) {
		if hl := perHostLimiter.Slot(req.URL.GetURL().Host); hl != nil {
			hl.Begin()
			defer hl.End()
		}

		resp, err := MakeRequest(ctx, state, req.ParsedHTTPRequest)
		if resp != nil {
			*req.Response = *resp
		}
		result <- err
	}

	counter, i32reqCount := int32(-1), int32(reqCount) //nolint:gosec
	for range workers {
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
