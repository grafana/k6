package common

import (
	"context"
	"testing"

	"github.com/chromedp/cdproto/network"
	"github.com/stretchr/testify/assert"
)

// The extraInfoTracker pairs CDP ExtraInfo events (channel B) with the
// responses they belong to (channel A) for a single requestId. The tests below
// walk the tracker through the event orderings CDP can legitimately produce,
// since the two channels are not synchronised. All of them are pure unit
// tests: no browser, no CDP connection.

// newTrackedRequestResponse builds the minimal Request/Response pair the
// tracker operates on, without going through the full constructors (which
// need a VU context and CDP events).
func newTrackedRequestResponse(ctx context.Context) (*Request, *Response) {
	req := &Request{
		ctx:          ctx,
		rawHeadersCh: make(chan struct{}),
	}
	resp := &Response{
		ctx:          ctx,
		request:      req,
		rawHeadersCh: make(chan struct{}),
	}
	return req, resp
}

// rawHeadersResolved reports whether the raw-headers channel of a
// Request/Response has been closed, without blocking.
func rawHeadersResolved(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func reqRawHeaders(r *Request) map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.extraHeaders
}

func respRawHeaders(r *Response) map[string][]string {
	r.extraHeadersMu.RLock()
	defer r.extraHeadersMu.RUnlock()
	return r.extraHeaders
}

func TestExtraInfoTracker_PairsWhenResponseArrivesFirst(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tracker := newExtraInfoTracker()
	reqID := network.RequestID("req-1")
	req, resp := newTrackedRequestResponse(ctx)

	// Channel A first: the response is registered before any ExtraInfo event.
	tracker.processResponse(reqID, resp, true)

	// Nothing is applied yet, and readers are still blocked.
	assert.Nil(t, reqRawHeaders(req))
	assert.Nil(t, respRawHeaders(resp))
	assert.False(t, rawHeadersResolved(req.rawHeadersCh))
	assert.False(t, rawHeadersResolved(resp.rawHeadersCh))

	// The request ExtraInfo arrives and is applied immediately (its pairing
	// partner, the response, already exists at the same index).
	reqHeaders := map[string][]string{"Cookie": {"session=1"}}
	tracker.requestWillBeSentExtraInfo(reqID, reqHeaders)
	assert.Equal(t, reqHeaders, reqRawHeaders(req))
	assert.True(t, rawHeadersResolved(req.rawHeadersCh))
	// ... but the response side is still waiting for its own event.
	assert.Nil(t, respRawHeaders(resp))
	assert.False(t, rawHeadersResolved(resp.rawHeadersCh))

	// The response ExtraInfo completes the pair.
	respHeaders := map[string][]string{"Set-Cookie": {"token=abc"}}
	tracker.responseReceivedExtraInfo(reqID, respHeaders)
	assert.Equal(t, respHeaders, respRawHeaders(resp))
	assert.True(t, rawHeadersResolved(resp.rawHeadersCh))
}

func TestExtraInfoTracker_PairsWhenExtraInfoArrivesFirst(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tracker := newExtraInfoTracker()
	reqID := network.RequestID("req-1")
	req, resp := newTrackedRequestResponse(ctx)

	// Channel B first: both ExtraInfo events are queued before the response
	// is even registered.
	reqHeaders := map[string][]string{"Authorization": {"Bearer tok"}}
	respHeaders := map[string][]string{"Content-Type": {"text/html"}}
	tracker.requestWillBeSentExtraInfo(reqID, reqHeaders)
	tracker.responseReceivedExtraInfo(reqID, respHeaders)
	assert.Nil(t, reqRawHeaders(req))
	assert.Nil(t, respRawHeaders(resp))

	// Registering the response applies both queued header sets at once.
	tracker.processResponse(reqID, resp, true)
	assert.Equal(t, reqHeaders, reqRawHeaders(req))
	assert.Equal(t, respHeaders, respRawHeaders(resp))
	assert.True(t, rawHeadersResolved(req.rawHeadersCh))
	assert.True(t, rawHeadersResolved(resp.rawHeadersCh))
}

func TestExtraInfoTracker_PairsWhenOnlyResponseExtraInfoIsQueued(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tracker := newExtraInfoTracker()
	reqID := network.RequestID("req-1")
	req, resp := newTrackedRequestResponse(ctx)

	// A queued response ExtraInfo without its request counterpart.
	respHeaders := map[string][]string{"Location": {"/moved"}}
	tracker.responseReceivedExtraInfo(reqID, respHeaders)

	tracker.processResponse(reqID, resp, true)

	assert.Equal(t, respHeaders, respRawHeaders(resp))
	assert.True(t, rawHeadersResolved(resp.rawHeadersCh))
	// No request ExtraInfo was seen, so the request keeps waiting for one
	// (until a terminal event resolves it as a backstop).
	assert.Nil(t, reqRawHeaders(req))
	assert.False(t, rawHeadersResolved(req.rawHeadersCh))
}

