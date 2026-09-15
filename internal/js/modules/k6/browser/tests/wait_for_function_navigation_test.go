package tests

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

func TestWaitForFunctionPredicateErrors(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t)
	p := b.NewPage(nil)
	for _, tc := range []struct{ predicate, want string }{
		{`() => { throw new Error('persistent predicate failure'); }`, "persistent predicate failure"},
		{`() => { throw 'Inspected target navigated or closed'; }`, "Inspected target navigated or closed"},
		{`() => { throw 'Cannot find context with specified id'; }`, "Cannot find context with specified id"},
		{`() => { throw 'Execution context was destroyed'; }`, "Execution context was destroyed"},
		{`(() => { throw 'Cannot find context with specified id'; })()`, "Cannot find context with specified id"},
	} {
		t.Run(tc.predicate, func(t *testing.T) {
			t.Parallel()

			started := time.Now()
			_, err := p.MainFrame().WaitForFunction(tc.predicate, common.NewFrameWaitForFunctionOptions(3*time.Second))
			require.ErrorContains(t, err, tc.want)
			require.NotContains(t, err.Error(), "timed out")
			require.Less(t, time.Since(started), time.Second)
			t.Logf("preserved error: %v", err)
		})
	}
}

func TestWaitForFunctionStops(t *testing.T) {
	t.Parallel()

	for _, action := range []string{"close", "detach", "cancel"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()

			b := newTestBrowser(t)
			p := b.NewPage(nil)
			frame := p.MainFrame()
			if action == "detach" {
				require.NoError(t, p.SetContent(`<iframe srcdoc="<body>child</body>"></iframe>`, nil))
				require.Eventually(t, func() bool { return len(p.Frames()) == 2 }, time.Second, 10*time.Millisecond)
				for _, f := range p.Frames() {
					if f != p.MainFrame() {
						frame = f
					}
				}
			}
			started := make(chan struct{}, 1)
			require.NoError(t, p.On(common.PageEventConsole, func(common.PageEvent) error {
				select {
				case started <- struct{}{}:
				default:
				}
				return nil
			}))
			done := make(chan error, 1)
			go func() {
				_, err := frame.WaitForFunction(`() => { console.log('waiting'); return false; }`,
					common.NewFrameWaitForFunctionOptions(5*time.Second))
				done <- err
			}()
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("predicate did not start")
			}
			switch action {
			case "close":
				require.NoError(t, p.Close())
			case "detach":
				_, err := p.Evaluate(`() => document.querySelector('iframe').remove()`)
				require.NoError(t, err)
			case "cancel":
				b.cancelContext()
			}
			select {
			case err := <-done:
				require.Error(t, err)
				require.NotContains(t, err.Error(), "timed out")
				if action == "cancel" {
					require.ErrorIs(t, err, context.Canceled)
				}
				t.Logf("%s stopped wait: %v", action, err)
			case <-time.After(time.Second):
				t.Fatal("wait did not stop after " + action)
			}
			if action == "detach" {
				_, err := frame.WaitForFunction(`() => true`, common.NewFrameWaitForFunctionOptions(time.Second))
				require.ErrorIs(t, err, common.ErrFrameDetached)
			}
		})
	}
}

func TestWaitForFunctionNavigation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		predicate string
		timeout   time.Duration
		wantError bool
	}{
		{"reload_timeout", `() => false`, 4 * time.Second, true},
		{"new_document", `(text, origin) => performance.timeOrigin !== origin && document.body.textContent.trim() === text && document.body`, 4 * time.Second, false},
		{"repeated_navigation_budget", `() => {
			if (!window.scheduled) {
				window.scheduled = true;
				sessionStorage.reloads = Number(sessionStorage.reloads || 0) + 1;
				setTimeout(() => location.reload(), 250);
			}
			return false;
		}`, 2 * time.Second, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := newTestBrowser(t, withHTTPServer())
			b.withHandler("/wait", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				_, err := w.Write([]byte("<!doctype html><html><body>local fixture</body></html>"))
				require.NoError(t, err)
			})
			p := b.GotoNewPage(b.url("/wait"))
			origin, err := p.Evaluate(`() => {
				setTimeout(() => location.reload(), 250);
				return performance.timeOrigin;
			}`)
			require.NoError(t, err)

			started := time.Now()
			result, err := p.WaitForFunction(tc.predicate, common.NewFrameWaitForFunctionOptions(tc.timeout), "local fixture", origin)
			elapsed := time.Since(started)
			t.Logf("elapsed=%s error=%v", elapsed, err)
			if tc.wantError {
				require.ErrorContains(t, err, "timed out")
				require.GreaterOrEqual(t, elapsed, tc.timeout-100*time.Millisecond)
				require.Less(t, elapsed, tc.timeout+time.Second, "navigation must not restart the timeout")
			} else {
				require.NoError(t, err)
				handle, ok := result.(common.JSHandleAPI)
				require.True(t, ok, "the new document's element handle must survive the wait")
				text, err := handle.Evaluate(`element => element.textContent.trim()`)
				require.NoError(t, err)
				require.Equal(t, "local fixture", text)
				require.NoError(t, handle.Dispose())
			}
			if tc.name == "repeated_navigation_budget" {
				reloads, err := p.Evaluate(`() => Number(sessionStorage.reloads)`)
				require.NoError(t, err)
				require.GreaterOrEqual(t, reloads, float64(3))
			}
			ready, err := p.Evaluate(`() => document.body.textContent.trim()`)
			require.NoError(t, err)
			require.Equal(t, "local fixture", ready)
		})
	}
}
