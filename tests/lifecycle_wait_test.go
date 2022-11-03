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

func TestLifecycleWaitForLoadStateLoad(t *testing.T) {
	// Test description
	//
	// 1. goto /home and wait for the load lifecycle event.
	// 2. use WaitForLoadState with load to ensure that load
	//    lifecycle event has already fired.
	//
	// Success criteria: We don't wait for all network requests to
	//                   complete, but we are interested in waiting
	//                   for all async scripts to have fully loaded
	//                   (which is when load is fired). We also want
	//                   to ensure that the load event is stored
	//                   internally, and we don't block on WaitForLoadState.

	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	tb.withHandler("/home", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, tb.staticURL("wait_for_nav_lifecycle.html"), http.StatusMovedPermanently)
	})

	var counter int64
	var counterMu sync.Mutex
	tb.withHandler("/ping", func(w http.ResponseWriter, _ *http.Request) {
		counterMu.Lock()
		defer counterMu.Unlock()

		time.Sleep(time.Millisecond * 100)

		counter++
		fmt.Fprintf(w, "pong %d", counter)
	})

	tb.withHandler("/ping.js", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `
				var pingJSTextOutput = document.getElementById("pingJSText");
				pingJSTextOutput.innerText = "ping.js loaded from server";
			`)
	})

	waitUntil := common.LifecycleEventLoad
	assertHome(t, tb, p, waitUntil, func() {
		result := p.TextContent("#pingRequestText", nil)
		assert.NotEqualValues(t, "Waiting... pong 10 - for loop complete", result)

		result = p.TextContent("#pingJSText", nil)
		assert.EqualValues(t, "ping.js loaded from server", result)

		// This shouldn't block and return after calling hasLifecycleEventFired.
		p.WaitForLoadState(waitUntil.String(), nil)
	})
}

func TestLifecycleReloadLoad(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	tb.withHandler("/home", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, tb.staticURL("reload_lifecycle.html"), http.StatusMovedPermanently)
	})

	var counter int64
	var counterMu sync.Mutex
	tb.withHandler("/ping", func(w http.ResponseWriter, _ *http.Request) {
		counterMu.Lock()
		defer counterMu.Unlock()

		time.Sleep(time.Millisecond * 100)

		counter++
		fmt.Fprintf(w, "pong %d", counter)
	})

	tb.withHandler("/ping.js", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `
				var pingJSTextOutput = document.getElementById("pingJSText");
				pingJSTextOutput.innerText = "ping.js loaded from server";
			`)
	})

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
	tb.withHandler("/home", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, tb.staticURL("reload_lifecycle.html"), http.StatusMovedPermanently)
	})

	var counter int64
	var counterMu sync.Mutex
	tb.withHandler("/ping", func(w http.ResponseWriter, _ *http.Request) {
		counterMu.Lock()
		defer counterMu.Unlock()

		time.Sleep(time.Millisecond * 100)

		counter++
		fmt.Fprintf(w, "pong %d", counter)
	})

	tb.withHandler("/ping.js", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `
				await new Promise(resolve => setTimeout(resolve, 1000));

				var pingJSTextOutput = document.getElementById("pingJSText");
				pingJSTextOutput.innerText = "ping.js loaded from server";
			`)
	})

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
	tb.withHandler("/home", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, tb.staticURL("reload_lifecycle.html"), http.StatusMovedPermanently)
	})

	var counter int64
	var counterMu sync.Mutex
	tb.withHandler("/ping", func(w http.ResponseWriter, _ *http.Request) {
		counterMu.Lock()
		defer counterMu.Unlock()

		counter++
		fmt.Fprintf(w, "pong %d", counter)
	})

	tb.withHandler("/ping.js", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `
				var pingJSTextOutput = document.getElementById("pingJSText");
				pingJSTextOutput.innerText = "ping.js loaded from server";
			`)
	})

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

		tb.withHandler("/ping.js", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintf(w, `
				var pingJSTextOutput = document.getElementById("pingJSText");
				pingJSTextOutput.innerText = "ping.js loaded from server";
			`)
		})

		assertHome(t, tb, p, common.LifecycleEventNetworkIdle, func() {
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
		var counterMu sync.Mutex
		tb.withHandler("/ping", func(w http.ResponseWriter, _ *http.Request) {
			<-ch

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

		assertHome(t, tb, p, common.LifecycleEventNetworkIdle, func() {
			result := p.TextContent("#pingRequestText", nil)
			assert.EqualValues(t, "Waiting... pong 10 - for loop complete", result)
		})
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
