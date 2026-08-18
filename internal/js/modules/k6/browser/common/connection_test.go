package common

import (
	"context"
	"fmt"
	"net/url"
	"sync"
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

// resumeRespondedSentinel marks, in the received stream, the moment the
// server wrote its response to Runtime.runIfWaitingForDebugger. Anything
// the client sends after seeing that response arrives later in the stream.
const resumeRespondedSentinel = cdproto.MethodType("test:resume-responded")

// newAttachedToTargetServer starts a fake CDP websocket server that records
// every command it receives and emits the given Target.attachedToTarget
// event when the client sends Target.setDiscoverTargets. The response to
// Runtime.runIfWaitingForDebugger is delayed by resumeDelay, and a
// resumeRespondedSentinel is recorded when it is written.
func newAttachedToTargetServer(
	t *testing.T, attachedEvent string, resumeDelay time.Duration,
) (string, chan cdproto.Message) {
	t.Helper()

	received := make(chan cdproto.Message, 16)
	handler := func(conn *websocket.Conn, msg *cdproto.Message, writeCh chan cdproto.Message, done chan struct{}) {
		if msg.Method == "" {
			return
		}
		received <- *msg
		switch msg.Method {
		case cdproto.MethodType(cdproto.CommandTargetSetDiscoverTargets):
			writeCh <- cdproto.Message{
				Method: cdproto.EventTargetAttachedToTarget,
				Params: jsontext.Value(attachedEvent),
			}
			writeCh <- cdproto.Message{
				ID:     msg.ID,
				Result: jsontext.Value([]byte("{}")),
			}
		case cdproto.MethodType(cdpruntime.CommandRunIfWaitingForDebugger):
			response := cdproto.Message{
				ID:        msg.ID,
				SessionID: msg.SessionID,
				Result:    jsontext.Value([]byte("{}")),
			}
			// Respond asynchronously so a delayed response does not also
			// delay reading the messages the client sends in the meantime.
			go func() {
				select {
				case <-time.After(resumeDelay):
				case <-done:
					return
				}
				received <- cdproto.Message{Method: resumeRespondedSentinel}
				select {
				case writeCh <- response:
				case <-done:
				}
			}()
		}
	}

	server := ws.NewServer(t, ws.WithCDPHandler("/cdp", handler, nil))
	u, err := url.Parse(server.ServerHTTP.URL)
	require.NoError(t, err)

	return fmt.Sprintf("ws://%s/cdp", u.Host), received
}

// requireResumeThenDetach consumes the server-received messages until the
// rejected target's session is detached from, asserting the client resumed
// the target, awaited the server's response, and only then detached, with
// both commands correctly addressed.
func requireResumeThenDetach(t *testing.T, received <-chan cdproto.Message, sid target.SessionID) {
	t.Helper()

	timeout := time.After(5 * time.Second)
	var sawResume, sawResponse bool
	for {
		select {
		case msg := <-received:
			switch msg.Method {
			case cdproto.MethodType(cdpruntime.CommandRunIfWaitingForDebugger):
				// The resume targets the paused session directly.
				require.Equal(t, sid, msg.SessionID)
				sawResume = true
			case resumeRespondedSentinel:
				sawResponse = true
			case cdproto.MethodType(target.CommandDetachFromTarget):
				// Target.detachFromTarget is a browser-level command: the
				// session goes in the params, not in the message session ID.
				require.Empty(t, msg.SessionID)
				var params target.DetachFromTargetParams
				require.NoError(t, jsonv2.Unmarshal(msg.Params, &params, defaultJSONV2Options))
				require.Equal(t, sid, params.SessionID)
				require.True(t, sawResume, "expected Runtime.runIfWaitingForDebugger before the detach")
				require.True(t, sawResponse,
					"expected the detach to be sent only after the resume response was written")
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
// Wire order is not enough: only awaiting the response guarantees the
// browser processed the resume before the detach, so the server delays the
// resume response to catch a client that does not wait for it.
func TestConnectionRejectedTarget(t *testing.T) {
	t.Parallel()

	const rejectedSessionID = target.SessionID("session_id_0123456789")

	wsURL, received := newAttachedToTargetServer(t,
		attachedToTargetEvent(rejectedSessionID, "page", "browser_context_id_0123456789"),
		300*time.Millisecond)

	ctx := context.Background()
	rejectAll := func(*target.EventAttachedToTarget) bool { return false }
	conn, err := NewConnection(ctx, wsURL, log.NewNullLogger(), rejectAll)
	require.NoError(t, err)
	t.Cleanup(conn.Close)

	action := target.SetDiscoverTargets(true)
	require.NoError(t, action.Do(cdp.WithExecutor(ctx, conn)))

	requireResumeThenDetach(t, received, rejectedSessionID)
}

// newTestConnection dials a minimal fake CDP server that acknowledges
// every command with an empty success reply.
func newTestConnection(t *testing.T) *Connection {
	t.Helper()

	handler := func(_ *websocket.Conn, msg *cdproto.Message, writeCh chan cdproto.Message, _ chan struct{}) {
		writeCh <- cdproto.Message{ID: msg.ID, SessionID: msg.SessionID, Result: jsontext.Value([]byte("{}"))}
	}
	server := ws.NewServer(t, ws.WithCDPHandler("/cdp", handler, nil))
	u, err := url.Parse(server.ServerHTTP.URL)
	require.NoError(t, err)

	conn, err := NewConnection(context.Background(), fmt.Sprintf("ws://%s/cdp", u.Host), log.NewNullLogger(), nil)
	require.NoError(t, err)
	t.Cleanup(conn.Close)

	return conn
}

// TestConnectionDeliverToSessionAfterClose is a deterministic regression
// test for a race in recvLoop's dispatch to a session's readCh: getSession
// only holds the sessions lock for the map lookup, so a session obtained
// this way can already be closed - by closeSession, callable from any
// goroutine, not just recvLoop - by the time the dispatch actually runs.
// Without the <-session.done case, delivering to an already-closed
// session's readCh blocks forever: its readLoop has already exited, so
// there is no reader left, and neither c.closeCh nor c.done fire just
// because one session closed. That would stall recvLoop - the single
// goroutine reading every message for the whole connection - along with
// every other session sharing it.
//
// The session's readLoop is deliberately never started (done is closed by
// hand instead of via NewSession + closeSession), so there is provably no
// reader on readCh, ever - this removes any dependency on readLoop's own
// exit timing and drives the exact interleaving deterministically, rather
// than hoping to catch it.
func TestConnectionDeliverToSessionAfterClose(t *testing.T) {
	t.Parallel()

	const sid = target.SessionID("session_id_0123456789")
	const tid = target.ID("target_id_0123456789")

	conn := newTestConnection(t)

	session := &Session{
		BaseEventEmitter: NewBaseEventEmitter(context.Background()),
		conn:             conn,
		id:               sid,
		targetID:         tid,
		readCh:           make(chan *cdproto.Message),
		done:             make(chan struct{}),
		msgIDGen:         conn.msgIDGen,
		logger:           log.NewNullLogger(),
	}
	close(session.done) // no readLoop was ever started to close it itself

	done := make(chan bool, 1)
	go func() {
		done <- conn.deliverToSession(session, &cdproto.Message{Method: "Test.event", SessionID: sid})
	}()

	select {
	case stop := <-done:
		require.False(t, stop, "delivering to a closed session must not signal a connection-level stop")
	case <-time.After(2 * time.Second):
		t.Fatal("deliverToSession blocked forever delivering to an already-closed session")
	}
}

// TestConnectionCloseSessionConcurrent proves closeSession is safe to call
// concurrently, and repeatedly, for the same session - the exact shape of
// detachSession's own closeSession call racing the browser's genuine
// Target.detachedFromTarget event arriving and being handled by recvLoop's
// own closeSession call, or any other pair of callers. Under -race, a
// data race here would fail the test; closeSession's own locking must
// also make it safe to call twice - only the first caller may release the
// session, every other caller must see it already gone.
func TestConnectionCloseSessionConcurrent(t *testing.T) {
	t.Parallel()

	const sid = target.SessionID("session_id_0123456789")
	const tid = target.ID("target_id_0123456789")
	const callers = 50

	conn := newTestConnection(t)

	conn.sessionsMu.Lock()
	conn.sessions[sid] = NewSession(context.Background(), conn, sid, tid, log.NewNullLogger(), conn.msgIDGen)
	conn.sessionsMu.Unlock()

	var wg sync.WaitGroup
	results := make([]bool, callers)
	for i := range callers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = conn.closeSession(sid, tid)
		}(i)
	}
	wg.Wait()

	var released int
	for _, ok := range results {
		if ok {
			released++
		}
	}
	require.Equal(t, 1, released, "exactly one concurrent closeSession call should release the session")
	require.Nil(t, conn.getSession(sid), "the session must be gone from the connection after closing")
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
