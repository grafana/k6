package tests

import (
	"testing"

	"github.com/grafana/xk6-browser/api"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Strict mode:
// All operations on locators throw an exception if more
// than one element matches the locator's selector.

// Note:
// We skip adding t.Parallel to subtests because goja or our code might race.

func TestLocatorClick(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	// Selecting a single element and clicking on it is OK.
	p.Locator("#link", nil).Click(nil)
	require.True(t, tb.asGojaBool(p.Evaluate(tb.toGojaValue(`() => window.result`))), "could not click the link")
}

func TestLocatorDblclick(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	p.Locator("#link", nil).Dblclick(nil)
	require.True(t, tb.asGojaBool(p.Evaluate(tb.toGojaValue(`() => window.dblclick`))), "could not double click the link")
}

//nolint:tparallel
func TestLocatorCheck(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	t.Run("check", func(t *testing.T) {
		check := func() bool {
			return tb.asGojaBool(p.Evaluate(tb.toGojaValue(`() => window.check`)))
		}

		l := p.Locator("#inputCheckbox", nil)
		require.False(t, check(), "should be unchecked first")

		l.Check(nil)
		require.True(t, check(), "could not check the input box")

		l.Uncheck(nil)
		require.False(t, check(), "could not uncheck the input box")
	})
	t.Run("is_checked", func(t *testing.T) {
		l := p.Locator("#inputCheckbox", nil)

		l.Check(nil)
		require.True(t, l.IsChecked(nil))

		l.Uncheck(nil)
		require.False(t, l.IsChecked(nil))
	})
}

//nolint:tparallel
func TestLocatorElementState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state, eval string
		query       func(api.Locator) bool
	}{
		{
			"readOnly",
			`() => document.getElementById('inputText').readOnly = true`,
			func(l api.Locator) bool { return l.IsEditable(nil) },
		},
		{
			"enabled",
			`() => document.getElementById('inputText').disabled = true`,
			func(l api.Locator) bool { return l.IsEnabled(nil) },
		},
		{
			"disabled",
			`() => document.getElementById('inputText').disabled = true`,
			func(l api.Locator) bool { return !l.IsDisabled(nil) },
		},
		{
			"visible",
			`() => document.getElementById('inputText').style.visibility = 'hidden'`,
			func(l api.Locator) bool { return l.IsVisible(nil) },
		},
		{
			"hidden",
			`() => document.getElementById('inputText').style.visibility = 'hidden'`,
			func(l api.Locator) bool { return !l.IsHidden(nil) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)
			require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

			l := p.Locator("#inputText", nil)
			require.True(t, tt.query(l))

			p.Evaluate(tb.toGojaValue(tt.eval))
			require.False(t, tt.query(l))
		})
	}
}

func TestLocatorFill(t *testing.T) {
	t.Parallel()

	const value = "fill me up"

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	p.Locator("#inputText", nil).Fill(value, nil)
	require.Equal(t, value, p.InputValue("#inputText", nil))
}

func TestLocatorFocus(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	focused := func() bool {
		return tb.asGojaBool(p.Evaluate(tb.toGojaValue(
			`() => document.activeElement == document.getElementById('inputText')`,
		)))
	}
	l := p.Locator("#inputText", nil)
	require.False(t, focused(), "should not be focused first")

	l.Focus(nil)
	require.True(t, focused(), "should be focused")
}

func TestLocatorGetAttribute(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	l := p.Locator("#inputText", nil)
	v := l.GetAttribute("value", nil)
	require.NotNil(t, v)
	require.Equal(t, "something", v.ToString().String())
}

func TestLocatorInnerHTML(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	require.Equal(t, `<span>hello</span>`, p.Locator("#divHello", nil).InnerHTML(nil))
}

func TestLocatorInnerText(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	require.Equal(t, `hello`, p.Locator("#divHello > span", nil).InnerText(nil))
}

func TestLocatorTextContent(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	require.Equal(t, `hello`, p.Locator("#divHello", nil).TextContent(nil))
}

//nolint:tparallel
func TestLocatorInputValue(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	t.Run("input", func(t *testing.T) {
		require.Equal(t, "something", p.Locator("#inputText", nil).InputValue(nil))
	})
	t.Run("textarea", func(t *testing.T) {
		require.Equal(t, "text area", p.Locator("textarea", nil).InputValue(nil))
	})
	t.Run("select", func(t *testing.T) {
		require.Equal(t, "option text", p.Locator("#selectElement", nil).InputValue(nil))
	})
}

func TestLocatorSelectOption(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	rv := p.Locator("#selectElement", nil).SelectOption(tb.toGojaValue(`option text 2`), nil)
	require.Len(t, rv, 1)
	require.Equal(t, "option text 2", rv[0])
}

func TestLocatorPress(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	p.Locator("#inputText", nil).Press("x", nil)
	require.Equal(t, "xsomething", p.InputValue("#inputText", nil))
}

func TestLocatorType(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	p.Locator("#inputText", nil).Type("real ", nil)
	require.Equal(t, "real something", p.InputValue("#inputText", nil))
}

func TestLocatorHover(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	result := func() bool {
		return tb.asGojaBool(p.Evaluate(tb.toGojaValue(`() => window.result`)))
	}
	require.False(t, result(), "should not be hovered first")
	p.Locator("#inputText", nil).Hover(nil)
	require.True(t, result(), "should be hovered")
}

