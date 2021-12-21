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

package ws

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/gorilla/websocket"
	"github.com/mailru/easyjson"
	"github.com/mailru/easyjson/jlexer"
	"github.com/mailru/easyjson/jwriter"
	"github.com/mccutchen/go-httpbin/httpbin"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"

	k6lib "go.k6.io/k6/lib"
	k6netext "go.k6.io/k6/lib/netext"
	k6types "go.k6.io/k6/lib/types"
)

const (
	DummyCDPSessionID        = "session_id_0123456789"
	DummyCDPTargetID         = "target_id_0123456789"
	DummyCDPBrowserContextID = "browser_context_id_0123456789"
	WebSocketServerURL       = "wsbin.local"
)

var (
	CDPTargetAttachedToTargetRequest = fmt.Sprintf(`
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
	`, DummyCDPSessionID, DummyCDPTargetID, DummyCDPBrowserContextID)

	CDPTargetAttachedToTargetResponse = fmt.Sprintf(`{"sessionId":"%s"}`, DummyCDPSessionID)
)

// NewWSServerWithCDPHandler creates a WS test server with a custom CDP handler function
func NewWSServerWithCDPHandler(
	t testing.TB,
	fn func(conn *websocket.Conn, msg *cdproto.Message, writeCh chan cdproto.Message, done chan struct{}),
	cmdsReceived *[]cdproto.MethodType) *WSTestServer {
	return NewWSServer(t, "/cdp", getWebsocketHandlerCDP(fn, cmdsReceived))
}

// WSTestServer can be used as a test alternative to a real CDP compatible browser.
type WSTestServer struct {
	Mux           *http.ServeMux
	ServerHTTP    *httptest.Server
	Dialer        *k6netext.Dialer
	HTTPTransport *http.Transport
	Context       context.Context
	Cleanup       func()
}

// NewWSServerWithClosureAbnormal creates a WS test server with abnormal closure behavior
func NewWSServerWithClosureAbnormal(t testing.TB) *WSTestServer {
	return NewWSServer(t, "/closure-abnormal", getWebsocketHandlerAbnormalClosure())
}

// NewWSServerWithEcho creates a WS test server with an echo handler
func NewWSServerWithEcho(t testing.TB) *WSTestServer {
	return NewWSServer(t, "/echo", getWebsocketHandlerEcho())
}

// NewWSServer returns a fully configured and running WS test server
func NewWSServer(t testing.TB, path string, handler http.Handler) *WSTestServer {
	t.Helper()

	// Create a http.ServeMux and set the httpbin handler as the default
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.Handle("/", httpbin.New().Handler())

	// Initialize the HTTP server and get its details
	httpSrv := httptest.NewServer(mux)
	httpURL, err := url.Parse(httpSrv.URL)
	require.NoError(t, err)
	httpIP := net.ParseIP(httpURL.Hostname())
	require.NotNil(t, httpIP)

	httpDomainValue, err := k6lib.NewHostAddress(httpIP, "")
	require.NoError(t, err)

	// Set up the dialer with shorter timeouts and the custom domains
	dialer := k6netext.NewDialer(net.Dialer{
		Timeout:   2 * time.Second,
		KeepAlive: 10 * time.Second,
		DualStack: true,
	}, k6netext.NewResolver(net.LookupIP, 0, k6types.DNSfirst, k6types.DNSpreferIPv4))
	dialer.Hosts = map[string]*k6lib.HostAddress{
		WebSocketServerURL: httpDomainValue,
	}

	// Pre-configure the HTTP client transport with the dialer and TLS config (incl. HTTP2 support)
	transport := &http.Transport{
		DialContext: dialer.DialContext,
	}
	require.NoError(t, http2.ConfigureTransport(transport))

	ctx, ctxCancel := context.WithCancel(context.Background())
	return &WSTestServer{
		Mux:           mux,
		ServerHTTP:    httpSrv,
		Dialer:        dialer,
		HTTPTransport: transport,
		Context:       ctx,
		Cleanup: func() {
			httpSrv.Close()
			ctxCancel()
		},
	}
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
				Params: easyjson.RawMessage([]byte(CDPTargetAttachedToTargetRequest)),
			}
			writeCh <- cdproto.Message{
				ID:        msg.ID,
				SessionID: msg.SessionID,
				Result:    easyjson.RawMessage([]byte(CDPTargetAttachedToTargetResponse)),
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

func getWebsocketHandlerEcho() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, w.Header())
		if err != nil {
			return
		}
		messageType, r, e := conn.NextReader()
		if e != nil {
			return
		}
		var wc io.WriteCloser
		wc, err = conn.NextWriter(messageType)
		if err != nil {
			return
		}
		if _, err = io.Copy(wc, r); err != nil {
			return
		}
		if err = wc.Close(); err != nil {
			return
		}
		err = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(10*time.Second),
		)
		if err != nil {
			return
		}
	})
}

func getWebsocketHandlerAbnormalClosure() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, w.Header())
		if err != nil {
			return
		}
		err = conn.Close() // This forces a connection closure without a proper WS close message exchange
		if err != nil {
			return
		}
	})
}
