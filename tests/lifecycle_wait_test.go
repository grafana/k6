package tests

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/common"
)

// General guidelines on lifecycle events:
//
// - load: This lifecycle event is emitted by the browser once:
//            1. The HTML is loaded;
//            2. The async scripts have loaded;
//         It does not wait for the other network requests to
//         complete.
//
// - domcontentloaded: This lifecycle event is emitted by the
//                     browser once:
//                         1. The HTML is loaded;
//                     It does not wait for the async scripts or
//                     the other network requests to complete.
//
// - networkidle: This lifecycle event is emitted by the browser once:
//            1. The HTML is loaded;
//            2. The async scripts have loaded;
//            3. All other network requests have completed;

func TestLifecycleWaitForLoadState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		pingSlowness          time.Duration
		pingJSSlow            bool
		waitUntil             common.LifecycleEvent
		pingRequestTextAssert func(result string)
		pingJSTextAssert      func(result string)
		assertFunc            func(p api.Page)
	}{
		{
			// Test description
			//
			// 1. goto /home and wait for the load lifecycle event.
			// 2. use WaitForLoadState with load.
			//
			// Success criteria: We want to ensure that the load event is persisted in
			//                   memory, and we don't block on WaitForLoadState.
			name:         "load",
			pingSlowness: time.Millisecond * 100,
			pingJSSlow:   false,
			waitUntil:    common.LifecycleEventLoad,
			pingRequestTextAssert: func(result string) {
				assert.NotEqualValues(t, "Waiting... pong 10 - for loop complete", result)
			},
			pingJSTextAssert: func(result string) {
				assert.EqualValues(t, "ping.js loaded from server", result)
			},
		},
		{
			// Test description
			//
			// 1. goto /home and wait for the domcontentloaded lifecycle event.
			// 2. use WaitForLoadState with domcontentloaded.
			//
			// Success criteria: We want to ensure that the domcontentloaded event is
			//                   persisted in memory, and we don't block on WaitForLoadState.
			name:         "domcontentloaded",
			pingSlowness: time.Millisecond * 100,
			pingJSSlow:   true,
			waitUntil:    common.LifecycleEventDOMContentLoad,
			pingRequestTextAssert: func(result string) {
				assert.NotEqualValues(t, "Waiting... pong 10 - for loop complete", result)
			},
			pingJSTextAssert: func(result string) {
				assert.EqualValues(t, "Waiting...", result)
			},
		},
		{
			// Test description
			//
			// 1. goto /home and wait for the networkidle lifecycle event.
			// 2. use WaitForLoadState with networkidle.
			//
			// Success criteria: We want to ensure that the networkidle event is
			//                   persisted in memory, and we don't block on WaitForLoadState.
			name:         "networkidle",
			pingSlowness: 0,
			pingJSSlow:   false,
			waitUntil:    common.LifecycleEventNetworkIdle,
			pingRequestTextAssert: func(result string) {
				assert.EqualValues(t, "Waiting... pong 10 - for loop complete", result)
			},
			pingJSTextAssert: func(result string) {
				assert.EqualValues(t, "ping.js loaded from server", result)
			},
		},
		{
			// Test description
			//
			// 1. goto /home and wait for the domcontentloaded lifecycle event.
			// 2. use WaitForLoadState with networkidle.
			//
			// Success criteria: We want to quickly move to calling WaitForLoadState
			//                   so that we wait until networkidle is received from
			//                   the browser. So not relying on the persisted state in memory.
			name:         "domcontentloaded then networkidle",
			pingSlowness: time.Millisecond * 100,
			pingJSSlow:   false,
			waitUntil:    common.LifecycleEventDOMContentLoad,
			assertFunc: func(p api.Page) {
				p.WaitForLoadState(common.LifecycleEventNetworkIdle.String(), nil)

				result := p.TextContent("#pingRequestText", nil)
				assert.EqualValues(t, "Waiting... pong 10 - for loop complete", result)

				result = p.TextContent("#pingJSText", nil)
				assert.EqualValues(t, "ping.js loaded from server", result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)

			withHomeHandler(t, tb, "wait_for_nav_lifecycle.html")
			withPingHandler(t, tb, tt.pingSlowness, nil)
			withPingJSHandler(t, tb, tt.pingJSSlow, nil)

			if tt.assertFunc != nil {
				assertHome(t, tb, p, tt.waitUntil, func() { tt.assertFunc(p) })
				return
			}

			assertHome(t, tb, p, tt.waitUntil, func() {
				result := p.TextContent("#pingRequestText", nil)
				tt.pingRequestTextAssert(result)

				result = p.TextContent("#pingJSText", nil)
				tt.pingJSTextAssert(result)

				// This shouldn't block and return after calling hasLifecycleEventFired.
				p.WaitForLoadState(tt.waitUntil.String(), nil)
			})
		})
	}
}

