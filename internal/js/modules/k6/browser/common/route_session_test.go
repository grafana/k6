package common

import (
	"context"
	"net/url"
	"testing"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/stretchr/testify/require"
	k6lib "go.k6.io/k6/v2/lib"
	k6types "go.k6.io/k6/v2/lib/types"
)

func TestRouteInterceptionMandatory(t *testing.T) {
	t.Parallel()
	for _, mandatory := range []string{"none", "auth", "blacklist", "hostname"} {
		t.Run(mandatory, func(t *testing.T) {
			t.Parallel()
			opts := k6lib.Options{}
			if mandatory == "blacklist" {
				opts.BlacklistIPs = []*k6lib.IPNet{{}}
			}
			if mandatory == "hostname" {
				var err error
				opts.BlockedHostnames, err = k6types.NewNullHostnameTrie([]string{"blocked.test"})
				require.NoError(t, err)
			}
			nm, s := newTestNetworkManager(t, opts)
			if mandatory == "auth" {
				require.NoError(t, nm.Authenticate(Credentials{Username: "user", Password: "pass"}))
			}
			require.NoError(t, nm.setRequestInterception(true))
			require.NoError(t, nm.setRequestInterception(false))
			require.Equal(t, mandatory != "none", nm.protocolReqInterceptionEnabled)
			if mandatory != "none" {
				require.NotContains(t, s.cdpCalls, "Fetch.disable")
			}
			if mandatory == "auth" {
				require.NoError(t, nm.Authenticate(Credentials{}))
				require.False(t, nm.protocolReqInterceptionEnabled)
			}
		})
	}
}

func TestPageRoutesExistingAndFutureSessions(t *testing.T) {
	t.Parallel()
	first, s1 := newTestNetworkManager(t, k6lib.Options{})
	second, s2 := newTestNetworkManager(t, k6lib.Options{})
	p := &Page{session: s1, logger: first.logger, frameSessions: map[cdp.FrameID]*FrameSession{}}
	makeFrame := func(nm *NetworkManager) *FrameSession {
		return &FrameSession{ctx: t.Context(), session: nm.session, logger: nm.logger, networkManager: nm, page: p}
	}
	p.mainFrameSession = makeFrame(first)
	p.frameSessions["main"] = p.mainFrameSession
	p.frameSessions["existing"] = makeFrame(second)
	require.NoError(t, p.Route("**/*", func(*Route) error { return nil }, func(string, string) (bool, error) { return true, nil }))
	require.True(t, first.protocolReqInterceptionEnabled)
	require.True(t, second.protocolReqInterceptionEnabled)
	third, s3 := newTestNetworkManager(t, k6lib.Options{})
	require.NoError(t, p.attachFrameSession("future", makeFrame(third)))
	require.True(t, third.protocolReqInterceptionEnabled)
	require.NoError(t, p.Unroute("**/*"))
	for _, s := range []*fakeSession{s1, s2, s3} {
		require.Contains(t, s.cdpCalls, "Fetch.enable")
		require.Contains(t, s.cdpCalls, "Fetch.disable")
	}
	require.NoError(t, p.Route("**/*", func(*Route) error { return nil }, func(string, string) (bool, error) { return true, nil }))
	require.NoError(t, p.UnrouteAll())
	require.False(t, third.protocolReqInterceptionEnabled)
}

func TestRequestRouteOriginAndMissingID(t *testing.T) {
	t.Parallel()
	for _, intercepted := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "owned"}[intercepted], func(t *testing.T) {
			t.Parallel()
			origin, s := newTestNetworkManager(t, k6lib.Options{})
			other, foreign := newTestNetworkManager(t, k6lib.Options{})
			called := 0
			p := &Page{routes: []*RouteHandler{{urlMatcher: func(string) (bool, error) { return true, nil }, handler: func(r *Route) error { called++; return r.Continue(ContinueOptions{}) }}}, mainFrameSession: &FrameSession{networkManager: other}}
			fm := origin.frameManager
			fm.page = p
			u, err := url.Parse("http://localhost/owned.js")
			require.NoError(t, err)
			req := &Request{ctx: t.Context(), url: u, frame: &Frame{log: origin.logger, inflightRequests: map[network.RequestID]bool{}}, requestID: "request"}
			if intercepted {
				req.interceptionID = "child-interception"
			}
			fm.requestStarted(req, origin)
			require.Equal(t, map[bool]int{false: 0, true: 1}[intercepted], called)
			require.Empty(t, foreign.cdpCalls)
			if intercepted {
				require.Equal(t, []string{"Fetch.continueRequest"}, s.cdpCalls)
			} else {
				require.Empty(t, s.cdpCalls)
			}
		})
	}
}

func TestPageRoutesSkipClosedSession(t *testing.T) {
	t.Parallel()
	live, s := newTestNetworkManager(t, k6lib.Options{})
	dead, closed := newTestNetworkManager(t, k6lib.Options{})
	done := make(chan struct{})
	close(done)
	closed.session = &Session{done: done}
	p := &Page{session: s, logger: live.logger, frameSessions: map[cdp.FrameID]*FrameSession{
		"live": {session: s, logger: live.logger, networkManager: live},
		"dead": {session: closed, logger: dead.logger, networkManager: dead},
	}}
	p.mainFrameSession = p.frameSessions["live"]
	require.NoError(t, p.Route("**/*", func(*Route) error { return nil }, func(string, string) (bool, error) { return true, nil }))
	require.NoError(t, p.UnrouteAll())
	require.Empty(t, closed.cdpCalls)
	require.Equal(t, []string{"Network.setCacheDisabled", "Fetch.enable", "Network.setCacheDisabled", "Fetch.disable"}, s.cdpCalls)
}

type routeCrashSession struct{ *fakeSession }

func (s *routeCrashSession) Execute(context.Context, string, any, any) error { return ErrTargetCrashed }

func TestPageRoutesCrashedChildAndMain(t *testing.T) {
	t.Parallel()
	for _, mainCrashes := range []bool{false, true} {
		t.Run(map[bool]string{false: "child", true: "main"}[mainCrashes], func(t *testing.T) {
			t.Parallel()
			live, s := newTestNetworkManager(t, k6lib.Options{})
			crashed, c := newTestNetworkManager(t, k6lib.Options{})
			bad := &routeCrashSession{c}
			crashed.session = bad
			goodFS := &FrameSession{session: s, logger: live.logger, networkManager: live}
			badFS := &FrameSession{session: bad, logger: crashed.logger, networkManager: crashed}
			p := &Page{session: s, logger: live.logger, mainFrameSession: goodFS, frameSessions: map[cdp.FrameID]*FrameSession{"live": goodFS, "crashed": badFS}}
			if mainCrashes {
				p.mainFrameSession = badFS
			}
			err := p.Route("**/*", func(*Route) error { return nil }, func(string, string) (bool, error) { return true, nil })
			if mainCrashes {
				require.ErrorIs(t, err, ErrTargetCrashed)
			} else {
				require.NoError(t, err)
				require.True(t, live.protocolReqInterceptionEnabled)
				require.NoError(t, p.UnrouteAll())
				require.False(t, live.protocolReqInterceptionEnabled)
			}
		})
	}
}
