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
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/gorilla/websocket"
	"github.com/mailru/easyjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionCreateSession(t *testing.T) {
	t.Parallel()

	const (
		cdpTargetID         = "target_id_0123456789"
		cdpBrowserContextID = "browser_context_id_0123456789"

		targetAttachedToTargetEvent = `
		{
			"sessionId": "session_id_0123456789",
			"targetInfo": {
				"targetId": "target_id_0123456789",
				"type": "page",
				"title": "",
				"url": "about:blank",
				"attached": true,
				"browserContextId": "browser_context_id_0123456789"
			},
			"waitingForDebugger": false
		}`

		targetAttachedToTargetResult = `
		{
			"sessionId":"session_id_0123456789"
		}
		`
	)

	cmdsReceived := make([]cdproto.MethodType, 0)
	handler := func(conn *websocket.Conn, msg *cdproto.Message, writeCh chan cdproto.Message, done chan struct{}) {
		if msg.SessionID != "" && msg.Method != "" {
			if msg.Method == cdproto.MethodType(cdproto.CommandPageEnable) {
				writeCh <- cdproto.Message{
					ID:        msg.ID,
					SessionID: msg.SessionID,
				}
				close(done) // We're done after receiving the Page.enable command
			}
		} else if msg.Method != "" {
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
					Params: easyjson.RawMessage([]byte(targetAttachedToTargetEvent)),
				}
				writeCh <- cdproto.Message{
					ID:        msg.ID,
					SessionID: msg.SessionID,
					Result:    easyjson.RawMessage([]byte(targetAttachedToTargetResult)),
				}
			}
		}
	}

	server := ws.NewServer(t, ws.WithCDPHandler("/cdp", handler, &cmdsReceived))

	t.Run("send and recv session commands", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		url, _ := url.Parse(server.ServerHTTP.URL)
		wsURL := fmt.Sprintf("ws://%s/cdp", url.Host)
		conn, err := NewConnection(ctx, wsURL, log.NewNullLogger(), nil)

		if assert.NoError(t, err) {
			session, err := conn.createSession(&target.Info{
				Type:             "page",
				TargetID:         cdpTargetID,
				BrowserContextID: cdpBrowserContextID,
			})

			if assert.NoError(t, err) {
				action := cdppage.Enable()
				err := action.Do(cdp.WithExecutor(ctx, session))

				require.NoError(t, err)
				require.Equal(t, []cdproto.MethodType{
					cdproto.CommandTargetAttachToTarget,
					cdproto.CommandPageEnable,
				}, cmdsReceived)
			}

			conn.Close()
		}
	})
}