func TestLifecycleReloadLoad(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)

	withHomeHandler(t, tb, "reload_lifecycle.html")
	withPingHandler(t, tb, time.Millisecond*100, nil)
	withPingJSHandler(t, tb, false, nil)

	waitUntil := common.LifecycleEventLoad
	assertHome(t, tb, p, waitUntil, func() {
		result := p.TextContent("#pingRequestText", nil)
		assert.NotEqualValues(t, "Waiting... pong 10 - for loop complete", result)

		result = p.TextContent("#pingJSText", nil)
		assert.EqualValues(t, "ping.js loaded from server", result)

		opts := tb.toGojaValue(common.PageReloadOptions{
			WaitUntil: waitUntil,
			Timeout:   30 * time.Second,
		})
		p.Reload(opts)

		result = p.TextContent("#pingRequestText", nil)
		assert.NotEqualValues(t, "Waiting... pong 20 - for loop complete", result)

		result = p.TextContent("#pingJSText", nil)
		assert.EqualValues(t, "ping.js loaded from server", result)
	})
}

func TestLifecycleReloadDOMContentLoaded(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)

	withHomeHandler(t, tb, "reload_lifecycle.html")
	withPingHandler(t, tb, time.Millisecond*100, nil)
	withPingJSHandler(t, tb, true, nil)

	waitUntil := common.LifecycleEventDOMContentLoad
	assertHome(t, tb, p, waitUntil, func() {
		result := p.TextContent("#pingRequestText", nil)
		assert.NotEqualValues(t, "Waiting... pong 10 - for loop complete", result)

		result = p.TextContent("#pingJSText", nil)
		assert.EqualValues(t, "Waiting...", result)

		opts := tb.toGojaValue(common.PageReloadOptions{
			WaitUntil: waitUntil,
			Timeout:   30 * time.Second,
		})
		p.Reload(opts)

		result = p.TextContent("#pingRequestText", nil)
		assert.NotEqualValues(t, "Waiting... pong 20 - for loop complete", result)

		result = p.TextContent("#pingJSText", nil)
		assert.EqualValues(t, "Waiting...", result)
	})
}

func TestLifecycleReloadNetworkIdle(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)

	withHomeHandler(t, tb, "reload_lifecycle.html")
	withPingHandler(t, tb, 0, nil)
	withPingJSHandler(t, tb, false, nil)

	waitUntil := common.LifecycleEventNetworkIdle
	assertHome(t, tb, p, waitUntil, func() {
		result := p.TextContent("#pingRequestText", nil)
		assert.EqualValues(t, "Waiting... pong 10 - for loop complete", result)

		result = p.TextContent("#pingJSText", nil)
		assert.EqualValues(t, "ping.js loaded from server", result)

		opts := tb.toGojaValue(common.PageReloadOptions{
			WaitUntil: waitUntil,
			Timeout:   30 * time.Second,
		})
		p.Reload(opts)

		result = p.TextContent("#pingRequestText", nil)
		assert.EqualValues(t, "Waiting... pong 20 - for loop complete", result)

		result = p.TextContent("#pingJSText", nil)
		assert.EqualValues(t, "ping.js loaded from server", result)
	})
}