func TestExtraInfoTracker_ResponseWithoutExtraInfoResolvesImmediately(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tracker := newExtraInfoTracker()
	reqID := network.RequestID("req-1")
	req, resp := newTrackedRequestResponse(ctx)

	// hasExtraInfo=false: no raw headers will ever arrive, so the provisional
	// headers are final and readers must be unblocked right away.
	tracker.processResponse(reqID, resp, false)

	assert.True(t, rawHeadersResolved(req.rawHeadersCh))
	assert.True(t, rawHeadersResolved(resp.rawHeadersCh))
	assert.Nil(t, reqRawHeaders(req))
	assert.Nil(t, respRawHeaders(resp))
	// Nothing is tracked for this requestId.
	assert.NotContains(t, tracker.requests, reqID)
}

func TestExtraInfoTracker_ServedFromCacheIsNotTracked(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tracker := newExtraInfoTracker()
	reqID := network.RequestID("req-1")
	req, resp := newTrackedRequestResponse(ctx)

	// Chrome can report hasExtraInfo=true for cached responses but never emit
	// the ExtraInfo events (crbug.com/1340398). Such responses must not be
	// tracked, otherwise the index would slip for later hops.
	tracker.requestServedFromCache(reqID)
	tracker.processResponse(reqID, resp, true)

	assert.True(t, rawHeadersResolved(req.rawHeadersCh))
	assert.True(t, rawHeadersResolved(resp.rawHeadersCh))

	// Even if stray ExtraInfo events show up for this requestId, they must
	// not leak headers onto the cached response.
	tracker.requestWillBeSentExtraInfo(reqID, map[string][]string{"Cookie": {"a=1"}})
	tracker.responseReceivedExtraInfo(reqID, map[string][]string{"X-Stray": {"yes"}})
	assert.Nil(t, reqRawHeaders(req))
	assert.Nil(t, respRawHeaders(resp))
}

func TestExtraInfoTracker_RedirectChainPairsByIndex(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tracker := newExtraInfoTracker()
	reqID := network.RequestID("req-redirect-chain")

	// A redirect chain reuses one requestId for multiple request/response
	// pairs; the i-th tracked response pairs with the i-th ExtraInfo events.
	hop1Req, hop1Resp := newTrackedRequestResponse(ctx)
	hop2Req, hop2Resp := newTrackedRequestResponse(ctx)

	tracker.processResponse(reqID, hop1Resp, true)
	tracker.processResponse(reqID, hop2Resp, true)

	// The ExtraInfo events arrive in hop order but after both responses.
	tracker.requestWillBeSentExtraInfo(reqID, map[string][]string{"Cookie": {"hop=1"}})
	tracker.responseReceivedExtraInfo(reqID, map[string][]string{"Location": {"/hop2"}})
	tracker.requestWillBeSentExtraInfo(reqID, map[string][]string{"Cookie": {"hop=2"}})
	tracker.responseReceivedExtraInfo(reqID, map[string][]string{"Location": {"/final"}})

	assert.Equal(t, map[string][]string{"Cookie": {"hop=1"}}, reqRawHeaders(hop1Req))
	assert.Equal(t, map[string][]string{"Location": {"/hop2"}}, respRawHeaders(hop1Resp))
	assert.Equal(t, map[string][]string{"Cookie": {"hop=2"}}, reqRawHeaders(hop2Req))
	assert.Equal(t, map[string][]string{"Location": {"/final"}}, respRawHeaders(hop2Resp))

	// Each hop's raw headers are resolved once both of its events landed.
	for _, ch := range []chan struct{}{
		hop1Req.rawHeadersCh, hop1Resp.rawHeadersCh, hop2Req.rawHeadersCh, hop2Resp.rawHeadersCh,
	} {
		assert.True(t, rawHeadersResolved(ch))
	}
}

func TestExtraInfoTracker_LoadingFinishedRemovesFullyPairedEntry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tracker := newExtraInfoTracker()
	reqID := network.RequestID("req-1")
	_, resp := newTrackedRequestResponse(ctx)

	tracker.processResponse(reqID, resp, true)
	tracker.requestWillBeSentExtraInfo(reqID, map[string][]string{"Cookie": {"a=1"}})
	tracker.responseReceivedExtraInfo(reqID, map[string][]string{"X-Resp": {"1"}})

	// Every tracked response has its response ExtraInfo, so the terminal
	// event can drop the tracking entry.
	tracker.loadingFinished(reqID)
	assert.NotContains(t, tracker.requests, reqID)
}

