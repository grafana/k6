package tests

import (
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

	tests := []struct {
		name string
	}{
		{
			name: "alert",
		},
		{
			name: "confirm",
		},
		{
			name: "prompt",
		},
		{
			name: "beforeunload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
					b.staticURL("dialog.html?dialogType="+tt.name),
					opts,
				)
				b.promise(pageGoto).then(func() *goja.Promise {
					if tt.name == "beforeunload" {
						return p.Click("#clickHere", nil)
					}

					result := p.TextContent("#textField", nil)
					assert.EqualValues(t, tt.name+" dismissed", result)

					return nil
				}).then(func() {
					result := p.TextContent("#textField", nil)
					assert.EqualValues(t, tt.name+" dismissed", result)
				})

				return nil
			})
			require.NoError(t, err)
		})
	}
}
