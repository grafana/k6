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

func TestLocator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		do   func(*testBrowser, api.Page)
	}{
		{
			"Check", func(tb *testBrowser, p api.Page) {
				t.Run("check", func(t *testing.T) {
					check := func() bool {
						v := p.Evaluate(tb.toGojaValue(`() => window.check`))
						return tb.asGojaBool(v)
					}
					l := p.Locator("#inputCheckbox", nil)
					require.False(t, check(), "should be unchecked first")
					l.Check(nil)
					require.True(t, check(), "cannot not check the input box")
					l.Uncheck(nil)
					require.False(t, check(), "cannot not uncheck the input box")
				})
				t.Run("is_checked", func(t *testing.T) {
					l := p.Locator("#inputCheckbox", nil)
					l.Check(nil)
					require.True(t, l.IsChecked(nil))
					l.Uncheck(nil)
					require.False(t, l.IsChecked(nil))
				})
			},
		},
		{
			"Click", func(tb *testBrowser, p api.Page) {
				p.Locator("#link", nil).Click(nil)
				v := p.Evaluate(tb.toGojaValue(`() => window.result`))
				require.True(t, tb.asGojaBool(v), "cannot not click the link")
			},
		},
		{
			"DblClick", func(tb *testBrowser, p api.Page) {
				p.Locator("#link", nil).Dblclick(nil)
				v := p.Evaluate(tb.toGojaValue(`() => window.dblclick`))
				require.True(t, tb.asGojaBool(v), "cannot not double click the link")
			},
		},
		{
			"DispatchEvent", func(tb *testBrowser, p api.Page) {
				result := func() bool {
					v := p.Evaluate(tb.toGojaValue(`() => window.result`))
					return tb.asGojaBool(v)
				}
				require.False(t, result(), "should not be clicked first")
				p.Locator("#link", nil).DispatchEvent("click", tb.toGojaValue("mouseevent"), nil)
				require.True(t, result(), "cannot not dispatch event")
			},
		},
		{
			"Fill", func(tb *testBrowser, p api.Page) {
				const value = "fill me up"
				p.Locator("#inputText", nil).Fill(value, nil)
				require.Equal(t, value, p.InputValue("#inputText", nil))
			},
		},
		{
			"Focus", func(tb *testBrowser, p api.Page) {
				focused := func() bool {
					v := p.Evaluate(tb.toGojaValue(
						`() => document.activeElement == document.getElementById('inputText')`,
					))
					return tb.asGojaBool(v)
				}
				l := p.Locator("#inputText", nil)
				require.False(t, focused(), "should not be focused first")
				l.Focus(nil)
				require.True(t, focused(), "should be focused")
			},
		},
		{
			"GetAttribute", func(tb *testBrowser, p api.Page) {
				l := p.Locator("#inputText", nil)
				v := l.GetAttribute("value", nil)
				require.NotNil(t, v)
				require.Equal(t, "something", v.ToString().String())
			},
		},
		{
			"Hover", func(tb *testBrowser, p api.Page) {
				result := func() bool {
					v := p.Evaluate(tb.toGojaValue(`() => window.result`))
					return tb.asGojaBool(v)
				}
				require.False(t, result(), "should not be hovered first")
				p.Locator("#inputText", nil).Hover(nil)
				require.True(t, result(), "should be hovered")
			},
		},
		{
			"InnerHTML", func(tb *testBrowser, p api.Page) {
				require.Equal(t, `<span>hello</span>`, p.Locator("#divHello", nil).InnerHTML(nil))
			},
		},
		{
			"InnerText", func(tb *testBrowser, p api.Page) {
				require.Equal(t, `hello`, p.Locator("#divHello > span", nil).InnerText(nil))
			},
		},
		{
			"InputValue", func(tb *testBrowser, p api.Page) {
				t.Run("input", func(t *testing.T) {
					require.Equal(t, "something", p.Locator("#inputText", nil).InputValue(nil))
				})
				t.Run("textarea", func(t *testing.T) {
					require.Equal(t, "text area", p.Locator("textarea", nil).InputValue(nil))
				})
				t.Run("select", func(t *testing.T) {
					require.Equal(t, "option text", p.Locator("#selectElement", nil).InputValue(nil))
				})
			},
		},
		{
			"Press", func(tb *testBrowser, p api.Page) {
				p.Locator("#inputText", nil).Press("x", nil)
				require.Equal(t, "xsomething", p.InputValue("#inputText", nil))
			},
		},
		{
			"SelectOption", func(tb *testBrowser, p api.Page) {
				l := p.Locator("#selectElement", nil)
				rv := l.SelectOption(tb.toGojaValue(`option text 2`), nil)
				require.Len(t, rv, 1)
				require.Equal(t, "option text 2", rv[0])
			},
		},
		{
			"Tap", func(tb *testBrowser, p api.Page) {
				result := func() bool {
					v := p.Evaluate(tb.toGojaValue(`() => window.result`))
					return tb.asGojaBool(v)
				}
				require.False(t, result(), "should not be tapped first")
				p.Locator("#inputText", nil).Tap(nil)
				require.True(t, result(), "should be tapped")
			},
		},
		{
			"TextContent", func(tb *testBrowser, p api.Page) {
				require.Equal(t, `hello`, p.Locator("#divHello", nil).TextContent(nil))
			},
		},
		{
			"Type", func(tb *testBrowser, p api.Page) {
				p.Locator("#inputText", nil).Type("real ", nil)
				require.Equal(t, "real something", p.InputValue("#inputText", nil))
			},
		},
		{
			"WaitFor", func(tb *testBrowser, p api.Page) {
				timeout := tb.toGojaValue(jsFrameBaseOpts{Timeout: "100"})
				require.NotPanics(t, func() { p.Locator("#link", nil).WaitFor(timeout) })
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)
			require.NotNil(t, p.Goto(tb.staticURL("locators.html"), nil))
			tt.do(tb, p)
		})
	}

	timeout := func(tb *testBrowser) goja.Value {
		return tb.toGojaValue(jsFrameBaseOpts{Timeout: "100"})
	}
	sanityTests := []struct {
		name string
		do   func(api.Locator, *testBrowser)
	}{
		{
			"Check", func(l api.Locator, tb *testBrowser) { l.Check(timeout(tb)) },
		},
		{
			"Click", func(l api.Locator, tb *testBrowser) { l.Click(timeout(tb)) },
		},
		{
			"Dblclick", func(l api.Locator, tb *testBrowser) { l.Dblclick(timeout(tb)) },
		},
		{
			"DispatchEvent", func(l api.Locator, tb *testBrowser) {
				l.DispatchEvent("click", tb.toGojaValue("mouseevent"), timeout(tb))
			},
		},
		{
			"Focus", func(l api.Locator, tb *testBrowser) { l.Focus(timeout(tb)) },
		},
		{
			"Fill", func(l api.Locator, tb *testBrowser) { l.Fill("fill me up", timeout(tb)) },
		},
		{
			"GetAttribute", func(l api.Locator, tb *testBrowser) { l.GetAttribute("value", timeout(tb)) },
		},
		{
			"Hover", func(l api.Locator, tb *testBrowser) { l.Hover(timeout(tb)) },
		},
		{
			"InnerHTML", func(l api.Locator, tb *testBrowser) { l.InnerHTML(timeout(tb)) },
		},
		{
			"InnerText", func(l api.Locator, tb *testBrowser) { l.InnerText(timeout(tb)) },
		},
		{
			"InputValue", func(l api.Locator, tb *testBrowser) { l.InputValue(timeout(tb)) },
		},
		{
			"Press", func(l api.Locator, tb *testBrowser) { l.Press("a", timeout(tb)) },
		},
		{
			"SelectOption", func(l api.Locator, tb *testBrowser) { l.SelectOption(tb.toGojaValue(""), timeout(tb)) },
		},
		{
			"Tap", func(l api.Locator, tb *testBrowser) { l.Tap(timeout(tb)) },
		},
		{
			"Type", func(l api.Locator, tb *testBrowser) { l.Type("a", timeout(tb)) },
		},
		{
			"TextContent", func(l api.Locator, tb *testBrowser) { l.TextContent(timeout(tb)) },
		},
		{
			"Uncheck", func(l api.Locator, tb *testBrowser) { l.Uncheck(timeout(tb)) },
		},
		{
			"WaitFor", func(l api.Locator, tb *testBrowser) { l.WaitFor(timeout(tb)) },
		},
	}
	for _, tt := range sanityTests {
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
	require.NotNil(t, p.Goto(tb.staticURL("locators.html"), nil))
	for _, tt := range sanityTests {
		t.Run("strict/"+tt.name, func(t *testing.T) {
			assert.Panics(t, func() { tt.do(p.Locator("a", nil), tb) })
		})
	}
}

