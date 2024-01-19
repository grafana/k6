package tests

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/env"
)

func TestFramePress(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)

	p := tb.NewPage(nil)

	p.SetContent(`<input id="text1">`, nil)

	f := p.Frames()[0]

	f.Press("#text1", "Shift+KeyA", nil)
	f.Press("#text1", "KeyB", nil)
	f.Press("#text1", "Shift+KeyC", nil)

	require.Equal(t, "AbC", f.InputValue("#text1", nil))
}

func TestFrameDismissDialogBox(t *testing.T) {
	t.Parallel()

	for _, tt := range []string{
		"alert",
		"confirm",
		"prompt",
		"beforeunload",
	} {
		tt := tt
		t.Run(tt, func(t *testing.T) {
			t.Parallel()

			var (
				tb = newTestBrowser(t, withFileServer())
				p  = tb.NewPage(nil)
			)

			opts := tb.toGojaValue(struct {
				WaitUntil string `js:"waitUntil"`
			}{
				WaitUntil: "networkidle",
			})
			_, err := p.Goto(
				tb.staticURL("dialog.html?dialogType="+tt),
				opts,
			)
			require.NoError(t, err)

			if tt == "beforeunload" {
				err = p.Click("#clickHere", common.NewFrameClickOptions(p.Timeout()))
				require.NoError(t, err)
			}

			result := p.TextContent("#textField", nil)
			assert.EqualValues(t, tt+" dismissed", result)
		})
	}
}

func TestFrameNoPanicWithEmbeddedIFrame(t *testing.T) {
	t.Parallel()

	// We're skipping this when running in headless
	// environments since the bug that the test fixes
	// only surfaces when in headfull mode.
	// Remove this skip once we have headfull mode in
	// CI: https://github.com/grafana/xk6-browser/issues/678
	if env.IsBrowserHeadless() {
		t.Skip("skipped when in headless mode")
	}

	// run the browser in headfull mode.
	tb := newTestBrowser(
		t,
		withFileServer(),
		withEnvLookup(env.ConstLookup(env.BrowserHeadless, "0")),
	)

	p := tb.NewPage(nil)
	_, err := p.Goto(
		tb.staticURL("embedded_iframe.html"),
		tb.toGojaValue(struct {
			WaitUntil string `js:"waitUntil"`
		}{
			WaitUntil: "load",
		}),
	)
	require.NoError(t, err)

	result := p.TextContent("#doneDiv", nil)
	assert.EqualValues(t, "Done!", result)
}

// Without the fix in https://github.com/grafana/xk6-browser/pull/942
// this test would hang on the "sign in" link click.
func TestFrameNoPanicNavigateAndClickOnPageWithIFrames(t *testing.T) {
	t.Parallel()

	// We're skipping this when running in headless
	// environments since the bug that the test fixes
	// only surfaces when in headfull mode.
	// Remove this skip once we have headfull mode in
	// CI: https://github.com/grafana/xk6-browser/issues/678
	if env.IsBrowserHeadless() {
		t.Skip("skipped when in headless mode")
	}

	tb := newTestBrowser(
		t,
		withFileServer(),
		withEnvLookup(env.ConstLookup(env.BrowserHeadless, "0")),
	)
	p := tb.NewPage(nil)
	tb.withHandler("/iframeSignIn", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, tb.staticURL("iframe_signin.html"), http.StatusMovedPermanently)
	})

	_, err := p.Goto(tb.staticURL("iframe_home.html"), nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(tb.context(), 5*time.Second)
	defer cancel()

	err = tb.run(
		ctx,
		func() error { return p.Click(`a[href="/iframeSignIn"]`, common.NewFrameClickOptions(p.Timeout())) },
		func() error { _, err := p.WaitForNavigation(nil); return err },
	)
	require.NoError(t, err)

	result := p.TextContent("#doneDiv", nil)
	assert.EqualValues(t, "Sign In Page", result)
}

func TestFrameTitle(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	p.SetContent(`<html><head><title>Some title</title></head></html>`, nil)
	assert.Equal(t, "Some title", p.MainFrame().Title())
}
