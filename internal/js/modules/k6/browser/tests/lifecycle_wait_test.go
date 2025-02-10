// practically none of this work on windows
//go:build !windows

package tests

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
)

// TODO
// Remove the promises. We don't need them anymore.

// General guidelines on lifecycle events:
//
// load
//
//   The load event is fired when the initial HTML document has been completely
//   loaded. It does not wait for the other network requests to complete.
//
//   Emitted by the browser once:
//   1. The HTML is loaded.
//   2. The async scripts have loaded.
//
// domcontentloaded
//
//   The DOMContentLoaded event is fired when the initial HTML document has been
//   completely loaded and parsed. It does not wait for the async scripts or the
//   other network requests to complete.
//
//   Emitted by the browser once:
//   1. The HTML is loaded.
//
// networkidle
//
//   The networkidle event is fired when there are no network connections for at
//   least 500ms.
//
//   Emitted by the browser once:
//   1. The HTML is loaded.
//   2. The async scripts have loaded.
//   3. All other network requests have completed.

func TestLifecycleWaitForNavigation(t *testing.T) {
	// Test description
	//
	// Steps:
	//   1. goto /home and wait for the specified lifecycle event.
	//   2. click on a link that navigates to a page, and wait on
	//    the specified lifecycle event.
	//
	// Success criteria:
	//   The click will perform a navigation away from the current page,
	//   it should wait for the specified lifecycle event and the result
	//   of the page should match the original nav.

	t.Parallel()

	tests := []struct {
		name                  string
		pingSlowness          time.Duration
		pingJSSlow            bool
		waitUntil             common.LifecycleEvent
		pingRequestTextAssert func(result string, pingCount int)
		pingJSTextAssert      func(result string)
		assertFunc            func(tb *testBrowser, p *common.Page) error
		wantError             string
	}{
		{
			name:         "load",
			pingSlowness: 100 * time.Millisecond,
			pingJSSlow:   false,
			waitUntil:    common.LifecycleEventLoad,
			pingRequestTextAssert: func(result string, pingCount int) {
				assert.NotEqualValues(t, fmt.Sprintf("Waiting... pong %d - for loop complete", pingCount), result)
			},
			pingJSTextAssert: func(result string) {
				assert.EqualValues(t, "ping.js loaded from server", result)
			},
		},
		{
			name:         "domcontentloaded",
			pingSlowness: 100 * time.Millisecond,
			pingJSSlow:   true,
			waitUntil:    common.LifecycleEventDOMContentLoad,
			pingRequestTextAssert: func(result string, pingCount int) {
				assert.NotEqualValues(t, fmt.Sprintf("Waiting... pong %d - for loop complete", pingCount), result)
			},
			pingJSTextAssert: func(result string) {
				assert.EqualValues(t, "Waiting...", result)
			},
		},
		{
			name:         "networkidle",
			pingSlowness: 0,
			pingJSSlow:   false,
			waitUntil:    common.LifecycleEventNetworkIdle,
			pingRequestTextAssert: func(result string, pingCount int) {
				assert.EqualValues(t, fmt.Sprintf("Waiting... pong %d - for loop complete", pingCount), result)
			},
			pingJSTextAssert: func(result string) {
				assert.EqualValues(t, "ping.js loaded from server", result)
			},
		},
		{
			// Test description
			//
			// Steps:
			//   1. goto /home and wait for the specified lifecycle event.
			//   2. call WaitForNavigation without clicking on the link.
			//    the specified lifecycle event.
			//
			// Success criteria:
			//   We want this test to timeout since the navigation has
			//   completed, a new one hasn't started but we "accidentally"
			//   call WaitForNavigation.
			name:         "timeout",
			pingSlowness: 0,
			pingJSSlow:   false,
			waitUntil:    common.LifecycleEventNetworkIdle,
			assertFunc: func(tb *testBrowser, p *common.Page) error {
				result, _, err := p.TextContent("#pingRequestText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				if err != nil {
					return err
				}
				assert.EqualValues(t, "Waiting... pong 10 - for loop complete", result)

				opts := &common.FrameWaitForNavigationOptions{
					Timeout:   1000 * time.Millisecond,
					WaitUntil: common.LifecycleEventNetworkIdle,
				}
				_, err = p.WaitForNavigation(opts)

				return err
			},
			wantError: "waiting for navigation: timed out after 1s",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)

			withHomeHandler(t, tb, "lifecycle.html?pingCount=10")
			withPingHandler(t, tb, tt.pingSlowness, nil)
			withPingJSHandler(t, tb, tt.pingJSSlow, nil, false)

			if tt.assertFunc != nil {
				assertHome(t, tb, p, tt.waitUntil, func() error {
					return tt.assertFunc(tb, p)
				}, nil, tt.wantError)
				return
			}

			assertHome(t, tb, p, tt.waitUntil, func() error {
				result, _, err := p.TextContent("#pingRequestText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				if err != nil {
					return err
				}
				tt.pingRequestTextAssert(result, 10)

				result, _, err = p.TextContent("#pingJSText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				if err != nil {
					return err
				}
				tt.pingJSTextAssert(result)

				waitForNav := func() error {
					opts := &common.FrameWaitForNavigationOptions{
						Timeout:   30000 * time.Millisecond,
						WaitUntil: tt.waitUntil,
					}
					_, err := p.WaitForNavigation(opts)
					return err
				}
				click := func() error {
					return p.Click(`a`, common.NewFrameClickOptions(p.Timeout()))
				}

				ctx, cancel := context.WithTimeout(tb.ctx, 5*time.Second)
				defer cancel()

				return tb.run(ctx, waitForNav, click)
			}, func() {
				result, _, err := p.TextContent("#pingRequestText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				tt.pingRequestTextAssert(result, 20)

				result, _, err = p.TextContent("#pingJSText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				tt.pingJSTextAssert(result)
			}, "")
		})
	}
}

