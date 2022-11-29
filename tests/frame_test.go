package tests

import (
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
				b.promise(p.Goto(b.staticURL("dialog.html?dialogType="+tt.name), opts)).then(func() {
					result := p.TextContent("#text", nil)
					assert.EqualValues(t, "Hello World", result)
				})

				return nil
			})
			require.NoError(t, err)
		})
	}
}
