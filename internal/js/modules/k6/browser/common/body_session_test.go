package common

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

type bodySessionRecorder struct {
	session
	calls   int
	failure error
}

func (s *bodySessionRecorder) Execute(_ context.Context, method string, _, result any) error {
	s.calls++
	if s.failure != nil {
		return s.failure
	}
	if method == "Network.getResponseBody" {
		result.(*network.GetResponseBodyReturns).Body = "owned"
	}
	return nil
}

func TestResponseBodyOriginatingSession(t *testing.T) {
	t.Parallel()
	for _, swapped := range []bool{false, true} {
		name := "same session"
		if swapped {
			name = "different frame session"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			origin, foreign := &bodySessionRecorder{}, &bodySessionRecorder{failure: errors.New("No resource with given identifier found (-32000)")}
			frameSession := origin
			if swapped {
				frameSession = foreign
			}
			ts := cdp.MonotonicTime(time.Now())
			wt := cdp.TimeSinceEpoch(time.Now())
			req, err := NewRequest(t.Context(), log.NewNullLogger(), NewRequestParams{session: origin, frame: &Frame{manager: &FrameManager{session: frameSession}}, event: &network.EventRequestWillBeSent{RequestID: "owned-request", Request: &network.Request{URL: "http://localhost/payload", Method: "GET"}, Timestamp: &ts, WallTime: &wt}})
			require.NoError(t, err)
			response := &Response{ctx: t.Context(), request: req, status: 200, logger: log.NewNullLogger()}
			// Metric sizing uses the same fetch path as public body reads.
			require.Equal(t, int64(5), response.Size().Body)
			require.Equal(t, 1, origin.calls)
			require.Zero(t, foreign.calls)
			// Cached data must survive a later frame/session change without refetch.
			req.frame.manager.session = foreign
			req.session = nil
			body, err := response.Body()
			require.NoError(t, err)
			require.Equal(t, "owned", string(body))
			require.Equal(t, 1, origin.calls)
			require.Zero(t, foreign.calls)
		})
	}
}

func TestResponseBodyRedirectSessionUnchanged(t *testing.T) {
	t.Parallel()
	origin := &bodySessionRecorder{}
	response := &Response{ctx: t.Context(), request: &Request{frame: &Frame{}, session: origin}, status: 302, logger: log.NewNullLogger()}
	_, err := response.Body()
	require.ErrorContains(t, err, "unavailable for redirect")
	require.Zero(t, response.Size().Body)
	require.Zero(t, origin.calls)
}
