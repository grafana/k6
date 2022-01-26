package common

import (
	"context"
	"testing"

	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/mailru/easyjson"
	"github.com/oxtoacart/bpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k6lib "go.k6.io/k6/lib"
	k6metrics "go.k6.io/k6/lib/metrics"
	k6test "go.k6.io/k6/lib/testutils"
	k6types "go.k6.io/k6/lib/types"
	k6stats "go.k6.io/k6/stats"
)

type testSession struct {
	*Session
	cdpCalls []string
}

// Execute implements the cdp.Executor interface to record calls made to it and
// allow assertions in tests.
func (s *testSession) Execute(ctx context.Context, method string,
	params easyjson.Marshaler, res easyjson.Unmarshaler) error {
	s.cdpCalls = append(s.cdpCalls, method)
	return nil
}

func newTestNetworkManager(t *testing.T, k6opts k6lib.Options) (*NetworkManager, *testSession) {
	ctx := context.Background()

	root, err := k6lib.NewGroup("", nil)
	require.NoError(t, err)

	state := &k6lib.State{
		Options:        k6opts,
		Logger:         k6test.NewLogger(t),
		Group:          root,
		BPool:          bpool.NewBufferPool(1),
		Samples:        make(chan k6stats.SampleContainer, 1000),
		Tags:           k6lib.NewTagMap(map[string]string{"group": root.Path}),
		BuiltinMetrics: k6metrics.RegisterBuiltinMetrics(k6metrics.NewRegistry()),
	}

	ctx = k6lib.WithState(ctx, state)
	logger := NewLogger(ctx, state.Logger, false, nil)

	session := &testSession{
		Session: &Session{
			id: "1234",
		},
	}

	nm := &NetworkManager{
		ctx:     ctx,
		logger:  logger,
		session: session,
	}

	return nm, session
}

func TestOnRequestPaused(t *testing.T) {
	testCases := []struct {
		name, reqURL                  string
		blockedHostnames, expCDPCalls []string
	}{
		{
			name:             "ok_fail_simple",
			blockedHostnames: []string{"*.test"},
			reqURL:           "http://host.test/",
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
			reqURL:           "http://127.0.0.1/",
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
