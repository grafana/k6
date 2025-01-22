package tests

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

// testBrowserProxy wraps a testBrowser and
// proxies WS messages to/from it.
type testBrowserProxy struct {
	t testing.TB

	mu sync.Mutex // avoid concurrent connect requests

	tb *testBrowser
	ts *httptest.Server

	connected bool
}

func newTestBrowserProxy(tb testing.TB, b *testBrowser) *testBrowserProxy {
	tb.Helper()

	p := &testBrowserProxy{
		t:  tb,
		tb: b,
	}
	p.ts = httptest.NewServer(p.connHandler())

	return p
}

func (p *testBrowserProxy) wsURL() string {
	p.t.Helper()

	tsURL, err := url.Parse(p.ts.URL)
	if err != nil {
		p.t.Fatalf("error parsing test server URL: %v", err)
	}
	return fmt.Sprintf("ws://%s", tsURL.Host)
}

func (p *testBrowserProxy) connHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		defer p.mu.Unlock()

		upgrader := websocket.Upgrader{} // default options

		// Upgrade in connection from client
		in, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			p.t.Fatalf("error upgrading proxy connection: %v", err)
		}
		defer in.Close() //nolint:errcheck

		// Connect to testBrowser CDP WS
		out, _, err := websocket.DefaultDialer.Dial(p.tb.wsURL, nil) //nolint:bodyclose
		if err != nil {
			p.t.Fatalf("error connecting to test browser: %v", err)
		}
		defer out.Close() //nolint:errcheck

		p.connected = true

		// Stop proxy when test exits
		ctx, cancel := context.WithCancel(context.Background())
		p.t.Cleanup(func() {
			cancel()     // stop forwarding mssgs
			p.ts.Close() // close test server
		})

		var wg sync.WaitGroup
		wg.Add(2)

		go p.fwdMssgs(ctx, in, out, &wg)
		go p.fwdMssgs(ctx, out, in, &wg)

		wg.Wait()
	})
}

func (p *testBrowserProxy) fwdMssgs(ctx context.Context,
	in, out *websocket.Conn, wg *sync.WaitGroup,
) {
	p.t.Helper()
	defer wg.Done()

LOOP:
	for {
		select {
		case <-ctx.Done():
			break LOOP
		default:
			mt, message, err := in.ReadMessage()
			if err != nil {
				var cerr *websocket.CloseError
				if errors.As(err, &cerr) {
					// If WS conn is closed, just return
					return
				}
				p.t.Fatalf("error reading message: %v", err)
			}

			err = out.WriteMessage(mt, message)
			if err != nil {
				var cerr *websocket.CloseError
				if errors.As(err, &cerr) {
					// If WS conn is closed, just return
					return
				}
				p.t.Fatalf("error writing message: %v", err)
			}
		}
	}
}
