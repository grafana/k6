package common

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/tests/ws"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	jsonv2 "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnection(t *testing.T) {
	t.Parallel()

	server := ws.NewServer(t, ws.WithEchoHandler("/echo"))

	t.Run("connect", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		url, _ := url.Parse(server.ServerHTTP.URL)
		wsURL := fmt.Sprintf("ws://%s/echo", url.Host)
		conn, err := NewConnection(ctx, wsURL, log.NewNullLogger(), nil)
		conn.Close()

		require.NoError(t, err)
	})
}

func TestConnectionClosureAbnormal(t *testing.T) {
	t.Parallel()

	server := ws.NewServer(t, ws.WithClosureAbnormalHandler("/closure-abnormal"))

	t.Run("closure abnormal", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		url, _ := url.Parse(server.ServerHTTP.URL)
		wsURL := fmt.Sprintf("ws://%s/closure-abnormal", url.Host)
		conn, err := NewConnection(ctx, wsURL, log.NewNullLogger(), nil)

		if assert.NoError(t, err) {
			action := target.SetDiscoverTargets(true)
			err := action.Do(cdp.WithExecutor(ctx, conn))
			require.ErrorContains(t, err, "websocket: close 1006 (abnormal closure): unexpected EOF")
		}
	})
}

func TestConnectionSendRecv(t *testing.T) {
	t.Parallel()

	server := ws.NewServer(t, ws.WithCDPHandler("/cdp", ws.CDPDefaultHandler, nil))

	t.Run("send command with empty reply", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		url, _ := url.Parse(server.ServerHTTP.URL)
		wsURL := fmt.Sprintf("ws://%s/cdp", url.Host)
		conn, err := NewConnection(ctx, wsURL, log.NewNullLogger(), nil)

		if assert.NoError(t, err) {
			action := target.SetDiscoverTargets(true)
			err := action.Do(cdp.WithExecutor(ctx, conn))
			require.NoError(t, err)
		}
	})
}

// TestConnectionRejectedTarget ensures a target rejected by the attach filter
// is both resumed and detached from. Staying attached without resuming keeps
// the target paused for every client; staying attached after resuming can
// still stall it later, since the browser waits on all attached clients for
// some events (e.g. after a crash-reload). Resuming alone is enough to make
// TestContextlessConnectionDoesNotStallNavigation pass, so this is the only
// test that catches a missing detach.
func TestConnectionRejectedTarget(t *testing.T) {
	t.Parallel()

	const rejectedSessionID = "session_id_0123456789"

	attachedEvent := fmt.Sprintf(`
	{
		"sessionId": %q,
		"targetInfo": {
			"targetId": "target_id_0123456789",
			"type": "page",
			"title": "",
			"url": "about:blank",
			"attached": true,
			"browserContextId": "browser_context_id_0123456789"
		},
		"waitingForDebugger": true
	}`, rejectedSessionID)

	received := make(chan cdproto.Message, 16)
	handler := func(conn *websocket.Conn, msg *cdproto.Message, writeCh chan cdproto.Message, done chan struct{}) {
		if msg.Method == "" {
			return
		}
		received <- *msg
		if msg.Method == cdproto.MethodType(cdproto.CommandTargetSetDiscoverTargets) {
			writeCh <- cdproto.Message{
				Method: cdproto.EventTargetAttachedToTarget,
				Params: jsontext.Value(attachedEvent),
			}
			writeCh <- cdproto.Message{
				ID:     msg.ID,
				Result: jsontext.Value([]byte("{}")),
			}
		}
	}

	server := ws.NewServer(t, ws.WithCDPHandler("/cdp", handler, nil))

	ctx := context.Background()
	u, err := url.Parse(server.ServerHTTP.URL)
	require.NoError(t, err)
	wsURL := fmt.Sprintf("ws://%s/cdp", u.Host)
	rejectAll := func(*target.EventAttachedToTarget) bool { return false }
	conn, err := NewConnection(ctx, wsURL, log.NewNullLogger(), rejectAll)
	require.NoError(t, err)
	t.Cleanup(conn.Close)

	action := target.SetDiscoverTargets(true)
	require.NoError(t, action.Do(cdp.WithExecutor(ctx, conn)))

	timeout := time.After(5 * time.Second)
	var gotRunIfWaiting, gotDetach bool
	for !gotRunIfWaiting || !gotDetach {
		select {
		case msg := <-received:
			switch msg.Method {
			case cdproto.MethodType(cdpruntime.CommandRunIfWaitingForDebugger):
				require.Equal(t, target.SessionID(rejectedSessionID), msg.SessionID)
				gotRunIfWaiting = true
			case cdproto.MethodType(target.CommandDetachFromTarget):
				// Target.detachFromTarget is a browser-level command: the
				// session goes in the params, not in the message session ID.
				require.Empty(t, msg.SessionID)
				var params target.DetachFromTargetParams
				require.NoError(t, jsonv2.Unmarshal(msg.Params, &params, defaultJSONV2Options))
				require.Equal(t, target.SessionID(rejectedSessionID), params.SessionID)
				gotDetach = true
			}
		case <-timeout:
			t.Fatalf(
				"timed out waiting for the rejected target to be released: runIfWaitingForDebugger=%t detachFromTarget=%t",
				gotRunIfWaiting, gotDetach,
			)
		}
	}
}

