package common

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/internal/js/modules/k6/browser/log"

	k6mockresolver "go.k6.io/k6/internal/lib/testutils/mockresolver"
	k6lib "go.k6.io/k6/lib"
	k6types "go.k6.io/k6/lib/types"
	k6metrics "go.k6.io/k6/metrics"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/mailru/easyjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mockHostname = "host.test"

type fakeSession struct {
	session
	cdpCalls []string
}

// Execute implements the cdp.Executor interface to record calls made to it and
// allow assertions in tests.
func (s *fakeSession) Execute(
	ctx context.Context, method string, params easyjson.Marshaler, res easyjson.Unmarshaler,
) error {
	s.cdpCalls = append(s.cdpCalls, method)
	return nil
}

func newTestNetworkManager(t *testing.T, k6opts k6lib.Options) (*NetworkManager, *fakeSession) {
	t.Helper()

	session := &fakeSession{
		session: &Session{
			id: "1234",
		},
	}

	mr := k6mockresolver.New(map[string][]net.IP{
		mockHostname: {
			net.ParseIP("127.0.0.10"),
			net.ParseIP("127.0.0.11"),
			net.ParseIP("127.0.0.12"),
			net.ParseIP("2001:db8::10"),
			net.ParseIP("2001:db8::11"),
			net.ParseIP("2001:db8::12"),
		},
	})

	vu := k6test.NewVU(t)
	vu.ActivateVU()
	st := vu.State()
	st.Options = k6opts
	logger := log.New(st.Logger, "")
	nm := &NetworkManager{
		ctx:            vu.Context(),
		logger:         logger,
		session:        session,
		resolver:       mr,
		vu:             vu,
		reqIDToRequest: map[network.RequestID]*Request{},
	}

	return nm, session
}

func TestOnRequestPausedBlockedHostnames(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name, reqURL                  string
		blockedHostnames, expCDPCalls []string
	}{
		{
			name:             "ok_fail_simple",
			blockedHostnames: []string{"*.test"},
			reqURL:           fmt.Sprintf("http://%s/", mockHostname),
			expCDPCalls:      []string{"Fetch.failRequest"},
		},
		{
			name:             "ok_continue_simple",
			blockedHostnames: []string{"*.test"},
			reqURL:           "http://host.com/",
			expCDPCalls:      []string{"Fetch.continueRequest"},
		},
		{
			name:             "ok_continue_empty",
			blockedHostnames: nil,
			reqURL:           "http://host.com/",
			expCDPCalls:      []string{"Fetch.continueRequest"},
		},
		{
			name:             "ok_continue_ip",
			blockedHostnames: []string{"*.test"},
			reqURL:           "http://127.0.0.1:8000/",
			expCDPCalls:      []string{"Fetch.continueRequest"},
		},
		{
			name:             "err_url_continue",
			blockedHostnames: []string{"*.test"},
			reqURL:           ":::",
			expCDPCalls:      []string{"Fetch.continueRequest"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			blocked, err := k6types.NewNullHostnameTrie(tc.blockedHostnames)
			require.NoError(t, err)

			k6opts := k6lib.Options{BlockedHostnames: blocked}
			nm, session := newTestNetworkManager(t, k6opts)
			ev := &fetch.EventRequestPaused{
				RequestID: "1234",
				Request: &network.Request{
					Method: "GET",
					URL:    tc.reqURL,
				},
			}

			nm.onRequestPaused(ev)

			assert.Equal(t, tc.expCDPCalls, session.cdpCalls)
		})
	}
}

func TestOnRequestPausedBlockedIPs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name, reqURL            string
		blockedIPs, expCDPCalls []string
	}{
		{
			name:        "ok_fail_simple",
			blockedIPs:  []string{"10.0.0.0/8", "192.168.0.0/16"},
			reqURL:      "http://10.0.0.1:8000/",
			expCDPCalls: []string{"Fetch.failRequest"},
		},
		{
			name:        "ok_fail_resolved_ip",
			blockedIPs:  []string{"127.0.0.10/32"},
			reqURL:      fmt.Sprintf("http://%s/", mockHostname),
			expCDPCalls: []string{"Fetch.failRequest"},
		},
		{
			name:        "ok_continue_resolved_ip",
			blockedIPs:  []string{"127.0.0.50/32"},
			reqURL:      fmt.Sprintf("http://%s/", mockHostname),
			expCDPCalls: []string{"Fetch.continueRequest"},
		},
		{
			name:        "ok_continue_simple",
			blockedIPs:  []string{"127.0.0.0/8"},
			reqURL:      "http://10.0.0.1:8000/",
			expCDPCalls: []string{"Fetch.continueRequest"},
		},
		{
			name:        "ok_continue_empty",
			blockedIPs:  nil,
			reqURL:      "http://127.0.0.1/",
			expCDPCalls: []string{"Fetch.continueRequest"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			blockedIPs := make([]*k6lib.IPNet, len(tc.blockedIPs))
			for i, ipcidr := range tc.blockedIPs {
				ipnet, err := k6lib.ParseCIDR(ipcidr)
				require.NoError(t, err)
				blockedIPs[i] = ipnet
			}

			k6opts := k6lib.Options{BlacklistIPs: blockedIPs}
			nm, session := newTestNetworkManager(t, k6opts)
			ev := &fetch.EventRequestPaused{
				RequestID: "1234",
				Request: &network.Request{
					Method: "GET",
					URL:    tc.reqURL,
				},
			}

			nm.onRequestPaused(ev)

			assert.Equal(t, tc.expCDPCalls, session.cdpCalls)
		})
	}
}

type EventInterceptorMock struct{}