func TestLifecycleWaitForLoadState(t *testing.T) {
	t.Parallel()

	// Test description
	//
	// Steps:
	//   1. goto /home and wait for the specified lifecycle event.
	//   2. use WaitForLoadState with the same specified lifecycle event.
	//
	// Success criteria:
	//   We want to ensure that the specified event is persisted in
	//   memory, and we don't block on WaitForLoadState.

	tests := []struct {
		name                  string
		pingSlowness          time.Duration
		pingJSSlow            bool
		waitUntil             common.LifecycleEvent
		pingRequestTextAssert func(result string)
		pingJSTextAssert      func(result string)
		assertFunc            func(p *common.Page)
	}{
		{
			name:         "load",
			pingSlowness: 100 * time.Millisecond,
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
			name:         "domcontentloaded",
			pingSlowness: 100 * time.Millisecond,
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
			// Steps:
			//   1. goto /home and wait for the domcontentloaded lifecycle event.
			//   2. use WaitForLoadState with networkidle.
			//
			// Success criteria:
			//   We want to quickly move to calling WaitForLoadState
			//   so that we wait until networkidle is received from
			//   the browser -- not relying on the persisted state in memory.
			name:         "domcontentloaded then networkidle",
			pingSlowness: 100 * time.Millisecond,
			pingJSSlow:   false,
			waitUntil:    common.LifecycleEventDOMContentLoad,
			assertFunc: func(p *common.Page) {
				err := p.WaitForLoadState(common.LifecycleEventNetworkIdle.String(), common.NewFrameWaitForLoadStateOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)

				result, _, err := p.TextContent("#pingRequestText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				assert.EqualValues(t, "Waiting... pong 10 - for loop complete", result)

				result, _, err = p.TextContent("#pingJSText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				assert.EqualValues(t, "ping.js loaded from server", result)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)

			withHomeHandler(t, tb, "lifecycle.html?pingCount=10")
			withPingHandler(t, tb, tt.pingSlowness, nil)
			withPingJSHandler(t, tb, tt.pingJSSlow, nil, false)

			if tt.assertFunc != nil {
				assertHome(t, tb, p, tt.waitUntil, func() error {
					tt.assertFunc(p)
					return nil
				}, nil, "")
				return
			}

			assertHome(t, tb, p, tt.waitUntil, func() error {
				result, _, err := p.TextContent("#pingRequestText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				tt.pingRequestTextAssert(result)

				result, _, err = p.TextContent("#pingJSText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				tt.pingJSTextAssert(result)

				// This shouldn't block and return after calling hasLifecycleEventFired.
				return p.WaitForLoadState(tt.waitUntil.String(), common.NewFrameWaitForLoadStateOptions(p.MainFrame().Timeout()))
			}, nil, "")
		})
	}
}

func TestLifecycleReload(t *testing.T) {
	t.Parallel()

	// Test description
	//
	// Steps:
	//   1. goto /home and wait for the specified lifecycle event.
	//   2. reload the page and wait for the specified lifecycle event.
	//
	// Success criteria:
	//   The resulting page after reload is the same as
	//   the initial navigation with goto.

	tests := []struct {
		name                  string
		pingSlowness          time.Duration
		pingJSSlow            bool
		waitUntil             common.LifecycleEvent
		pingRequestTextAssert func(result string, pingCount int)
		pingJSTextAssert      func(result string)
	}{
		{
			name:         "load",
			pingSlowness: 100 * time.Millisecond,
			pingJSSlow:   false,
			waitUntil:    common.LifecycleEventLoad,
			pingRequestTextAssert: func(result string, pingCount int) {
				assert.NotEqualValues(t, fmt.Sprintf("Waiting... pong %d - for loop complete", pingCount), result)
			},
			pingJSTextAssert: func(result string) {
				assert.EqualValues(t, "ping.js loaded from server", result)
			},
		},
		{
			name:         "domcontentloaded",
			pingSlowness: 100 * time.Millisecond,
			pingJSSlow:   true,
			waitUntil:    common.LifecycleEventDOMContentLoad,
			pingRequestTextAssert: func(result string, pingCount int) {
				assert.NotEqualValues(t, fmt.Sprintf("Waiting... pong %d - for loop complete", pingCount), result)
			},
			pingJSTextAssert: func(result string) {
				assert.EqualValues(t, "Waiting...", result)
			},
		},
		{
			name:         "networkidle",
			pingSlowness: 0,
			pingJSSlow:   false,
			waitUntil:    common.LifecycleEventNetworkIdle,
			pingRequestTextAssert: func(result string, pingCount int) {
				assert.EqualValues(t, fmt.Sprintf("Waiting... pong %d - for loop complete", pingCount), result)
			},
			pingJSTextAssert: func(result string) {
				assert.EqualValues(t, "ping.js loaded from server", result)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)

			withHomeHandler(t, tb, "lifecycle.html?pingCount=10")
			withPingHandler(t, tb, tt.pingSlowness, nil)
			withPingJSHandler(t, tb, tt.pingJSSlow, nil, false)

			assertHome(t, tb, p, tt.waitUntil, func() error {
				result, _, err := p.TextContent("#pingRequestText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				if err != nil {
					return err
				}
				tt.pingRequestTextAssert(result, 10)

				result, _, err = p.TextContent("#pingJSText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				if err != nil {
					return err
				}
				tt.pingJSTextAssert(result)

				opts := &common.PageReloadOptions{
					WaitUntil: tt.waitUntil,
					Timeout:   30 * time.Second,
				}
				_, err = p.Reload(opts)
				require.NoError(t, err)

				result, _, err = p.TextContent("#pingRequestText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				if err != nil {
					return err
				}
				tt.pingRequestTextAssert(result, 20)

				result, _, err = p.TextContent("#pingJSText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				if err != nil {
					return err
				}
				tt.pingJSTextAssert(result)

				return nil
			}, nil, "")
		})
	}
}

func TestLifecycleGotoWithSubFrame(t *testing.T) {
	t.Parallel()

	// Test description
	//
	// Steps:
	//   1. goto /home (with iframe to /sub) and wait for the specified lifecycle event.
	//
	// Success criteria:
	//   The web page (all frames) is in the expected state
	//   once we receive the specified lifecycle event from
	//   the browser.

	tests := []struct {
		name                  string
		pingSlowness          time.Duration
		pingJSSlow            bool
		waitUntil             common.LifecycleEvent
		pingRequestTextAssert func(result string)
		pingJSTextAssert      func(result string)
	}{
		{
			name:         "load",
			pingSlowness: 100 * time.Millisecond,
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
			name:         "domcontentloaded",
			pingSlowness: 100 * time.Millisecond,
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
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)

			withHomeHandler(t, tb, "lifecycle_main_frame.html")
			withSubHandler(t, tb, "lifecycle.html?pingCount=10")
			withPingHandler(t, tb, tt.pingSlowness, nil)
			withPingJSHandler(t, tb, tt.pingJSSlow, nil, true)

			assertHome(t, tb, p, tt.waitUntil, func() error {
				result, _, err := p.TextContent("#subFramePingRequestText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				if err != nil {
					return err
				}
				tt.pingRequestTextAssert(result)

				result, _, err = p.TextContent("#subFramePingJSText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				if err != nil {
					return err
				}
				tt.pingJSTextAssert(result)

				return nil
			}, nil, "")
		})
	}
}

func TestLifecycleGoto(t *testing.T) {
	t.Parallel()

	// Test description
	//
	// Steps:
	//   1. goto /home and wait for the specified lifecycle event.
	//
	// Success criteria:
	//   The web page is in the expected state once we receive
	//   the specified lifecycle event from the browser.

	tests := []struct {
		name                  string
		pingSlowness          time.Duration
		pingJSSlow            bool
		waitUntil             common.LifecycleEvent
		pingRequestTextAssert func(result string)
		pingJSTextAssert      func(result string)
	}{
		{
			name:         "load",
			pingSlowness: 100 * time.Millisecond,
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
			name:         "domcontentloaded",
			pingSlowness: 100 * time.Millisecond,
			pingJSSlow:   true,
			waitUntil:    common.LifecycleEventDOMContentLoad,
			pingRequestTextAssert: func(result string) {
				assert.NotEqualValues(t, "Waiting... pong 10 - for loop complete", result)
			},
			pingJSTextAssert: func(result string) {
				assert.EqualValues(t, "Waiting...", result)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)

			withHomeHandler(t, tb, "lifecycle.html?pingCount=10")
			withPingHandler(t, tb, tt.pingSlowness, nil)
			withPingJSHandler(t, tb, tt.pingJSSlow, nil, false)

			assertHome(t, tb, p, tt.waitUntil, func() error {
				result, _, err := p.TextContent("#pingRequestText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				if err != nil {
					return err
				}
				tt.pingRequestTextAssert(result)

				result, _, err = p.TextContent("#pingJSText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				if err != nil {
					return err
				}
				tt.pingJSTextAssert(result)

				return nil
			}, nil, "")
		})
	}
}

