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
	k6lib "go.k6.io/k6/lib"
	k6netext "go.k6.io/k6/lib/netext"
	k6types "go.k6.io/k6/lib/types"
	"golang.org/x/net/http2"
)

// Server can be used as a test alternative to a real CDP compatible browser.
type Server struct {
	t             testing.TB
	Mux           *http.ServeMux
	ServerHTTP    *httptest.Server
	Dialer        *k6netext.Dialer
	HTTPTransport *http.Transport
	Context       context.Context
}

// NewServer returns a fully configured and running WS test server.
func NewServer(t testing.TB, opts ...func(*Server)) *Server {
	t.Helper()

	// Create a http.ServeMux and set the httpbin handler as the default
	mux := http.NewServeMux()
	mux.Handle("/", httpbin.New().Handler())

	// Initialize the HTTP server and get its details
	server := httptest.NewServer(mux)
	url, err := url.Parse(server.URL)
	require.NoError(t, err)
	ip := net.ParseIP(url.Hostname())
	require.NotNil(t, ip)
	domain, err := k6lib.NewHostAddress(ip, "")
	require.NoError(t, err)

	// Set up the dialer with shorter timeouts and the custom domains
	dialer := k6netext.NewDialer(net.Dialer{
		Timeout:   2 * time.Second,
		KeepAlive: 10 * time.Second,
		DualStack: true,
	}, k6netext.NewResolver(net.LookupIP, 0, k6types.DNSfirst, k6types.DNSpreferIPv4))

	const wsURL = "wsbin.local"
	dialer.Hosts = map[string]*k6lib.HostAddress{
		wsURL: domain,
	}

	// Pre-configure the HTTP client transport with the dialer and TLS config (incl. HTTP2 support)
	transport := &http.Transport{
		DialContext: dialer.DialContext,
	}
	require.NoError(t, http2.ConfigureTransport(transport))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		server.Close()
		cancel()
	})
	s := &Server{
		t:             t,
		Mux:           mux,
		ServerHTTP:    server,
		Dialer:        dialer,
		HTTPTransport: transport,
		Context:       ctx,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithClosureAbnormalHandler attaches an abnormal closure behavior to Server.
func WithClosureAbnormalHandler(path string) func(*Server) {
	handler := func(w http.ResponseWriter, req *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, w.Header())
		if err != nil {
			// TODO: log
			return
		}
		err = conn.Close() // This forces a connection closure without a proper WS close message exchange
		if err != nil {
			// TODO: log
			return
		}
	}
	return func(s *Server) {
		s.Mux.Handle(path, http.HandlerFunc(handler))
	}
}

// WithEchoHandler attaches an echo handler to Server.
func WithEchoHandler(path string) func(*Server) {
	handler := func(w http.ResponseWriter, req *http.Request) {
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
	}
	return func(s *Server) {
		s.Mux.Handle(path, http.HandlerFunc(handler))
	}
}

// WithCDPHandler attaches a custom CDP handler function to Server.
func WithCDPHandler(
	path string,
	fn func(conn *websocket.Conn, msg *cdproto.Message, writeCh chan cdproto.Message, done chan struct{}),
	cmdsReceived *[]cdproto.MethodType,
) func(*Server) {
	handler := func(w http.ResponseWriter, req *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, w.Header())
		if err != nil {
			return
		}

		done := make(chan struct{})
		writeCh := make(chan cdproto.Message)

		go func() {
			read := func(conn *websocket.Conn) (*cdproto.Message, error) {
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

			for {
				select {
				case <-done:
					return
				default:
				}

				msg, err := read(conn)
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

		go func() {
			write := func(conn *websocket.Conn, msg *cdproto.Message) {
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

			for {
				select {
				case msg := <-writeCh:
					write(conn, &msg)
				case <-done:
					return
				}
			}
		}()

		<-done // Wait for done channel to be closed before closing connection
	}
	return func(s *Server) {
		s.Mux.Handle(path, http.HandlerFunc(handler))
	}
}

// CDPDefaultHandler is a default handler for the CDP WS server.
func CDPDefaultHandler(conn *websocket.Conn, msg *cdproto.Message, writeCh chan cdproto.Message, done chan struct{}) {
	const (
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
		}`
	)

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
				Params: easyjson.RawMessage([]byte(targetAttachedToTargetEvent)),
			}
			writeCh <- cdproto.Message{
				ID:        msg.ID,
				SessionID: msg.SessionID,
				Result:    easyjson.RawMessage([]byte(targetAttachedToTargetResult)),
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
