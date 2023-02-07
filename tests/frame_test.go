package tests

import (
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFramePress(t *testing.T) {
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
				err = p.Click("#clickHere", nil)
				require.NoError(t, err)
			}

			result := p.TextContent("#textField", nil)
			assert.EqualValues(t, tt+" dismissed", result)
		})
	}
}

// FIX
// This test does not work on my machine. It fails with:
// "" != "Done!".
//
// OSX: 13.1 (22C65).
func TestFrameNoPanicWithEmbeddedIFrame(t *testing.T) {
	if s, ok := os.LookupEnv("XK6_HEADLESS"); ok {
		if v, err := strconv.ParseBool(s); err == nil && v {
			// We're skipping this when running in headless
			// environments since the bug that the test fixes
			// only surfaces when in headfull mode.
			// Remove this skip once we have headfull mode in
			// CI: https://github.com/grafana/xk6-browser/issues/678
			t.Skip("skipped when in headless mode")
		}
	}

	t.Parallel()

	opts := defaultLaunchOpts()
	opts.Headless = false
	tb := newTestBrowser(t, withFileServer(), opts)
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

func TestFrameTitle(t *testing.T) {
	p := newTestBrowser(t).NewPage(nil)
	p.SetContent(`<html><head><title>Some title</title></head></html>`, nil)
	assert.Equal(t, "Some title", p.MainFrame().Title())
}