func TestLocatorElementState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state, eval string
		query       func(api.Locator) bool
	}{
		{
			"disabled",
			`() => document.getElementById('inputText').disabled = true`,
			func(l api.Locator) bool { return !l.IsDisabled(nil) },
		},
		{
			"enabled",
			`() => document.getElementById('inputText').disabled = true`,
			func(l api.Locator) bool { return l.IsEnabled(nil) },
		},
		{
			"hidden",
			`() => document.getElementById('inputText').style.visibility = 'hidden'`,
			func(l api.Locator) bool { return !l.IsHidden(nil) },
		},
		{
			"readOnly",
			`() => document.getElementById('inputText').readOnly = true`,
			func(l api.Locator) bool { return l.IsEditable(nil) },
		},
		{
			"visible",
			`() => document.getElementById('inputText').style.visibility = 'hidden'`,
			func(l api.Locator) bool { return l.IsVisible(nil) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)
			require.NotNil(t, p.Goto(tb.staticURL("locators.html"), nil))

			l := p.Locator("#inputText", nil)
			require.True(t, tt.query(l))

			p.Evaluate(tb.toGojaValue(tt.eval))
			require.False(t, tt.query(l))
		})
	}

	timeout := func(tb *testBrowser) goja.Value {
		return tb.toGojaValue(jsFrameBaseOpts{Timeout: "100"})
	}
	sanityTests := []struct {
		name string
		do   func(api.Locator, *testBrowser)
	}{

		{
			"IsChecked", func(l api.Locator, tb *testBrowser) { l.IsChecked(timeout(tb)) },
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
	for _, tt := range sanityTests {
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
	require.NotNil(t, p.Goto(tb.staticURL("locators.html"), nil))
	for _, tt := range sanityTests {
		t.Run("strict/"+tt.name, func(t *testing.T) {
			assert.Panics(t, func() { tt.do(p.Locator("a", nil), tb) })
		})
	}
}
