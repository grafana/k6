package tests

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

		tb.withHandler("/ping.js", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintf(w, `
				var pingJSTextOutput = document.getElementById("pingJSText");
				pingJSTextOutput.innerText = "ping.js loaded from server";
			`)
		})

		assertHome(t, tb, p, "/home", common.LifecycleEventNetworkIdle, func() {
			result := p.TextContent("#pingJSText", nil)
			assert.EqualValues(t, "ping.js loaded from server", result)
		})
	})

	t.Run("doesn't unblock wait for networkIdle too early", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())
		p := tb.NewPage(nil)
		tb.withHandler("/home", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, tb.staticURL("prolonged_network_idle.html"), http.StatusMovedPermanently)
		})

		var counter int64
		ch := make(chan bool)
		tb.withHandler("/ping", func(w http.ResponseWriter, _ *http.Request) {
			<-ch

			var counterMu sync.Mutex
			counterMu.Lock()
			defer counterMu.Unlock()

			time.Sleep(time.Millisecond * 50)

			counter++
			fmt.Fprintf(w, "pong %d", counter)
		})

		tb.withHandler("/ping.js", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintf(w, `
				var pingJSTextOutput = document.getElementById("pingJSText");
				pingJSTextOutput.innerText = "ping.js loaded from server";
			`)
			close(ch)
		})

		assertHome(t, tb, p, "/home", common.LifecycleEventNetworkIdle, func() {
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
		tb.withHandler("/home", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, tb.staticURL("prolonged_network_idle_10.html"), http.StatusMovedPermanently)
		})

		var counterMu sync.Mutex
		var counter int64
		tb.withHandler("/ping", func(w http.ResponseWriter, _ *http.Request) {
			counterMu.Lock()
			defer counterMu.Unlock()

			time.Sleep(time.Millisecond * 50)

			counter++
			fmt.Fprintf(w, "pong %d", counter)
		})

		assertHome(t, tb, p, "/home", common.LifecycleEventNetworkIdle, func() {
			result := p.TextContent("#pingRequestText", nil)
			assert.EqualValues(t, "Waiting... pong 10 - for loop complete", result)
		})
	})
}

func assertHome(t *testing.T, tb *testBrowser, p api.Page, gotoURL string, waitUntil common.LifecycleEvent, check func()) {
	t.Helper()

	var resolved, rejected bool
	err := tb.await(func() error {
		opts := tb.toGojaValue(common.FrameGotoOptions{
			WaitUntil: waitUntil,
			Timeout:   30 * time.Second,
		})
		tb.promise(p.Goto(tb.URL(gotoURL), opts)).then(
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