func (m *EventInterceptorMock) urlTagName(_ string, _ string) (string, bool) {
	return "", false
}

func (m *EventInterceptorMock) onRequest(_ *Request) {}

func (m *EventInterceptorMock) onResponse(_ *Response) {}

func TestNetworkManagerEmitRequestResponseMetricsTimingSkew(t *testing.T) {
	t.Parallel()

	now := time.Now()
	type tm struct{ ts, wt time.Time }
	tests := []struct {
		name                       string
		req, res, wantReq, wantRes tm
	}{
		{
			name:    "ok",
			req:     tm{ts: now, wt: now},
			res:     tm{ts: now, wt: now},
			wantReq: tm{wt: now},
			wantRes: tm{wt: now},
		},
		{
			name:    "ok2",
			req:     tm{ts: now, wt: now},
			res:     tm{ts: now.Add(time.Minute)},
			wantReq: tm{wt: now},
			wantRes: tm{wt: now.Add(time.Minute)},
		},
		{
			name:    "ts_past",
			req:     tm{ts: now.Add(-time.Hour), wt: now},
			res:     tm{ts: now.Add(-time.Hour).Add(time.Minute)},
			wantReq: tm{wt: now},
			wantRes: tm{wt: now.Add(time.Minute)},
		},
		{
			name:    "ts_future",
			req:     tm{ts: now.Add(time.Hour), wt: now},
			res:     tm{ts: now.Add(time.Hour).Add(time.Minute)},
			wantReq: tm{wt: now},
			wantRes: tm{wt: now.Add(time.Minute)},
		},
		{
			name:    "wt_past",
			req:     tm{ts: now, wt: now.Add(-time.Hour)},
			res:     tm{ts: now.Add(time.Minute)},
			wantReq: tm{wt: now.Add(-time.Hour)},
			wantRes: tm{wt: now.Add(-time.Hour).Add(time.Minute)},
		},
		{
			name:    "wt_future",
			req:     tm{ts: now, wt: now.Add(time.Hour)},
			res:     tm{ts: now.Add(time.Minute)},
			wantReq: tm{wt: now.Add(time.Hour)},
			wantRes: tm{wt: now.Add(time.Hour).Add(time.Minute)},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			registry := k6metrics.NewRegistry()
			k6m := k6ext.RegisterCustomMetrics(registry)

			var (
				vu = k6test.NewVU(t)
				nm = &NetworkManager{ctx: vu.Context(), vu: vu, customMetrics: k6m, eventInterceptor: &EventInterceptorMock{}}
			)
			vu.ActivateVU()

			req, err := NewRequest(vu.Context(), log.NewNullLogger(), NewRequestParams{
				event: &network.EventRequestWillBeSent{
					Request:   &network.Request{},
					Timestamp: (*cdp.MonotonicTime)(&tt.req.ts),
					WallTime:  (*cdp.TimeSinceEpoch)(&tt.req.wt),
				},
			})
			require.NoError(t, err)
			nm.emitRequestMetrics(req)
			n := vu.AssertSamples(func(s k6metrics.Sample) {
				assert.Equalf(t, tt.wantReq.wt, s.Time, "timing skew in %s", s.Metric.Name)
			})
			assert.Equalf(t, 1, n, "should emit %d request metric", 1)
			res := NewHTTPResponse(vu.Context(), req,
				&network.Response{Timing: &network.ResourceTiming{}},
				(*cdp.MonotonicTime)(&tt.res.ts),
			)
			nm.emitResponseMetrics(res, req)
			n = vu.AssertSamples(func(s k6metrics.Sample) {
				assert.Equalf(t, tt.wantRes.wt, s.Time, "timing skew in %s", s.Metric.Name)
			})
			assert.Equalf(t, 3, n, "should emit 8 response metrics")
		})
	}
}

func TestRequestForOnLoadingFinished(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		ID      network.RequestID
		request *Request
		parent  *Request
		want    *Request
	}{
		"non_nil_request": {
			ID: "1234",
			request: &Request{
				requestID:  "1234",
				documentID: "1234",
			},
			want: &Request{
				requestID:  "1234",
				documentID: "1234",
			},
		},
		"non_nil_request_with_parent": {
			ID: "1234",
			request: &Request{
				requestID:  "1234",
				documentID: "1234",
			},
			parent: &Request{
				documentID: "3421",
			},
			want: &Request{
				requestID:  "1234",
				documentID: "1234",
			},
		},
		"nil_request": {
			request: nil,
			want:    nil,
		},
		"nil_request_with_non_nil_parent_with_matching_document_id": {
			ID:      "1234",
			request: nil,
			parent: &Request{
				requestID:  "1234",
				documentID: "1234",
			},
			want: &Request{
				requestID:  "1234",
				documentID: "1234",
			},
		},
		"nil_request_with_non_nil_parent_with_non_matching_document_id": {
			ID:      "1234",
			request: nil,
			parent: &Request{
				requestID:  "1234",
				documentID: "4321",
			},
			want: nil,
		},
	}
	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			nm, _ := newTestNetworkManager(t, k6lib.Options{})
			nm.parent, _ = newTestNetworkManager(t, k6lib.Options{})

			if tt.request != nil {
				tt.request.requestID = tt.ID
				nm.reqIDToRequest[tt.ID] = tt.request
			}
			if tt.parent != nil {
				nm.parent.reqIDToRequest[tt.parent.requestID] = tt.parent
			}

			r := nm.requestForOnLoadingFinished(tt.ID)
			if tt.want == nil {
				require.Nil(t, r)
				return
			}
			require.NotNil(t, r)
			assert.Equal(t, tt.want, r)
		})
	}
}