func TestConnectionCreateSession(t *testing.T) {
	t.Parallel()

	cmdsReceived := make([]cdproto.MethodType, 0)
	handler := func(conn *websocket.Conn, msg *cdproto.Message, writeCh chan cdproto.Message, done chan struct{}) {
		if msg.SessionID == "" && msg.Method != "" {
			switch msg.Method {
			case cdproto.MethodType(cdproto.CommandTargetSetDiscoverTargets):
				writeCh <- cdproto.Message{
					ID:        msg.ID,
					SessionID: msg.SessionID,
					Result:    jsontext.Value([]byte("{}")),
				}
			case cdproto.MethodType(cdproto.CommandTargetAttachToTarget):
				switch msg.Method {
				case cdproto.MethodType(cdproto.CommandTargetSetDiscoverTargets):
					writeCh <- cdproto.Message{
						ID:        msg.ID,
						SessionID: msg.SessionID,
						Result:    jsontext.Value([]byte("{}")),
					}
				case cdproto.MethodType(cdproto.CommandTargetAttachToTarget):
					writeCh <- cdproto.Message{
						Method: cdproto.EventTargetAttachedToTarget,
						Params: jsontext.Value([]byte(`
						{
							"sessionId": "0123456789",
							"targetInfo": {
								"targetId": "abcdef0123456789",
								"type": "page",
								"title": "",
								"url": "about:blank",
								"attached": true,
								"browserContextId": "0123456789876543210"
							},
							"waitingForDebugger": false
						}
						`)),
					}
					writeCh <- cdproto.Message{
						ID:        msg.ID,
						SessionID: msg.SessionID,
						Result:    jsontext.Value([]byte(`{"sessionId":"0123456789"}`)),
					}
				}
			}
		}
	}

	server := ws.NewServer(t, ws.WithCDPHandler("/cdp", handler, &cmdsReceived))

	t.Run("create session for target", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		url, _ := url.Parse(server.ServerHTTP.URL)
		wsURL := fmt.Sprintf("ws://%s/cdp", url.Host)
		conn, err := NewConnection(ctx, wsURL, log.NewNullLogger(), nil)

		if assert.NoError(t, err) {
			session, err := conn.createSession(&target.Info{
				TargetID:         "abcdef0123456789",
				Type:             "page",
				BrowserContextID: "0123456789876543210",
			})

			require.NoError(t, err)
			require.NotNil(t, session)
			require.NotEmpty(t, session.id)
			require.NotEmpty(t, conn.sessions)
			require.Len(t, conn.sessions, 1)
			require.Equal(t, conn.sessions[session.id], session)
			require.Equal(t, []cdproto.MethodType{
				cdproto.CommandTargetAttachToTarget,
			}, cmdsReceived)
		}
	})
}
