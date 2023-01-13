package tests

import (
	"os"
	"strconv"
	"testing"

	"github.com/dop251/goja"
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

	tests := []string{
		"alert",
		"confirm",
		"prompt",
		"beforeunload",
	}

	for _, test := range tests {
		test := test
		t.Run(test, func(t *testing.T) {
			t.Parallel()

			b := newTestBrowser(t, withFileServer())

			p := b.NewPage(nil)

			err := b.await(func() error {
				opts := b.toGojaValue(struct {
					WaitUntil string `js:"waitUntil"`
				}{
					WaitUntil: "networkidle",
				})
				pageGoto := p.Goto(
					b.staticURL("dialog.html?dialogType="+test),
					opts,
				)
				b.promise(pageGoto).then(func() *goja.Promise {
					if test == "beforeunload" {
						return p.Click("#clickHere", nil)
					}

					result := p.TextContent("#textField", nil)
					assert.EqualValues(t, test+" dismissed", result)

					return nil
				}).then(func() {
					result := p.TextContent("#textField", nil)
					assert.EqualValues(t, test+" dismissed", result)
				})

				return nil
			})
			require.NoError(t, err)
		})
	}
}

func TestFrameNoPanicWithEmbeddedIFrame(t *testing.T) {
	if strValue, ok := os.LookupEnv("XK6_HEADLESS"); ok {
		if value, err := strconv.ParseBool(strValue); err == nil && value {
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
	b := newTestBrowser(t, withFileServer(), opts)
	p := b.NewPage(nil)

	var result string
	err := b.await(func() error {
		opts := b.toGojaValue(struct {
			WaitUntil string `js:"waitUntil"`
		}{
			WaitUntil: "load",
		})
		pageGoto := p.Goto(
			b.staticURL("embedded_iframe.html"),
			opts,
		)

		b.promise(pageGoto).
			then(func() {
				result = p.TextContent("#doneDiv", nil)
			})

		return nil
	})
	require.NoError(t, err)

	assert.EqualValues(t, "Done!", result)
}