func TestLifecycleGotoNetworkIdle(t *testing.T) {
	t.Parallel()

	t.Run("doesn't timeout waiting for networkIdle", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withHTTPServer())
		p := tb.NewPage(nil)
		tb.withHandler("/home", func(w http.ResponseWriter, _ *http.Request) {
			_, err := fmt.Fprintf(w, `
			<html>
				<head></head>
				<body>
					<div id="pingJSText">Waiting...</div>
					<script src="/ping.js" async></script>
				</body>
			</html>
			`)
			require.NoError(t, err)
		})

		withPingJSHandler(t, tb, false, nil, false)

		assertHome(t, tb, p, common.LifecycleEventNetworkIdle, func() error {
			result, _, err := p.TextContent("#pingJSText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
			if err != nil {
				return err
			}
			assert.EqualValues(t, "ping.js loaded from server", result)

			return nil
		}, nil, "")
	})

	t.Run("doesn't unblock wait for networkIdle too early", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())
		p := tb.NewPage(nil)

		withHomeHandler(t, tb, "lifecycle.html?pingCount=4")
		ch := make(chan bool)
		withPingHandler(t, tb, 50*time.Millisecond, ch)
		withPingJSHandler(t, tb, false, ch, false)

		assertHome(t, tb, p, common.LifecycleEventNetworkIdle, func() error {
			result, _, err := p.TextContent("#pingRequestText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
			if err != nil {
				return err
			}
			assert.EqualValues(t, "Waiting... pong 4 - for loop complete", result)

			result, _, err = p.TextContent("#pingJSText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
			if err != nil {
				return err
			}
			assert.EqualValues(t, "ping.js loaded from server", result)

			return nil
		}, nil, "")
	})

	t.Run("doesn't unblock wait on networkIdle early when load and domcontentloaded complete at once", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())
		p := tb.NewPage(nil)

		withHomeHandler(t, tb, "lifecycle_no_ping_js.html")
		withPingHandler(t, tb, 50*time.Millisecond, nil)

		assertHome(t, tb, p, common.LifecycleEventNetworkIdle, func() error {
			result, _, err := p.TextContent("#pingRequestText", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
			if err != nil {
				return err
			}
			assert.EqualValues(t, "Waiting... pong 10 - for loop complete", result)

			return nil
		}, nil, "")
	})
}

