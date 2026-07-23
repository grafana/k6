package common

import (
	"github.com/chromedp/cdproto/network"
)

// requestInfo aligns CDP ExtraInfo events with their Response objects for a
// single requestId, modelled after Playwright's ResponseExtraInfoTracker.
//
// CDP dispatches events on two independent channels:
//   - Channel A: requestWillBeSent, responseReceived, loadingFinished/loadingFailed
//   - Channel B: requestWillBeSentExtraInfo, responseReceivedExtraInfo
//
// They are not synchronised, so events arrive in any order between the
// channels, but within each channel they arrive in order for a given
// requestId. A single requestId can produce multiple request/response pairs
// (a redirect chain reuses the requestId), so we pair by index: the i-th
// tracked response pairs with the i-th request/response ExtraInfo event.
//
// Only responses that expect ExtraInfo are tracked (see processResponse).
// Because the ExtraInfo events are likewise only emitted for those responses,
// the indexes stay aligned even when some hops (e.g. cached redirects) carry no
// ExtraInfo. The raw request headers are attached to the response's Request,
// matching Playwright.
type requestInfo struct {
	// Indexed queues of parsed raw headers. Slots are set to nil after
	// pairing to avoid applying the same headers twice.
	requestExtraHeaders  []map[string][]string
	responseExtraHeaders []map[string][]string

	// Responses that expect ExtraInfo, in arrival order.
	responses []*Response

	loadingFinished bool
	loadingFailed   bool
	servedFromCache bool
}

// extraInfoTracker pairs requestWillBeSentExtraInfo / responseReceivedExtraInfo
// events with their corresponding Response (and its Request) using index-based
// matching (modelled after Playwright's ResponseExtraInfoTracker).
type extraInfoTracker struct {
	requests map[network.RequestID]*requestInfo
}

func newExtraInfoTracker() *extraInfoTracker {
	return &extraInfoTracker{
		requests: make(map[network.RequestID]*requestInfo),
	}
}

// getOrCreate returns the requestInfo for the given requestId, creating one if
// it doesn't exist yet.
func (t *extraInfoTracker) getOrCreate(reqID network.RequestID) *requestInfo {
	info, ok := t.requests[reqID]
	if !ok {
		info = &requestInfo{}
		t.requests[reqID] = info
	}
	return info
}

// requestWillBeSentExtraInfo records parsed raw request headers and tries to
// pair them with an already-registered response at the same index.
func (t *extraInfoTracker) requestWillBeSentExtraInfo(reqID network.RequestID, headers map[string][]string) {
	info := t.getOrCreate(reqID)
	info.requestExtraHeaders = append(info.requestExtraHeaders, headers)
	t.patchHeaders(info, len(info.requestExtraHeaders)-1)
	t.checkFinished(reqID, info)
}

// responseReceivedExtraInfo records parsed raw response headers and tries to
// pair them with an already-registered response at the same index.
func (t *extraInfoTracker) responseReceivedExtraInfo(reqID network.RequestID, headers map[string][]string) {
	info := t.getOrCreate(reqID)
	info.responseExtraHeaders = append(info.responseExtraHeaders, headers)
	t.patchHeaders(info, len(info.responseExtraHeaders)-1)
	t.checkFinished(reqID, info)
}

// requestServedFromCache marks the requestId as served from cache. Chrome can
// report hasExtraInfo=true for cached responses but never emit the matching
// ExtraInfo event (crbug.com/1340398), so such responses must not be tracked,
// otherwise the index would slip and later headers would land on the wrong
// response.
func (t *extraInfoTracker) requestServedFromCache(reqID network.RequestID) {
	info := t.getOrCreate(reqID)
	info.servedFromCache = true
}

// processResponse registers a Response that expects ExtraInfo. Responses that
// do not expect it (hasExtraInfo=false, or served from cache) are not tracked,
// so they keep their provisional headers as the raw ones.
func (t *extraInfoTracker) processResponse(reqID network.RequestID, resp *Response, hasExtraInfo bool) {
	info, ok := t.requests[reqID]
	if !hasExtraInfo || (ok && info.servedFromCache) {
		// No raw headers will arrive, so the provisional headers are final.
		// Unblock any readers waiting for the raw headers.
		resp.resolveRawHeaders()
		resp.request.resolveRawHeaders()
		return
	}
	if !ok {
		info = t.getOrCreate(reqID)
	}
	info.responses = append(info.responses, resp)
	t.patchHeaders(info, len(info.responses)-1)
}

// patchHeaders attaches the raw request and response headers at the given index
// when both the response and the corresponding ExtraInfo event are available.
// Consumed ExtraInfo slots are set to nil to avoid applying them twice.
func (t *extraInfoTracker) patchHeaders(info *requestInfo, index int) {
	if index < 0 || index >= len(info.responses) {
		return
	}
	resp := info.responses[index]
	if resp == nil {
		return
	}
	if index < len(info.requestExtraHeaders) {
		if extra := info.requestExtraHeaders[index]; extra != nil {
			resp.request.addExtraHeaders(extra)
			info.requestExtraHeaders[index] = nil
		}
	}
	if index < len(info.responseExtraHeaders) {
		if extra := info.responseExtraHeaders[index]; extra != nil {
			resp.addExtraHeaders(extra)
			info.responseExtraHeaders[index] = nil
		}
	}
}

// loadingFinished records that loading finished and stops tracking once every
// tracked response has been paired with its ExtraInfo event.
func (t *extraInfoTracker) loadingFinished(reqID network.RequestID) {
	info, ok := t.requests[reqID]
	if !ok {
		return
	}
	info.loadingFinished = true
	t.resolvePending(info)
	t.checkFinished(reqID, info)
}

// loadingFailed records that loading failed and stops tracking once every
// tracked response has been paired with its ExtraInfo event.
func (t *extraInfoTracker) loadingFailed(reqID network.RequestID) {
	info, ok := t.requests[reqID]
	if !ok {
		return
	}
	info.loadingFailed = true
	t.resolvePending(info)
	t.checkFinished(reqID, info)
}

// resolvePending unblocks readers waiting on the raw headers of any tracked
// response (and its request) whose ExtraInfo never arrived. It is a backstop
// run on the terminal event so a reader never waits forever; the entry itself
// is kept (see checkFinished) so a late ExtraInfo event can still be applied.
func (t *extraInfoTracker) resolvePending(info *requestInfo) {
	for _, resp := range info.responses {
		if resp == nil {
			continue
		}
		resp.resolveRawHeaders()
		resp.request.resolveRawHeaders()
	}
}

// checkFinished removes the tracking entry once loading has finished/failed and
// every tracked response has its response ExtraInfo. Deletion is deferred until
// then so a late-arriving ExtraInfo event (CDP may emit it after
// loadingFinished) can still be paired instead of being dropped.
func (t *extraInfoTracker) checkFinished(reqID network.RequestID, info *requestInfo) {
	if !info.loadingFinished && !info.loadingFailed {
		return
	}
	if len(info.responses) <= len(info.responseExtraHeaders) {
		delete(t.requests, reqID)
	}
}
