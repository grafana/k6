package tests

import (
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
)

func TestLocatorClick(t *testing.T) {
	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/strict_link.html"), nil))

	// Selecting a single element and clicking on it is OK.
	t.Run("ok", func(t *testing.T) {
		result := func() bool {
			cr := p.Evaluate(tb.toGojaValue(`() => window.result`))
			return cr.(goja.Value).ToBoolean() //nolint:forcetypeassert
		}
		link := p.Locator("#link", nil)
		link.Click(nil)
		require.True(t, result(), "could not click the link")
	})
	// There are two links in the document (strict_link.html).
	// The strict mode should disallow selecting multiple elements.
	t.Run("strict", func(t *testing.T) {
		link := p.Locator("a", nil)
		require.Panics(t, func() { link.Click(nil) })
	})
}