func TestLocatorTap(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	result := func() bool {
		return tb.asGojaBool(p.Evaluate(tb.toGojaValue(`() => window.result`)))
	}
	require.False(t, result(), "should not be tapped first")
	p.Locator("#inputText", nil).Tap(nil)
	require.True(t, result(), "should be tapped")
}

func TestLocatorDispatchEvent(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	result := func() bool {
		return tb.asGojaBool(p.Evaluate(tb.toGojaValue(`() => window.result`)))
	}
	require.False(t, result(), "should not be clicked first")
	p.Locator("#link", nil).DispatchEvent("click", tb.toGojaValue("mouseevent"), nil)
	require.True(t, result(), "could not dispatch event")
}

//nolint:tparallel
func TestLocatorWaitFor(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	timeout := tb.toGojaValue(jsFrameBaseOpts{Timeout: "100"})

	t.Run("exists", func(t *testing.T) {
		require.NotPanics(t, func() { p.Locator("#link", nil).WaitFor(timeout) })
	})
	t.Run("not_exists", func(t *testing.T) {
		require.Panics(t, func() { p.Locator("#notexists", nil).WaitFor(timeout) })
	})
}

func TestLocatorSanity(t *testing.T) {
	t.Parallel()

	timeout := func(tb *testBrowser) goja.Value {
		return tb.toGojaValue(jsFrameBaseOpts{Timeout: "100"})
	}

	tests := []struct {
		name string
		do   func(api.Locator, *testBrowser)
	}{
		{
			"WaitFor", func(l api.Locator, tb *testBrowser) { l.WaitFor(timeout(tb)) },
		},
		{
			"DispatchEvent", func(l api.Locator, tb *testBrowser) {
				l.DispatchEvent("click", tb.toGojaValue("mouseevent"), timeout(tb))
			},
		},
		{
			"Tap", func(l api.Locator, tb *testBrowser) { l.Tap(timeout(tb)) },
		},
		{
			"Hover", func(l api.Locator, tb *testBrowser) { l.Hover(timeout(tb)) },
		},
		{
			"Type", func(l api.Locator, tb *testBrowser) { l.Type("a", timeout(tb)) },
		},
		{
			"Press", func(l api.Locator, tb *testBrowser) { l.Press("a", timeout(tb)) },
		},
		{
			"SelectOption", func(l api.Locator, tb *testBrowser) { l.SelectOption(tb.toGojaValue(""), timeout(tb)) },
		},
		{
			"GetAttribute", func(l api.Locator, tb *testBrowser) { l.GetAttribute("value", timeout(tb)) },
		},
		{
			"InnerHTML", func(l api.Locator, tb *testBrowser) { l.InnerHTML(timeout(tb)) },
		},
		{
			"InnerText", func(l api.Locator, tb *testBrowser) { l.InnerText(timeout(tb)) },
		},
		{
			"TextContent", func(l api.Locator, tb *testBrowser) { l.TextContent(timeout(tb)) },
		},
		{
			"InputValue", func(l api.Locator, tb *testBrowser) { l.InputValue(timeout(tb)) },
		},
		{
			"Focus", func(l api.Locator, tb *testBrowser) { l.Focus(timeout(tb)) },
		},
		{
			"Fill", func(l api.Locator, tb *testBrowser) { l.Fill("fill me up", timeout(tb)) },
		},
		{
			"Check", func(l api.Locator, tb *testBrowser) { l.Check(timeout(tb)) },
		},
		{
			"Uncheck", func(l api.Locator, tb *testBrowser) { l.Uncheck(timeout(tb)) },
		},
		{
			"IsChecked", func(l api.Locator, tb *testBrowser) { l.IsChecked(timeout(tb)) },
		},
		{
			"Dblclick", func(l api.Locator, tb *testBrowser) { l.Dblclick(timeout(tb)) },
		},
		{
			"Click", func(l api.Locator, tb *testBrowser) { l.Click(timeout(tb)) },
		},
		{
			"IsEditable", func(l api.Locator, tb *testBrowser) { l.IsEditable(timeout(tb)) },
		},
		{
			"IsEnabled", func(l api.Locator, tb *testBrowser) { l.IsEnabled(timeout(tb)) },
		},
		{
			"IsDisabled", func(l api.Locator, tb *testBrowser) { l.IsDisabled(timeout(tb)) },
		},
		{
			"IsVisible", func(l api.Locator, tb *testBrowser) { l.IsVisible(timeout(tb)) },
		},
		{
			"IsHidden", func(l api.Locator, tb *testBrowser) { l.IsHidden(timeout(tb)) },
		},
	}
	for _, tt := range tests {
		t.Run("timeout/"+tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t)
			p := tb.NewPage(nil)
			p.SetContent("<html></html>", nil)
			assert.Panics(t, func() { tt.do(p.Locator("NOTEXIST", nil), tb) })
		})
	}

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))
	for _, tt := range tests {
		t.Run("strict/"+tt.name, func(t *testing.T) {
			assert.Panics(t, func() { tt.do(p.Locator("a", nil), tb) })
		})
	}
}
