/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

// Package testutils is indended only for use in tests, do not import in production code!
package testutils

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/chromedp/cdproto"
	"github.com/gorilla/websocket"
	"github.com/mailru/easyjson"
	"github.com/mailru/easyjson/jlexer"
	"github.com/mailru/easyjson/jwriter"
)

const (
	DEAFULT_CDP_SESSION_ID         = "session_id_0123456789"
	DEAFULT_CDP_TARGET_ID          = "target_id_0123456789"
	DEAFULT_CDP_BROWSER_CONTEXT_ID = "browser_context_id_0123456789"
)

var DEFAULT_CDP_TARGET_ATTACHED_TO_TARGET_MSG = fmt.Sprintf(`
	{
		"sessionId": "%s",
		"targetInfo": {
			"targetId": "%s",
			"type": "page",
			"title": "",
			"url": "about:blank",
			"attached": true,
			"browserContextId": "%s"
		},
		"waitingForDebugger": false
	}
	`, DEAFULT_CDP_SESSION_ID, DEAFULT_CDP_TARGET_ID, DEAFULT_CDP_BROWSER_CONTEXT_ID)
var DEFAULT_CDP_TARGET_ATTACH_TO_TARGET_RESPONSE = fmt.Sprintf(`{"sessionId":"%s"}`, DEAFULT_CDP_SESSION_ID)

func getWebsocketHandlerCDP(
	fn func(conn *websocket.Conn, msg *cdproto.Message, writeCh chan cdproto.Message, done chan struct{}),
	cmdsReceived *[]cdproto.MethodType) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, w.Header())
		if err != nil {
			return
		}

		done := make(chan struct{})
		writeCh := make(chan cdproto.Message)

		// Read loop
		go func() {
			for {
				select {
				case <-done:
					return
				default:
				}

				msg, err := CDPReadMsg(conn)
				if err != nil {
					close(done)
					return
				}

				if msg.Method != "" && cmdsReceived != nil {
					*cmdsReceived = append(*cmdsReceived, msg.Method)
				}

				fn(conn, msg, writeCh, done)
			}
		}()

		// Write loop
		go func() {
			for {
				select {
				case msg := <-writeCh:
					CDPWriteMsg(conn, &msg)
				case <-done:
					return
				}
			}
		}()

		<-done // Wait for done channel to be closed before closing connection
	})
}

// NewWSTestServerWithCDPHandler creates a WS test server with a custom CDP handler function
func NewWSTestServerWithCDPHandler(
	t testing.TB,
	fn func(conn *websocket.Conn, msg *cdproto.Message, writeCh chan cdproto.Message, done chan struct{}),
	cmdsReceived *[]cdproto.MethodType) *WSTestServer {
	return NewWSTestServer(t, "/cdp", getWebsocketHandlerCDP(fn, cmdsReceived))
}

// CDPDefaultCDPHandler is a default handler for the CDP WS server
func CDPDefaultHandler(conn *websocket.Conn, msg *cdproto.Message, writeCh chan cdproto.Message, done chan struct{}) {
	if msg.SessionID != "" && msg.Method != "" {
		switch msg.Method {
		default:
			writeCh <- cdproto.Message{
				ID:        msg.ID,
				SessionID: msg.SessionID,
			}
		}
	} else if msg.Method != "" {
		switch msg.Method {
		case cdproto.MethodType(cdproto.CommandTargetAttachToTarget):
			writeCh <- cdproto.Message{
				Method: cdproto.EventTargetAttachedToTarget,
				Params: easyjson.RawMessage([]byte(DEFAULT_CDP_TARGET_ATTACHED_TO_TARGET_MSG)),
			}
			writeCh <- cdproto.Message{
				ID:        msg.ID,
				SessionID: msg.SessionID,
				Result:    easyjson.RawMessage([]byte(DEFAULT_CDP_TARGET_ATTACH_TO_TARGET_RESPONSE)),
			}
		default:
			writeCh <- cdproto.Message{
				ID:        msg.ID,
				SessionID: msg.SessionID,
				Result:    easyjson.RawMessage([]byte("{}")),
			}
		}
	}
}

// CDPReadMsg reads a CDP message to the provided Websocket connection
func CDPReadMsg(conn *websocket.Conn) (*cdproto.Message, error) {
	_, buf, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	var msg cdproto.Message
	decoder := jlexer.Lexer{Data: buf}
	msg.UnmarshalEasyJSON(&decoder)
	if err := decoder.Error(); err != nil {
		return nil, err
	}

	return &msg, nil
}

// CDPWriteMsg writes a CDP message to the provided Websocket connection
func CDPWriteMsg(conn *websocket.Conn, msg *cdproto.Message) {
	encoder := jwriter.Writer{}
	msg.MarshalEasyJSON(&encoder)
	if err := encoder.Error; err != nil {
		return
	}

	writer, err := conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return
	}
	if _, err := encoder.DumpTo(writer); err != nil {
		return
	}
	if err := writer.Close(); err != nil {
		return
	}
}
