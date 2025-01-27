package common

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"go.k6.io/k6/internal/js/modules/k6/browser/log"
	"go.k6.io/k6/internal/js/modules/k6/browser/tests/ws"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/gorilla/websocket"
	"github.com/mailru/easyjson"
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
					Result:    easyjson.RawMessage([]byte("{}")),
				}
			case cdproto.MethodType(cdproto.CommandTargetAttachToTarget):
				switch msg.Method {
				case cdproto.MethodType(cdproto.CommandTargetSetDiscoverTargets):
					writeCh <- cdproto.Message{
						ID:        msg.ID,
						SessionID: msg.SessionID,
						Result:    easyjson.RawMessage([]byte("{}")),
					}
				case cdproto.MethodType(cdproto.CommandTargetAttachToTarget):
					writeCh <- cdproto.Message{
						Method: cdproto.EventTargetAttachedToTarget,
						Params: easyjson.RawMessage([]byte(`
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
						Result:    easyjson.RawMessage([]byte(`{"sessionId":"0123456789"}`)),
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
