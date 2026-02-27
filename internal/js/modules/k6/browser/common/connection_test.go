package common

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.k6.io/k6/internal/js/modules/k6/browser/log"
	"go.k6.io/k6/internal/js/modules/k6/browser/tests/ws"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
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

	ctx := context.Background()
	url, _ := url.Parse(server.ServerHTTP.URL)
	wsURL := fmt.Sprintf("ws://%s/closure-abnormal", url.Host)
	conn, err := NewConnection(ctx, wsURL, log.NewNullLogger(), nil)

	if !assert.NoError(t, err) {
		return
	}

	err = target.SetDiscoverTargets(true).Do(cdp.WithExecutor(ctx, conn))
	require.Error(t, err)

	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) {
		require.Equal(t, websocket.CloseAbnormalClosure, closeErr.Code)
		return
	}

	msg := err.Error()
	require.Truef(t,
		strings.Contains(msg, "1006") ||
			strings.Contains(msg, "connection reset by peer"),
		"expected abnormal websocket closure error, got: %v", err,
	)
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

// Ensure the connection can tear down even when an abnormal closure happens
// while there is no request actively waiting on c.errorCh.
func TestConnectionAbnormalClosureIdleCloses(t *testing.T) {
	t.Parallel()

	srv := ws.NewServer(t, ws.WithClosureAbnormalHandler("/closure-abnormal-idle"))
	u, err := url.Parse(srv.ServerHTTP.URL)
	require.NoError(t, err)
	wsURL := fmt.Sprintf("ws://%s/closure-abnormal-idle", u.Host)

	conn, err := NewConnection(t.Context(), wsURL, log.NewNullLogger(), nil)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	t.Cleanup(conn.Close)

	select {
	case <-conn.done:
		// Expected: the connection should tear down even if there is no active
		// request waiting on c.errorCh when the abnormal closure happens.
	case <-time.After(time.Minute):
		t.Fatalf("connection did not close after idle abnormal closure")
	}
}

// Ensure the connection can tear down even when an abnormal closure happens
// while there is a pending request waiting on c.errorCh. This tests the case
// where the connection receives an abnormal closure while there is an active
// request waiting for a response, which will be pending when the abnormal
// closure happens. The connection should still tear down properly without
// deadlocking, even if there is a pending request waiting on c.errorCh
// when the abnormal closure happens.
func TestConnectionAbnormalClosurePendingCloses(t *testing.T) {
	t.Parallel()

	srv := ws.NewServer(t, ws.WithClosureAbnormalHandler("/closure-abnormal-pending"))
	u, err := url.Parse(srv.ServerHTTP.URL)
	require.NoError(t, err)

	conn, err := NewConnection(
		t.Context(),
		fmt.Sprintf("ws://%s/closure-abnormal-pending", u.Host),
		log.NewNullLogger(),
		nil, // onTargetAttachedToTarget callback
	)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	t.Cleanup(conn.Close)

	// Send a command that will be pending when the abnormal closure happens.
	if err := target.SetDiscoverTargets(true).Do(
		cdp.WithExecutor(t.Context(), conn),
	); err == nil {
		t.Fatalf("expected abnormal-closure error")
	}

	// The connection should still tear down even if there is a pending
	// request waiting on c.errorCh when the abnormal closure happens.
	select {
	case <-conn.done:
		// Once a sender is receiving from errorCh/closeCh, teardown completes.
	case <-time.After(time.Minute):
		t.Fatalf("connection did not close with pending request")
	}
}

func TestConnectionSendDoneReturnsError(t *testing.T) {
	t.Parallel()

	conn := &Connection{
		ctx:     context.Background(),
		logger:  log.NewNullLogger(),
		sendCh:  make(chan *cdproto.Message),
		closeCh: make(chan int),
		errorCh: make(chan error, 1),
		done:    make(chan struct{}),
	}
	close(conn.done)

	err := conn.send(
		context.Background(),
		&cdproto.Message{ID: 1},
		make(chan *cdproto.Message),
		nil,
	)
	require.Error(t, err)
}
