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

// attachedToTargetEvent returns a Target.attachedToTarget event payload for
// a paused target, as the browser sends when auto-attaching a new target.
func attachedToTargetEvent(sessionID target.SessionID, targetType, browserContextID string) string {
	return fmt.Sprintf(`
	{
		"sessionId": %q,
		"targetInfo": {
			"targetId": "target_id_0123456789",
			"type": %q,
			"title": "",
			"url": "about:blank",
			"attached": true,
			"browserContextId": %q
		},
		"waitingForDebugger": true
	}`, sessionID, targetType, browserContextID)
}

// newAttachedToTargetServer starts a fake CDP websocket server that records
// every command it receives and emits the given Target.attachedToTarget
// event when the client sends Target.setDiscoverTargets.
func newAttachedToTargetServer(t *testing.T, attachedEvent string) (string, chan cdproto.Message) {
	t.Helper()

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
	u, err := url.Parse(server.ServerHTTP.URL)
	require.NoError(t, err)

	return fmt.Sprintf("ws://%s/cdp", u.Host), received
}

// requireResumeThenDetach consumes the server-received messages until the
// rejected target's session is detached from, asserting the target was
// resumed first and that both messages are correctly addressed. The
// websocket preserves ordering, so the resume must arrive before the
// detach.
func requireResumeThenDetach(t *testing.T, received <-chan cdproto.Message, sid target.SessionID) {
	t.Helper()

	timeout := time.After(5 * time.Second)
	var sawResume bool
	for {
		select {
		case msg := <-received:
			switch msg.Method {
			case cdproto.MethodType(cdpruntime.CommandRunIfWaitingForDebugger):
				// The resume targets the paused session directly.
				require.Equal(t, sid, msg.SessionID)
				sawResume = true
			case cdproto.MethodType(target.CommandDetachFromTarget):
				// Target.detachFromTarget is a browser-level command: the
				// session goes in the params, not in the message session ID.
				require.Empty(t, msg.SessionID)
				var params target.DetachFromTargetParams
				require.NoError(t, jsonv2.Unmarshal(msg.Params, &params, defaultJSONV2Options))
				require.Equal(t, sid, params.SessionID)
				require.True(t, sawResume, "expected Runtime.runIfWaitingForDebugger before the detach")
				return
			}
		case <-timeout:
			t.Fatal("timed out waiting for the rejected target to be released")
		}
	}
}

// TestConnectionRejectedTarget ensures a target rejected by the attach filter
// is resumed and then detached from. The browser keeps a new target paused
// until every client attached with waitForDebuggerOnStart releases it.
// Detaching should be enough to release the hold, but the browser has a bug
// where a detached target can stay paused, so the resume must come first —
// the same workaround Playwright uses (crConnection.ts, CRSession.detach).
func TestConnectionRejectedTarget(t *testing.T) {
	t.Parallel()

	const rejectedSessionID = target.SessionID("session_id_0123456789")

	wsURL, received := newAttachedToTargetServer(t,
		attachedToTargetEvent(rejectedSessionID, "page", "browser_context_id_0123456789"))

	ctx := context.Background()
	rejectAll := func(*target.EventAttachedToTarget) bool { return false }
	conn, err := NewConnection(ctx, wsURL, log.NewNullLogger(), rejectAll)
	require.NoError(t, err)
	t.Cleanup(conn.Close)

	action := target.SetDiscoverTargets(true)
	require.NoError(t, action.Do(cdp.WithExecutor(ctx, conn)))

	requireResumeThenDetach(t, received, rejectedSessionID)
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
