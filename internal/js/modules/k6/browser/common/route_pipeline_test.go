package common

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"testing/synctest"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	k6lib "go.k6.io/k6/v2/lib"
)

type pipelineSession struct {
	session
	entered chan fetch.RequestID
	release chan struct{}
	failure error
}

func (s *pipelineSession) Execute(ctx context.Context, method string, params, _ any) error {
	if method != "Fetch.continueRequest" {
		return nil
	}
	s.entered <- params.(*fetch.ContinueRequestParams).RequestID
	select {
	case <-s.release:
		return s.failure
	case <-ctx.Done():
		return ctx.Err()
	}
}

func pipelineRequest(t *testing.T, nm *NetworkManager, id string) *Request {
	t.Helper()
	u, err := url.Parse("http://localhost/" + id)
	require.NoError(t, err)
	f := &Frame{log: nm.logger, inflightRequests: map[network.RequestID]bool{}}
	return &Request{ctx: nm.ctx, url: u, frame: f, requestID: network.RequestID(id), interceptionID: fetch.RequestID(id), documentID: id}
}

func TestRouteDefaultPipeline(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		nm, original := newTestNetworkManager(t, k6lib.Options{})
		ctx, cancel := context.WithCancel(nm.ctx)
		defer cancel()
		nm.ctx = ctx
		s := &pipelineSession{session: original, entered: make(chan fetch.RequestID, 2), release: make(chan struct{})}
		nm.session = s
		nm.frameManager.page = &Page{routes: []*RouteHandler{{urlMatcher: func(string) (bool, error) { return false, nil }}}}
		a, b := pipelineRequest(t, nm, "a"), pipelineRequest(t, nm, "b")
		dispatched := make(chan struct{})
		go func() {
			nm.frameManager.requestStarted(a, nm)
			nm.frameManager.requestStarted(b, nm)
			close(dispatched)
		}()
		synctest.Wait()
		select {
		case <-dispatched:
			require.Len(t, s.entered, 2, "both independent requests must reach CDP before either ACK")
			require.True(t, a.frame.inflightRequests[a.requestID])
			require.Same(t, a, a.frame.pendingDocument.request)
			require.True(t, b.frame.inflightRequests[b.requestID])
		default:
			t.Error("request dispatch blocked on the first Continue ACK")
		}
		joined := make(chan struct{})
		go func() { nm.wait(); close(joined) }()
		synctest.Wait()
		select {
		case <-joined:
			t.Error("manager wait returned before outstanding Continue completed")
		default:
		}
		// Page.Close cancels its managers before waiting for their tracked work.
		cancel()
		synctest.Wait()
		<-dispatched
		<-joined
	})
}

func TestRouteCallbackFrames(t *testing.T) {
	t.Parallel()
	nm, _ := newTestNetworkManager(t, k6lib.Options{})
	p := &Page{}
	fm := nm.frameManager
	fm.page = p
	called := false
	p.routes = []*RouteHandler{{
		urlMatcher: func(string) (bool, error) { return true, nil },
		handler: func(*Route) error {
			// This is the same synchronous lock path as JS page.frames().
			require.Empty(t, fm.Frames())
			// Matched callbacks are not tracked jobs themselves: joining other work does not self-wait.
			nm.wait()
			// Route callbacks may change registration, without affecting this request's snapshot.
			p.routesMu.Lock()
			p.routes = nil
			p.routesMu.Unlock()
			called = true
			return nil
		},
	}}
	fm.frames = map[cdp.FrameID]*Frame{}
	fm.requestStarted(pipelineRequest(t, nm, "callback"), nm)
	require.True(t, called)
	nm.wait()
}

func TestRouteDefaultError(t *testing.T) {
	t.Parallel()
	nm, original := newTestNetworkManager(t, k6lib.Options{})
	hook := logtest.NewLocal(nm.logger.Logger)
	released := make(chan struct{})
	close(released)
	nm.session = &pipelineSession{session: original, entered: make(chan fetch.RequestID, 1), release: released, failure: errors.New("fixture continuation error")}
	nm.frameManager.page = &Page{routes: []*RouteHandler{{urlMatcher: func(string) (bool, error) { return false, nil }}}}
	nm.frameManager.requestStarted(pipelineRequest(t, nm, "error"), nm)
	nm.wait()
	require.NotNil(t, hook.LastEntry())
	require.Contains(t, hook.LastEntry().Message, "error continuing request")
	require.Contains(t, hook.LastEntry().Message, "fixture continuation error")
}