func TestExtraInfoTracker_LoadingFinishedKeepsEntryForLateExtraInfo(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tracker := newExtraInfoTracker()
	reqID := network.RequestID("req-1")
	req, resp := newTrackedRequestResponse(ctx)

	tracker.processResponse(reqID, resp, true)
	tracker.requestWillBeSentExtraInfo(reqID, map[string][]string{"Cookie": {"a=1"}})

	// The terminal event arrives while the response ExtraInfo is still
	// missing. Readers must be unblocked (backstop)...
	tracker.loadingFinished(reqID)
	assert.True(t, rawHeadersResolved(req.rawHeadersCh))
	assert.True(t, rawHeadersResolved(resp.rawHeadersCh))
	// ... but the entry itself is kept so a late ExtraInfo can still pair.
	assert.Contains(t, tracker.requests, reqID)

	// CDP may emit the response ExtraInfo after loadingFinished; it must be
	// applied to the retained response, and only then is the entry dropped.
	lateHeaders := map[string][]string{"X-Late": {"true"}}
	tracker.responseReceivedExtraInfo(reqID, lateHeaders)
	assert.Equal(t, lateHeaders, respRawHeaders(resp))
	assert.NotContains(t, tracker.requests, reqID)
}

func TestExtraInfoTracker_LoadingFailedBehavesLikeLoadingFinished(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tracker := newExtraInfoTracker()
	reqID := network.RequestID("req-1")
	_, resp := newTrackedRequestResponse(ctx)

	// A fully paired response: loadingFailed drops the entry, exactly like
	// loadingFinished.
	tracker.processResponse(reqID, resp, true)
	tracker.responseReceivedExtraInfo(reqID, map[string][]string{"X-Resp": {"1"}})
	tracker.loadingFailed(reqID)
	assert.NotContains(t, tracker.requests, reqID)

	// A pending one: the entry survives for a late ExtraInfo event.
	_, pending := newTrackedRequestResponse(ctx)
	tracker.processResponse(reqID, pending, true)
	tracker.loadingFailed(reqID)
	assert.Contains(t, tracker.requests, reqID)
	lateHeaders := map[string][]string{"X-Late": {"failed"}}
	tracker.responseReceivedExtraInfo(reqID, lateHeaders)
	assert.Equal(t, lateHeaders, respRawHeaders(pending))
	assert.NotContains(t, tracker.requests, reqID)
}

func TestExtraInfoTracker_TerminalEventForUnknownRequestIsNoOp(t *testing.T) {
	t.Parallel()

	tracker := newExtraInfoTracker()

	// Neither terminal event may create an entry or panic for an unknown
	// requestId (e.g. events for requests that started before k6 attached).
	tracker.loadingFinished(network.RequestID("unknown"))
	tracker.loadingFailed(network.RequestID("unknown"))

	assert.Empty(t, tracker.requests)
}

func TestExtraInfoTracker_RequestIDsAreIndependent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tracker := newExtraInfoTracker()
	reqID1 := network.RequestID("req-1")
	reqID2 := network.RequestID("req-2")
	req1, resp1 := newTrackedRequestResponse(ctx)
	req2, resp2 := newTrackedRequestResponse(ctx)

	// Interleaved traffic: the second request completes while the first is
	// still missing its response ExtraInfo.
	tracker.processResponse(reqID1, resp1, true)
	tracker.processResponse(reqID2, resp2, true)
	tracker.requestWillBeSentExtraInfo(reqID1, map[string][]string{"Cookie": {"first=1"}})
	tracker.responseReceivedExtraInfo(reqID2, map[string][]string{"X-Second": {"1"}})

	assert.Equal(t, map[string][]string{"Cookie": {"first=1"}}, reqRawHeaders(req1))
	assert.Nil(t, respRawHeaders(resp1)) // still waiting
	assert.Nil(t, reqRawHeaders(req2))   // no request ExtraInfo seen for req-2
	assert.Equal(t, map[string][]string{"X-Second": {"1"}}, respRawHeaders(resp2))

	// Terminal handling is per-requestId as well.
	tracker.loadingFinished(reqID2)
	assert.NotContains(t, tracker.requests, reqID2)
	assert.Contains(t, tracker.requests, reqID1)
}
