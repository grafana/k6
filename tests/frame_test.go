package tests

import (
	"testing"

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