func TestLifecycleNetworkIdle(t *testing.T) {
	t.Parallel()

	t.Run("doesn't timeout waiting for networkIdle", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withHTTPServer())
		p := tb.NewPage(nil)
		tb.withHandler("/home", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintf(w, `
			<html>
				<head></head>
				<body>
					<div id="pingJSText">Waiting...</div>
					<script src="/ping.js" async></script>
				</body>
			</html>
			`)
		})

		withPingJSHandler(t, tb, false, nil)

		assertHome(t, tb, p, common.LifecycleEventNetworkIdle, func() {
			result := p.TextContent("#pingJSText", nil)
			assert.EqualValues(t, "ping.js loaded from server", result)
		})
	})

	t.Run("doesn't unblock wait for networkIdle too early", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())
		p := tb.NewPage(nil)

		withHomeHandler(t, tb, "prolonged_network_idle.html")
		ch := make(chan bool)
		withPingHandler(t, tb, time.Millisecond*50, ch)
		withPingJSHandler(t, tb, false, ch)

		assertHome(t, tb, p, common.LifecycleEventNetworkIdle, func() {
			result := p.TextContent("#pingRequestText", nil)
			assert.EqualValues(t, "Waiting... pong 4 - for loop complete", result)

			result = p.TextContent("#pingJSText", nil)
			assert.EqualValues(t, "ping.js loaded from server", result)
		})
	})

	t.Run("doesn't unblock wait on networkIdle early when load and domcontentloaded complete at once", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())
		p := tb.NewPage(nil)

		withHomeHandler(t, tb, "prolonged_network_idle_10.html")
		withPingHandler(t, tb, time.Millisecond*50, nil)

		assertHome(t, tb, p, common.LifecycleEventNetworkIdle, func() {
			result := p.TextContent("#pingRequestText", nil)
			assert.EqualValues(t, "Waiting... pong 10 - for loop complete", result)
		})
	})
}

func withHomeHandler(t *testing.T, tb *testBrowser, htmlFile string) {
	t.Helper()

	tb.withHandler("/home", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, tb.staticURL(htmlFile), http.StatusMovedPermanently)
	})
}

func withPingHandler(t *testing.T, tb *testBrowser, slow time.Duration, ch chan bool) {
	t.Helper()

	var counter int64
	var counterMu sync.Mutex
	tb.withHandler("/ping", func(w http.ResponseWriter, _ *http.Request) {
		if ch != nil {
			<-ch
		}

		counterMu.Lock()
		defer counterMu.Unlock()

		time.Sleep(slow)

		counter++
		fmt.Fprintf(w, "pong %d", counter)
	})
}

func withPingJSHandler(t *testing.T, tb *testBrowser, slow bool, ch chan bool) {
	t.Helper()

	tb.withHandler("/ping.js", func(w http.ResponseWriter, _ *http.Request) {
		script := `
			var pingJSTextOutput = document.getElementById("pingJSText");
			pingJSTextOutput.innerText = "ping.js loaded from server";
		`
		if slow {
			script = `
			await new Promise(resolve => setTimeout(resolve, 1000));

			` + script
		}
		fmt.Fprint(w, script)

		if ch != nil {
			close(ch)
		}
	})
}

func assertHome(
	t *testing.T,
	tb *testBrowser,
	p api.Page,
	waitUntil common.LifecycleEvent,
	check func(),
) {
	t.Helper()

	var resolved, rejected bool
	err := tb.await(func() error {
		opts := tb.toGojaValue(common.FrameGotoOptions{
			WaitUntil: waitUntil,
			Timeout:   30 * time.Second,
		})
		tb.promise(p.Goto(tb.URL("/home"), opts)).then(
			func() {
				check()
				resolved = true
			},
			func() {
				rejected = true
			},
		)

		return nil
	})
	require.NoError(t, err)

	assert.True(t, resolved)
	assert.False(t, rejected)
}
