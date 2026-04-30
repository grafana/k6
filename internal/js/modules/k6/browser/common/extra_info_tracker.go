package common

import (
	"github.com/chromedp/cdproto/network"
)

// requestExtraInfo tracks the index-based pairing of extra info events
// and requests/responses for a single requestId.
//
// CDP dispatches events on two independent channels:
//   - Channel A: requestWillBeSent, responseReceived, loadingFinished/loadingFailed
//   - Channel B: requestWillBeSentExtraInfo, responseReceivedExtraInfo
//
// Events can arrive in any order between channels, but within each channel
// they arrive in order for a given requestId. In a redirect chain, a single
// requestId produces multiple request/response pairs. We pair them by index:
// the i-th extraInfo event corresponds to the i-th request/response since the
// ordering is guaranteed within each channel.
//
// Example:
// requestWillBeSentExtraInfo(123, hdrs) → pairs with Request_A at index 0
// responseReceivedExtraInfo(123, hdrs) → pairs with Response_0 at index 0
// requestWillBeSentExtraInfo(123, hdrs) → pairs with Request_B at index 1
// responseReceivedExtraInfo(123, hdrs) → pairs with Response_1 at index 1
// requestWillBeSentExtraInfo(123, hdrs) → pairs with Request_C at index 2
// responseReceivedExtraInfo(123, hdrs) → pairs with Response_2 at index 2
type requestExtraInfo struct {
	// Indexed queues of parsed extra headers. Slots are set to nil after
	// pairing to avoid applying headers twice.
	requestExtraHeaders  []map[string][]string
	responseExtraHeaders []map[string][]string

	// Indexed queues of requests/responses awaiting extra headers. Slots
	// are set to nil after pairing.
	requests  []*Request
	responses []*Response
}

// extraInfoTracker pairs requestWillBeSentExtraInfo / responseReceivedExtraInfo
// events with their corresponding Request / Response objects using index-based
// matching (modelled after Playwright's ResponseExtraInfoTracker).
type extraInfoTracker struct {
	requests map[network.RequestID]*requestExtraInfo
}

func newExtraInfoTracker() *extraInfoTracker {
	return &extraInfoTracker{
		requests: make(map[network.RequestID]*requestExtraInfo),
	}
}

// getOrCreate returns the requestExtraInfo for the given requestId,
// creating one if it doesn't exist yet.
func (t *extraInfoTracker) getOrCreate(reqID network.RequestID) *requestExtraInfo {
	info, ok := t.requests[reqID]
	if !ok {
		info = &requestExtraInfo{}
		t.requests[reqID] = info
	}
	return info
}

// requestWillBeSentExtraInfo records parsed request extra headers and
// tries to pair them with an already-registered Request at the same index.
func (t *extraInfoTracker) requestWillBeSentExtraInfo(reqID network.RequestID, headers map[string][]string) {
	info := t.getOrCreate(reqID)
	info.requestExtraHeaders = append(info.requestExtraHeaders, headers)
	t.patchRequestHeaders(info, len(info.requestExtraHeaders)-1)
}

// responseReceivedExtraInfo records parsed response extra headers and
// tries to pair them with an already-registered Response at the same index.
func (t *extraInfoTracker) responseReceivedExtraInfo(reqID network.RequestID, headers map[string][]string) {
	info := t.getOrCreate(reqID)
	info.responseExtraHeaders = append(info.responseExtraHeaders, headers)
	t.patchResponseHeaders(info, len(info.responseExtraHeaders)-1)
}

// processRequest registers a Request and tries to pair it with
// already-received request extra headers at the same index.
func (t *extraInfoTracker) processRequest(reqID network.RequestID, req *Request) {
	info := t.getOrCreate(reqID)
	info.requests = append(info.requests, req)
	t.patchRequestHeaders(info, len(info.requests)-1)
}

// processResponse registers a Response and tries to pair it with
// already-received response extra headers at the same index.
func (t *extraInfoTracker) processResponse(reqID network.RequestID, resp *Response) {
	info := t.getOrCreate(reqID)
	info.responses = append(info.responses, resp)
	t.patchResponseHeaders(info, len(info.responses)-1)
}

// patchRequestHeaders applies request extra headers at the given index
// if both the Request and the extra headers are available. After applying,
// both slots are nilled out to avoid double-application.
func (t *extraInfoTracker) patchRequestHeaders(info *requestExtraInfo, index int) {
	if index >= len(info.requests) || index >= len(info.requestExtraHeaders) {
		return
	}
	req := info.requests[index]
	extra := info.requestExtraHeaders[index]
	if req == nil || extra == nil {
		return
	}
	req.addExtraHeaders(extra)
	info.requests[index] = nil
	info.requestExtraHeaders[index] = nil
}

// patchResponseHeaders applies response extra headers at the given index
// if both the Response and the extra headers are available. After applying,
// both slots are nilled out to avoid double-application.
func (t *extraInfoTracker) patchResponseHeaders(info *requestExtraInfo, index int) {
	if index >= len(info.responses) || index >= len(info.responseExtraHeaders) {
		return
	}
	resp := info.responses[index]
	extra := info.responseExtraHeaders[index]
	if resp == nil || extra == nil {
		return
	}
	resp.addExtraHeaders(extra)
	info.responses[index] = nil
	info.responseExtraHeaders[index] = nil
}

// cleanup removes the tracking entry for the given requestId. Should be
// called when loading finishes or fails.
func (t *extraInfoTracker) cleanup(reqID network.RequestID) {
	delete(t.requests, reqID)
}