func withHomeHandler(t *testing.T, tb *testBrowser, htmlFile string) {
	t.Helper()

	tb.withHandler("/home", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, tb.staticURL(htmlFile), http.StatusMovedPermanently)
	})
}

func withSubHandler(t *testing.T, tb *testBrowser, htmlFile string) {
	t.Helper()

	tb.withHandler("/sub", func(w http.ResponseWriter, r *http.Request) {
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
		_, err := fmt.Fprintf(w, "pong %d", counter)
		require.NoError(t, err)
	})
}

func withPingJSHandler(t *testing.T, tb *testBrowser, slow bool, ch chan bool, withSubFrame bool) {
	t.Helper()

	tb.withHandler("/ping.js", func(w http.ResponseWriter, _ *http.Request) {
		script := `
			var pingJSTextOutput = document.getElementById("pingJSText");
			pingJSTextOutput.innerText = "ping.js loaded from server";
		`
		if withSubFrame {
			script += `

			var parentOutputServerMsg = window.parent.document.getElementById('subFramePingJSText');
			parentOutputServerMsg.innerText = pingJSTextOutput.innerText;
			`
		}
		if slow {
			script = `
			await new Promise(resolve => setTimeout(resolve, 1000));

			` + script
		}
		_, err := fmt.Fprint(w, script)
		require.NoError(t, err)

		if ch != nil {
			close(ch)
		}
	})
}

func assertHome(
	t *testing.T,
	tb *testBrowser, p *common.Page,
	waitUntil common.LifecycleEvent,
	check func() error, secondCheck func(), wantError string,
) {
	t.Helper()

	var resolved, rejected bool
	err := func() error {
		opts := &common.FrameGotoOptions{
			WaitUntil: waitUntil,
			Timeout:   common.DefaultTimeout,
		}
		_, err := p.Goto(tb.url("/home"), opts)
		if err == nil {
			resolved = true
		} else {
			rejected = true
		}

		err = check()
		if err != nil {
			return err
		}

		if secondCheck != nil {
			secondCheck()
		}

		return nil
	}()

	if wantError != "" {
		require.EqualError(t, err, wantError)
		return
	}

	require.NoError(t, err)

	assert.True(t, resolved)
	assert.False(t, rejected)
}
