// practically none of this work on windows
//go:build !windows

package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/browser"
	"go.k6.io/k6/internal/js/modules/k6/browser/common"
)

// Strict mode:
// All operations on locators throw an exception if more
// than one element matches the locator's selector.

// Note:
// We skip adding t.Parallel to subtests because sobek or our code might race.

func TestLocator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		do   func(*testBrowser, *common.Page)
	}{
		{
			"All", func(_ *testBrowser, p *common.Page) {
				locators, err := p.Locator("a", nil).All()
				require.NoError(t, err)
				require.Len(t, locators, 3)

				firstText, err := locators[0].InnerText(common.NewFrameInnerTextOptions(locators[0].Timeout()))
				assert.NoError(t, err)
				assert.Equal(t, `Click`, firstText)

				secondText, err := locators[1].InnerText(common.NewFrameInnerTextOptions(locators[1].Timeout()))
				assert.NoError(t, err)
				assert.Equal(t, `Dblclick`, secondText)

				thirdText, err := locators[2].InnerText(common.NewFrameInnerTextOptions(locators[2].Timeout()))
				assert.NoError(t, err)
				assert.Equal(t, `Click`, thirdText)
			},
		},
		{
			"BoundingBox", func(_ *testBrowser, p *common.Page) {
				rect, err := p.Locator("#divHello", nil).BoundingBox(&common.FrameBaseOptions{
					Timeout: p.Timeout(),
				})
				require.NoError(t, err)
				assert.GreaterOrEqual(t, rect.X, 0.0)
				assert.GreaterOrEqual(t, rect.Y, 0.0)
				assert.GreaterOrEqual(t, rect.Width, 0.0)
				assert.GreaterOrEqual(t, rect.Height, 0.0)
			},
		},
		{
			"Check", func(_ *testBrowser, p *common.Page) {
				check := func() bool {
					v, err := p.Evaluate(`() => window.check`)
					require.NoError(t, err)
					return asBool(t, v)
				}
				t.Run("check", func(t *testing.T) {
					l := p.Locator("#inputCheckbox", nil)
					require.False(t, check(), "should be unchecked first")
					require.NoError(t, l.Check(common.NewFrameCheckOptions(l.Timeout())))
					require.True(t, check(), "cannot not check the input box")
					require.NoError(t, l.Uncheck(common.NewFrameUncheckOptions(l.Timeout())))
					require.False(t, check(), "cannot not uncheck the input box")
				})
				t.Run("setChecked", func(t *testing.T) {
					l := p.Locator("#inputCheckbox", nil)
					require.False(t, check(), "should be unchecked first")
					require.NoError(t, l.SetChecked(true, common.NewFrameCheckOptions(l.Timeout())))
					require.True(t, check(), "cannot not check the input box")
					require.NoError(t, l.SetChecked(false, common.NewFrameCheckOptions(l.Timeout())))
					require.False(t, check(), "cannot not uncheck the input box")
				})
				t.Run("is_checked", func(t *testing.T) {
					l := p.Locator("#inputCheckbox", nil)
					require.NoError(t, l.Check(common.NewFrameCheckOptions(l.Timeout())))
					checked, err := l.IsChecked(common.NewFrameIsCheckedOptions(l.Timeout()))
					require.NoError(t, err)
					require.True(t, checked)

					require.NoError(t, l.Uncheck(common.NewFrameUncheckOptions(l.Timeout())))
					checked, err = l.IsChecked(common.NewFrameIsCheckedOptions(l.Timeout()))
					require.NoError(t, err)
					require.False(t, checked)
				})
			},
		},
		{
			"Clear", func(_ *testBrowser, p *common.Page) {
				const value = "fill me up"
				l := p.Locator("#inputText", nil)

				require.NoError(t, l.Fill(value, common.NewFrameFillOptions(l.Timeout())))
				inputValue, err := p.InputValue("#inputText", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				require.Equal(t, value, inputValue)

				err = l.Clear(common.NewFrameFillOptions(l.Timeout()))
				assert.NoError(t, err)
				inputValue, err = p.InputValue("#inputText", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				assert.Equal(t, "", inputValue)
			},
		},
		{
			"Click", func(_ *testBrowser, p *common.Page) {
				l := p.Locator("#link", nil)
				err := l.Click(common.NewFrameClickOptions(l.Timeout()))
				require.NoError(t, err)
				v, err := p.Evaluate(`() => window.result`)
				require.NoError(t, err)
				require.True(t, asBool(t, v), "cannot not click the link")
			},
		},
		{
			"Dblclick", func(_ *testBrowser, p *common.Page) {
				lo := p.Locator("#linkdbl", nil)
				require.NoError(t, lo.Dblclick(common.NewFrameDblClickOptions(lo.Timeout())))

				v, err := p.Evaluate(`() => window.dblclick`)
				require.NoError(t, err)
				require.True(t, asBool(t, v), "cannot double click the link")
			},
		},
		{
			"DispatchEvent", func(_ *testBrowser, p *common.Page) {
				result := func() bool {
					v, err := p.Evaluate(`() => window.result`)
					require.NoError(t, err)
					return asBool(t, v)
				}
				require.False(t, result(), "should not be clicked first")

				lo := p.Locator("#link", nil)
				opts := common.NewFrameDispatchEventOptions(0) // no timeout
				require.NoError(t, lo.DispatchEvent("click", "mouseevent", opts))
				require.True(t, result(), "cannot not dispatch event")
			},
		},
		{
			"Fill", func(_ *testBrowser, p *common.Page) {
				const value = "fill me up"
				lo := p.Locator("#inputText", nil)
				require.NoError(t, lo.Fill(value, common.NewFrameFillOptions(lo.Timeout())))
				inputValue, err := p.InputValue("#inputText", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				require.Equal(t, value, inputValue)
			},
		},
		{
			"FillTextarea", func(_ *testBrowser, p *common.Page) {
				const value = "fill me up"
				lo := p.Locator("textarea", nil)
				require.NoError(t, lo.Fill(value, common.NewFrameFillOptions(lo.Timeout())))
				inputValue, err := p.InputValue("textarea", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				require.Equal(t, value, inputValue)
			},
		},
		{
			"FillParagraph", func(_ *testBrowser, p *common.Page) {
				const value = "fill me up"
				lo := p.Locator("#firstParagraph", nil)
				require.NoError(t, lo.Fill(value, common.NewFrameFillOptions(lo.Timeout())))
				textContent, ok, err := p.TextContent("#firstParagraph", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, value, textContent)
				lo = p.Locator("#secondParagraph", nil)
				require.Error(t, lo.Fill(value, common.NewFrameFillOptions(lo.Timeout())))
			},
		},
		{
			"First", func(_ *testBrowser, p *common.Page) {
				first := p.Locator("a", nil).First()
				text, err := first.InnerText(common.NewFrameInnerTextOptions(first.Timeout()))
				require.NoError(t, err)
				require.Equal(t, `Click`, text)
			},
		},
		{
			"Focus", func(_ *testBrowser, p *common.Page) {
				focused := func() bool {
					v, err := p.Evaluate(
						`() => document.activeElement == document.getElementById('inputText')`,
					)
					require.NoError(t, err)
					return asBool(t, v)
				}
				lo := p.Locator("#inputText", nil)
				require.False(t, focused(), "should not be focused first")
				require.NoError(t, lo.Focus(common.NewFrameBaseOptions(lo.Timeout())))
				require.True(t, focused(), "should be focused")
			},
		},
		{
			"GetAttribute", func(_ *testBrowser, p *common.Page) {
				l := p.Locator("#inputText", nil)
				v, ok, err := l.GetAttribute("value", common.NewFrameBaseOptions(l.Timeout()))
				require.NoError(t, err)
				require.NotNil(t, v)
				require.True(t, ok)
				require.Equal(t, "something", v)
			},
		},
		{
			"Hover", func(_ *testBrowser, p *common.Page) {
				result := func() bool {
					v, err := p.Evaluate(`() => window.result`)
					require.NoError(t, err)
					return asBool(t, v)
				}
				require.False(t, result(), "should not be hovered first")
				lo := p.Locator("#inputText", nil)
				require.NoError(t, lo.Hover(common.NewFrameHoverOptions(lo.Timeout())))
				require.True(t, result(), "should be hovered")
			},
		},
		{
			"InnerHTML", func(_ *testBrowser, p *common.Page) {
				divHello := p.Locator("#divHello", nil)
				html, err := divHello.InnerHTML(common.NewFrameInnerHTMLOptions(divHello.Timeout()))
				require.NoError(t, err)
				require.Equal(t, `<span>hello</span>`, html)
			},
		},
		{
			"InnerText", func(_ *testBrowser, p *common.Page) {
				span := p.Locator("#divHello > span", nil)
				text, err := span.InnerText(common.NewFrameInnerTextOptions(span.Timeout()))
				require.NoError(t, err)
				require.Equal(t, `hello`, text)
			},
		},
		{
			"InputValue", func(_ *testBrowser, p *common.Page) {
				t.Run("input", func(t *testing.T) {
					input := p.Locator("#inputText", nil)
					v, err := input.InputValue(common.NewFrameInputValueOptions(input.Timeout()))
					require.NoError(t, err)
					require.Equal(t, "something", v)
				})
				t.Run("textarea", func(t *testing.T) {
					textarea := p.Locator("textarea", nil)
					v, err := textarea.InputValue(common.NewFrameInputValueOptions(textarea.Timeout()))
					require.NoError(t, err)
					require.Equal(t, "text area", v)
				})
				t.Run("select", func(t *testing.T) {
					selectElement := p.Locator("#selectElement", nil)
					v, err := selectElement.InputValue(common.NewFrameInputValueOptions(selectElement.Timeout()))
					require.NoError(t, err)
					require.Equal(t, "option text", v)
				})
			},
		},
		{
			"Last", func(_ *testBrowser, p *common.Page) {
				last := p.Locator("div", nil).Last()
				text, err := last.InnerText(common.NewFrameInnerTextOptions(last.Timeout()))
				require.NoError(t, err)
				require.Equal(t, `bye`, text)
			},
		},
		{
			"Nth", func(_ *testBrowser, p *common.Page) {
				nth := p.Locator("a", nil).Nth(0)
				text, err := nth.InnerText(common.NewFrameInnerTextOptions(nth.Timeout()))
				require.NoError(t, err)
				require.Equal(t, `Click`, text)
			},
		},
		{
			"Press", func(_ *testBrowser, p *common.Page) {
				lo := p.Locator("#inputText", nil)
				require.NoError(t, lo.Press("x", common.NewFramePressOptions(lo.Timeout())))
				inputValue, err := p.InputValue("#inputText", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				require.Equal(t, "xsomething", inputValue)
			},
		},

		{
			"PressSequentially", func(_ *testBrowser, p *common.Page) {
				lo := p.Locator("#inputText", nil)
				require.NoError(t, lo.Clear(common.NewFrameFillOptions(lo.Timeout())))

				require.NoError(t, lo.PressSequentially("hello", common.NewFrameTypeOptions(lo.Timeout())))

				value, err := p.InputValue("#inputText", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				require.Equal(t, "hello", value)
			},
		},
		{
			"PressSequentiallyWithDelayOption", func(_ *testBrowser, p *common.Page) {
				lo := p.Locator("#inputText", nil)
				require.NoError(t, lo.Clear(common.NewFrameFillOptions(lo.Timeout())))

				opts := common.NewFrameTypeOptions(lo.Timeout())
				opts.Delay = 100

				require.NoError(t, lo.PressSequentially("text", opts))

				value, err := p.InputValue("#inputText", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				require.Equal(t, "text", value)
			},
		},
		{
			"PressSequentiallyTextarea", func(_ *testBrowser, p *common.Page) {
				lo := p.Locator("textarea", nil)
				require.NoError(t, lo.Clear(common.NewFrameFillOptions(lo.Timeout())))

				require.NoError(t, lo.PressSequentially("some text", common.NewFrameTypeOptions(lo.Timeout())))

				value, err := lo.InputValue(common.NewFrameInputValueOptions(lo.Timeout()))
				require.NoError(t, err)
				require.Equal(t, "some text", value)
			},
		},
		{
			"SelectOption", func(tb *testBrowser, p *common.Page) {
				l := p.Locator("#selectElement", nil)
				a, err := browser.ConvertSelectOptionValues(tb.vu.Runtime(), tb.toSobekValue(`option text 2`))
				require.NoError(t, err)
				rv, err := l.SelectOption(a, common.NewFrameSelectOptionOptions(l.Timeout()))
				require.NoError(t, err)
				require.Len(t, rv, 1)
				require.Equal(t, "option text 2", rv[0])
			},
		},
		{
			"Tap", func(_ *testBrowser, p *common.Page) {
				result := func() bool {
					v, err := p.Evaluate(`() => window.result`)
					require.NoError(t, err)
					return asBool(t, v)
				}
				require.False(t, result(), "should not be tapped first")
				opts := common.NewFrameTapOptions(common.DefaultTimeout)
				err := p.Locator("#inputText", nil).Tap(opts)
				require.NoError(t, err)
				require.True(t, result(), "should be tapped")
			},
		},
		{
			"TextContent", func(_ *testBrowser, p *common.Page) {
				divHello := p.Locator("#divHello", nil)
				text, ok, err := divHello.TextContent(common.NewFrameTextContentOptions(divHello.Timeout()))
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, `hello`, text)
			},
		},
		{
			"Type", func(_ *testBrowser, p *common.Page) {
				lo := p.Locator("#inputText", nil)
				require.NoError(t, lo.Type("real ", common.NewFrameTypeOptions(lo.Timeout())))
				inputValue, err := p.InputValue("#inputText", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				require.Equal(t, "real something", inputValue)
			},
		},
		{
			"WaitFor state:visible", func(tb *testBrowser, p *common.Page) {
				opts := common.NewFrameWaitForSelectorOptions(100 * time.Millisecond)
				lo := p.Locator("#link", nil)
				require.NoError(t, lo.WaitFor(opts))
			},
		},
		{
			"WaitFor state:attached", func(tb *testBrowser, p *common.Page) {
				opts := common.NewFrameWaitForSelectorOptions(100 * time.Millisecond)
				opts.State = common.DOMElementStateAttached
				lo := p.Locator("#link", nil)
				require.NoError(t, lo.WaitFor(opts))
			},
		},
		{
			"WaitFor state:hidden", func(tb *testBrowser, p *common.Page) {
				opts := common.NewFrameWaitForSelectorOptions(100 * time.Millisecond)
				opts.State = common.DOMElementStateHidden
				lo := p.Locator("#inputHiddenText", nil)
				require.NoError(t, lo.WaitFor(opts))
			},
		},
		{
			"WaitFor state:detached", func(tb *testBrowser, p *common.Page) {
				opts := common.NewFrameWaitForSelectorOptions(100 * time.Millisecond)
				opts.State = common.DOMElementStateDetached
				lo := p.Locator("#nonExistingElement", nil)
				require.NoError(t, lo.WaitFor(opts))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)
			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err := p.Goto(
				tb.staticURL("locators.html"),
				opts,
			)
			tt.do(tb, p)
			require.NoError(t, err)
		})
	}

	sanityTests := []struct {
		name string
		do   func(*common.Locator, *testBrowser) error
	}{
		{
			"BoundingBox", func(l *common.Locator, tb *testBrowser) error {
				_, err := l.BoundingBox(common.NewFrameBaseOptions(100 * time.Millisecond))
				return err
			},
		},
		{
			"Check", func(l *common.Locator, tb *testBrowser) error {
				return l.Check(common.NewFrameCheckOptions(100 * time.Millisecond))
			},
		},
		{
			"Clear", func(l *common.Locator, _ *testBrowser) error {
				opts := common.NewFrameFillOptions(100 * time.Millisecond)
				return l.Clear(opts)
			},
		},
		{
			"Click", func(l *common.Locator, _ *testBrowser) error {
				opts := common.NewFrameClickOptions(100 * time.Millisecond)
				return l.Click(opts)
			},
		},
		{
			"Dblclick", func(l *common.Locator, tb *testBrowser) error {
				return l.Dblclick(common.NewFrameDblClickOptions(100 * time.Millisecond))
			},
		},
		{
			"DispatchEvent", func(l *common.Locator, _ *testBrowser) error {
				opts := common.NewFrameDispatchEventOptions(100 * time.Millisecond)
				return l.DispatchEvent("click", "mouseevent", opts)
			},
		},
		{
			"Focus", func(l *common.Locator, tb *testBrowser) error {
				return l.Focus(common.NewFrameBaseOptions(100 * time.Millisecond))
			},
		},
		{
			"Fill", func(l *common.Locator, tb *testBrowser) error {
				return l.Fill("fill me up", common.NewFrameFillOptions(100*time.Millisecond))
			},
		},
		{
			"GetAttribute", func(l *common.Locator, tb *testBrowser) error {
				_, _, err := l.GetAttribute("value", common.NewFrameBaseOptions(100*time.Millisecond))
				return err
			},
		},
		{
			"Hover", func(l *common.Locator, tb *testBrowser) error {
				return l.Hover(common.NewFrameHoverOptions(100 * time.Millisecond))
			},
		},
		{
			"InnerHTML", func(l *common.Locator, tb *testBrowser) error {
				_, err := l.InnerHTML(common.NewFrameInnerHTMLOptions(100 * time.Millisecond))
				return err
			},
		},
		{
			"InnerText", func(l *common.Locator, tb *testBrowser) error {
				_, err := l.InnerText(common.NewFrameInnerTextOptions(100 * time.Millisecond))
				return err
			},
		},
		{
			"InputValue", func(l *common.Locator, tb *testBrowser) error {
				_, err := l.InputValue(common.NewFrameInputValueOptions(100 * time.Millisecond))
				return err
			},
		},
		{
			"Press", func(l *common.Locator, tb *testBrowser) error {
				return l.Press("a", common.NewFramePressOptions(100*time.Millisecond))
			},
		},
		{
			"PressSequentially", func(l *common.Locator, tb *testBrowser) error {
				return l.PressSequentially("text", common.NewFrameTypeOptions(100*time.Millisecond))
			},
		},
		{
			"SetChecked", func(l *common.Locator, tb *testBrowser) error {
				return l.SetChecked(true, common.NewFrameCheckOptions(100*time.Millisecond))
			},
		},
		{
			"SelectOption", func(l *common.Locator, tb *testBrowser) error {
				_, err := l.SelectOption([]any{""}, common.NewFrameSelectOptionOptions(100*time.Millisecond))
				return err
			},
		},
		{
			"Tap", func(l *common.Locator, _ *testBrowser) error {
				opts := common.NewFrameTapOptions(100 * time.Millisecond)
				return l.Tap(opts)
			},
		},
		{
			"Type", func(l *common.Locator, tb *testBrowser) error {
				return l.Type("a", common.NewFrameTypeOptions(100*time.Millisecond))
			},
		},
		{
			"TextContent", func(l *common.Locator, tb *testBrowser) error {
				_, _, err := l.TextContent(common.NewFrameTextContentOptions(100 * time.Millisecond))
				return err
			},
		},
		{
			"Uncheck", func(l *common.Locator, tb *testBrowser) error {
				return l.Uncheck(common.NewFrameUncheckOptions(100 * time.Millisecond))
			},
		},
		{
			"WaitFor", func(l *common.Locator, tb *testBrowser) error {
				return l.WaitFor(common.NewFrameWaitForSelectorOptions(100 * time.Millisecond))
			},
		},
	}
	for _, tt := range sanityTests {
		t.Run("timeout/"+tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t)
			p := tb.NewPage(nil)
			err := p.SetContent("<html></html>", nil)
			require.NoError(t, err)
			require.Error(t, tt.do(p.Locator("NOTEXIST", nil), tb))
		})
	}

	for _, tt := range sanityTests {
		t.Run("strict/"+tt.name, func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)

			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err := p.Goto(
				tb.staticURL("locators.html"),
				opts,
			)
			require.NoError(t, err)

			require.Error(t, tt.do(p.Locator("a", nil), tb))
		})
	}
}

func TestLocatorElementState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state, eval string
		query       func(*common.Locator) (bool, error)
	}{
		{
			"disabled",
			`() => document.getElementById('inputText').disabled = true`,
			func(l *common.Locator) (bool, error) {
				resp, err := l.IsDisabled(common.NewFrameIsDisabledOptions(l.Timeout()))
				return !resp, err
			},
		},
		{
			"enabled",
			`() => document.getElementById('inputText').disabled = true`,
			func(l *common.Locator) (bool, error) {
				resp, err := l.IsEnabled(common.NewFrameIsEnabledOptions(l.Timeout()))
				return resp, err
			},
		},
		{
			"hidden",
			`() => document.getElementById('inputText').style.visibility = 'hidden'`,
			func(l *common.Locator) (bool, error) {
				resp, err := l.IsHidden()
				return !resp, err
			},
		},
		{
			"readOnly",
			`() => document.getElementById('inputText').readOnly = true`,
			func(l *common.Locator) (bool, error) {
				resp, err := l.IsEditable(common.NewFrameIsEditableOptions(l.Timeout()))
				return resp, err
			},
		},
		{
			"visible",
			`() => document.getElementById('inputText').style.visibility = 'hidden'`,
			func(l *common.Locator) (bool, error) {
				resp, err := l.IsVisible()
				return resp, err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)

			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err := p.Goto(
				tb.staticURL("locators.html"),
				opts,
			)
			require.NoError(t, err)

			l := p.Locator("#inputText", nil)
			result, err := tt.query(l)
			require.NoError(t, err)
			require.True(t, result)

			_, err = p.Evaluate(tt.eval)
			require.NoError(t, err)
			result, err = tt.query(l)
			require.NoError(t, err)
			require.False(t, result)
		})
	}

	sanityTests := []struct {
		name string
		do   func(*common.Locator, *testBrowser) error
	}{
		{
			"IsChecked", func(l *common.Locator, tb *testBrowser) error {
				_, err := l.IsChecked(common.NewFrameIsCheckedOptions(100 * time.Millisecond))
				return err
			},
		},
		{
			"IsEditable", func(l *common.Locator, tb *testBrowser) error {
				_, err := l.IsEditable(common.NewFrameIsEditableOptions(100 * time.Millisecond))
				return err
			},
		},
		{
			"IsEnabled", func(l *common.Locator, tb *testBrowser) error {
				_, err := l.IsEnabled(common.NewFrameIsEnabledOptions(100 * time.Millisecond))
				return err
			},
		},
		{
			"IsDisabled", func(l *common.Locator, tb *testBrowser) error {
				_, err := l.IsDisabled(common.NewFrameIsDisabledOptions(100 * time.Millisecond))
				return err
			},
		},
	}
	for _, tt := range sanityTests {
		t.Run("timeout/"+tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t)
			p := tb.NewPage(nil)
			err := p.SetContent("<html></html>", nil)
			require.NoError(t, err)
			require.Error(t, tt.do(p.Locator("NOTEXIST", nil), tb))
		})
	}

	for _, tt := range sanityTests {
		t.Run("strict/"+tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)

			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err := p.Goto(
				tb.staticURL("locators.html"),
				opts,
			)
			require.NoError(t, err)
			require.Error(t, tt.do(p.Locator("a", nil), tb))
		})
	}
}

func TestLocatorPress(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)

	p := tb.NewPage(nil)

	err := p.SetContent(`<input id="text1">`, nil)
	require.NoError(t, err)

	l := p.Locator("#text1", nil)

	require.NoError(t, l.Press("Shift+KeyA", common.NewFramePressOptions(l.Timeout())))
	require.NoError(t, l.Press("KeyB", common.NewFramePressOptions(l.Timeout())))
	require.NoError(t, l.Press("Shift+KeyC", common.NewFramePressOptions(l.Timeout())))

	v, err := l.InputValue(common.NewFrameInputValueOptions(l.Timeout()))
	require.NoError(t, err)
	require.Equal(t, "AbC", v)
}

func TestLocatorShadowDOM(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)

	popts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	_, err := p.Goto(
		tb.staticURL("shadow_dom_link.html"),
		popts,
	)
	require.NoError(t, err)
	err = p.Click("#inner-link", common.NewFrameClickOptions(time.Second))
	require.NoError(t, err)
}

func TestSelectOption(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	tb.vu.ActivateVU()
	tb.vu.StartIteration(t)

	got := tb.vu.RunPromise(t, `
		const page = await browser.newPage();

		await page.goto('%s');

		const options = page.locator('#numbers-options');

		await options.selectOption({label:'Five'});
		let selectedValue = await options.inputValue();
		if (selectedValue !== 'five') {
			throw new Error('Expected "five" but got ' + selectedValue);
		}

		await options.selectOption({index:5});
		selectedValue = await options.inputValue();
		if (selectedValue !== 'five') {
			throw new Error('Expected "five" but got ' + selectedValue);
		}

		await options.selectOption({value:'four'});
		selectedValue = await options.inputValue();
		if (selectedValue !== 'four') {
			throw new Error('Expected "four" but got ' + selectedValue);
		}

		await options.selectOption([{label:'One'}]);
		selectedValue = await options.inputValue();
		if (selectedValue !== 'one') {
			throw new Error('Expected "one" but got ' + selectedValue);
		}

		await options.selectOption(['two']); // Value
		selectedValue = await options.inputValue();
		if (selectedValue !== 'two') {
			throw new Error('Expected "two" but got ' + selectedValue);
		}

		await options.selectOption('five'); // Value
		selectedValue = await options.inputValue();
		if (selectedValue !== 'five') {
			throw new Error('Expected "five" but got ' + selectedValue);
		}

		await options.selectOption(['Three']); // Label
		selectedValue = await options.inputValue();
		if (selectedValue !== 'three') {
			throw new Error('Expected "three" but got ' + selectedValue);
		}

		await options.selectOption('Five'); // Label
		selectedValue = await options.inputValue();
		if (selectedValue !== 'five') {
			throw new Error('Expected "five" but got ' + selectedValue);
		}

		const results = await options.selectOption(['One', 'two']); // Both label and value
		if (results.length !== 2 || !results.includes('one') || !results.includes('two')) {
			throw new Error('Expected "one,two" but got ' + results);
		}
	`, tb.staticURL("select_options.html"))
	assert.Equal(t, sobek.Undefined(), got.Result())
}

func TestCount(t *testing.T) {
	t.Parallel()

	iframeID := "frameB"

	setupNonCORS := func(t *testing.T) (*testBrowser, *common.Page) {
		t.Helper()

		tb := newTestBrowser(t, withFileServer())

		p := tb.NewPage(nil)
		opts := &common.FrameGotoOptions{
			Timeout: common.DefaultTimeout,
		}
		_, err := p.Goto(
			tb.staticURL("locators.html"),
			opts,
		)
		require.NoError(t, err)

		return tb, p
	}

	setupCORS := func(t *testing.T) (*testBrowser, *common.Page) {
		t.Helper()

		iframeHTML := `<!DOCTYPE html>
		<html>
		<head></head>
		<body>
			<button id="incrementB">Increment Counter B</button>
		</body>
		</html>`

		tb := newTestBrowser(t, withIFrameContent(iframeHTML, iframeID))

		p := tb.GotoNewPage(tb.url("/iframe"))

		return tb, p
	}

	tests := []struct {
		name          string
		setup         func(*testing.T) (*testBrowser, *common.Page)
		do            func(*testBrowser, *common.Page) (int, error)
		expectedCount int
	}{
		{
			name:  "0",
			setup: setupNonCORS,
			do: func(_ *testBrowser, p *common.Page) (int, error) {
				l := p.Locator("#NOTEXIST", nil)
				return l.Count()
			},
			expectedCount: 0,
		},
		{
			name:  "1",
			setup: setupNonCORS,
			do: func(_ *testBrowser, p *common.Page) (int, error) {
				l := p.Locator("#link", nil)
				return l.Count()
			},
			expectedCount: 1,
		},
		{
			name:  "3",
			setup: setupNonCORS,
			do: func(_ *testBrowser, p *common.Page) (int, error) {
				l := p.Locator("a", nil)
				return l.Count()
			},
			expectedCount: 3,
		},
		{
			name:  "CORS",
			setup: setupCORS,
			do: func(_ *testBrowser, p *common.Page) (int, error) {
				frameBContent := p.Locator("#"+iframeID, nil).ContentFrame()
				return frameBContent.Locator("#incrementB", nil).Count()
			},
			expectedCount: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb, p := tt.setup(t)

			c, err := tt.do(tb, p)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, c)
		})
	}
}

func TestReactInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		do   func(*testBrowser, *common.Page)
	}{
		{
			"Fill", func(_ *testBrowser, p *common.Page) {
				const value = "test@example.com"
				lo := p.Locator("input[placeholder='Username or email']", nil)
				require.NoError(t, lo.Fill(value, common.NewFrameFillOptions(lo.Timeout())))
				inputValue, err := p.InnerText("p[id='react-state']", common.NewFrameInnerTextOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				require.Equal(t, fmt.Sprintf("React state: %q", value), inputValue)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)
			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err := p.Goto(
				tb.staticURL("react_input.html"),
				opts,
			)
			tt.do(tb, p)
			require.NoError(t, err)
		})
	}
}

func TestLocatorNesting(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())

	p := tb.NewPage(nil)

	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	_, err := p.Goto(
		tb.staticURL("locator_nesting.html"),
		opts,
	)
	require.NoError(t, err)

	qtyLocator := p.Locator(`[data-testid="inventory"]`, nil).
		Locator(`[data-item="apples"]`, nil).
		Locator(`.qty`, nil)
	q, err := qtyLocator.InnerText(common.NewFrameInnerTextOptions(qtyLocator.Timeout()))
	require.NoError(t, err)
	assert.Equal(t, "0", q)

	err = p.Locator(`[data-testid="inventory"]`, nil).
		Locator(`[data-item="apples"]`, nil).
		Locator(`button.add`, nil).
		Click(common.NewFrameClickOptions(common.DefaultTimeout))
	require.NoError(t, err)

	qtyLocator2 := p.Locator(`[data-testid="inventory"]`, nil).
		Locator(`[data-item="apples"]`, nil).
		Locator(`.qty`, nil)
	q, err = qtyLocator2.InnerText(common.NewFrameInnerTextOptions(qtyLocator2.Timeout()))
	require.NoError(t, err)
	assert.Equal(t, "1", q)
}

// This test ensures that the actionability checks are retried if we receive
// any visible based errors from chrome. This is done by navigating to a page
// where a button is hidden and unhidden every animation frame (~60 times per
// second). We should not be able to click on the button, and if chrome returns
// an error saying the element is not visible, we should retry. The only error
// we expect is the timeout error.
func TestActionabilityRetry(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())

	p := tb.NewPage(nil)

	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	_, err := p.Goto(
		tb.staticURL("hide_unhide.html"),
		opts,
	)
	require.NoError(t, err)

	lo := p.Locator("#incBtn", nil)
	err = lo.Click(common.NewFrameClickOptions(1 * time.Second))
	require.ErrorContains(t, err, "timed out after")

	value := p.Locator("#value", nil)
	text, err := value.InnerText(common.NewFrameInnerTextOptions(value.Timeout()))

	require.NoError(t, err)
	require.Equal(t, "0", text)
}

func TestLocatorFilter(t *testing.T) {
	t.Parallel()

	setupPage := func(t *testing.T) *common.Page {
		t.Helper()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		err := p.SetContent(`
			<section>
				<div>
					<span>hello</span>
				</div>
				<div>
					<span>world</span>
				</div>
			</section>`,
			nil,
		)
		require.NoError(t, err)
		return p
	}

	t.Run("filter_hasText", func(t *testing.T) {
		t.Parallel()

		count, err := setupPage(t).
			Locator("div", nil).
			Filter(&common.LocatorFilterOptions{
				LocatorOptions: &common.LocatorOptions{
					HasText: "hello",
				},
			}).
			Count()
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	t.Run("filter_hasText_on_locator_with_hasText", func(t *testing.T) {
		t.Parallel()

		count, err := setupPage(t).
			Locator("div", &common.LocatorOptions{
				HasText: "hello",
			}).
			Filter(&common.LocatorFilterOptions{
				LocatorOptions: &common.LocatorOptions{
					HasText: "hello",
				},
			}).
			Count()
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	t.Run("filter_hasText_different_on_locator_with_hasText", func(t *testing.T) {
		t.Parallel()

		count, err := setupPage(t).
			Locator("div", &common.LocatorOptions{
				HasText: "hello",
			}).
			Filter(&common.LocatorFilterOptions{
				LocatorOptions: &common.LocatorOptions{
					HasText: "world",
				},
			}).
			Count()
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("filter_hasText_section_with_world", func(t *testing.T) {
		t.Parallel()

		count, err := setupPage(t).
			Locator("section", &common.LocatorOptions{
				HasText: "hello",
			}).
			Filter(&common.LocatorFilterOptions{
				LocatorOptions: &common.LocatorOptions{
					HasText: "world",
				},
			}).
			Count()
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	t.Run("filter_hasText_nested_locator", func(t *testing.T) {
		t.Parallel()

		count, err := setupPage(t).
			Locator("div", nil).
			Filter(&common.LocatorFilterOptions{
				LocatorOptions: &common.LocatorOptions{
					HasText: "hello",
				},
			}).
			Locator("span", nil).
			Count()
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	t.Run("filter_hasNotText_hello", func(t *testing.T) {
		t.Parallel()

		count, err := setupPage(t).
			Locator("div", nil).
			Filter(&common.LocatorFilterOptions{
				LocatorOptions: &common.LocatorOptions{
					HasNotText: "hello",
				},
			}).
			Count()
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	t.Run("filter_hasNotText_foo", func(t *testing.T) {
		t.Parallel()

		count, err := setupPage(t).
			Locator("div", nil).
			Filter(&common.LocatorFilterOptions{
				LocatorOptions: &common.LocatorOptions{
					HasNotText: "foo",
				},
			}).
			Count()
		require.NoError(t, err)
		require.Equal(t, 2, count)
	})
}

func TestFrameLocatorLocatorOptions(t *testing.T) {
	t.Parallel()

	// We'll only test nil and non-nil LocatorOptions here, as the actual
	// filtering logic is tested in TestLocatorLocatorOptions. This test
	// just ensures that FrameLocator.Locator passes the options down correctly.

	setup := func(t *testing.T) *common.FrameLocator {
		t.Helper()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		err := p.SetContent(`
			<iframe srcdoc='
				<section>
					<div>
						<span>hello</span>
					</div>
					<div>
						<span>world</span>
					</div>
				</section>
			'></iframe>`,
			nil,
		)
		require.NoError(t, err)
		return p.Locator("iframe", nil).ContentFrame()
	}
	t.Run("nil_options", func(t *testing.T) {
		t.Parallel()

		n, err := setup(t).
			Locator("div", nil).
			Count()
		require.NoError(t, err)
		require.Equal(t, 2, n)
	})
	t.Run("options", func(t *testing.T) {
		t.Parallel()

		n, err := setup(t).
			Locator("div", &common.LocatorOptions{
				HasText: "hello",
			}).
			Count()
		require.NoError(t, err)
		require.Equal(t, 1, n)
	})
}

func TestLocatorLocatorOptions(t *testing.T) {
	t.Parallel()

	setupPage := func(t *testing.T) *common.Page {
		t.Helper()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		err := p.SetContent(`
			<section>
				<div>
					<span>hello</span>
				</div>
				<div>
					<span>world</span>
				</div>
				<div>
					<span>good bye</span>
					<div>
						<span>moon</span>
						<span>land</span>
					</div>
				</div>
			</section>`,
			nil,
		)
		require.NoError(t, err)
		return p
	}

	t.Run("nil_options", func(t *testing.T) {
		t.Parallel()

		loc := setupPage(t).
			Locator("div", nil).
			Locator("span", nil)
		n, err := loc.Count()
		require.NoError(t, err)
		require.Equal(t, 5, n)
	})

	t.Run("options", func(t *testing.T) {
		t.Parallel()

		// Selects the "moon" and "land" spans.
		loc := setupPage(t).
			Locator("div", &common.LocatorOptions{
				HasText: "good bye",
			}).
			Locator("span", &common.LocatorOptions{
				HasNotText: "good bye",
			})
		locs, err := loc.All()
		require.NoError(t, err)
		require.Len(t, locs, 2)

		text, ok, err := locs[0].TextContent(common.NewFrameTextContentOptions(locs[0].Timeout()))
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "moon", text)

		text, ok, err = locs[1].TextContent(common.NewFrameTextContentOptions(locs[1].Timeout()))
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "land", text)
	})

	t.Run("nil_options_with_filter", func(t *testing.T) {
		t.Parallel()

		// Finds the divs, filters to the one with "good bye",
		// then finds its spans and filters to the one with "moon".
		loc := setupPage(t).
			Locator("div", nil).
			Filter(&common.LocatorFilterOptions{
				LocatorOptions: &common.LocatorOptions{
					HasText: "good bye",
				},
			}).
			Locator("span", nil).
			Filter(&common.LocatorFilterOptions{
				LocatorOptions: &common.LocatorOptions{
					HasText: "moon",
				},
			})
		n, err := loc.Count()
		require.NoError(t, err)
		require.Equal(t, 1, n)

		text, ok, err := loc.TextContent(common.NewFrameTextContentOptions(loc.Timeout()))
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "moon", text)
	})

	t.Run("options_with_filter", func(t *testing.T) {
		t.Parallel()

		loc := setupPage(t).
			// Finds the div element with the "good bye" text.
			Locator("div", &common.LocatorOptions{
				HasText: "good bye",
			}).
			// Filters out child spans with the "good bye" text.
			Locator("span", &common.LocatorOptions{
				HasNotText: "good bye",
			}).
			// Filters out childs span with the "moon" text.
			Filter(&common.LocatorFilterOptions{
				LocatorOptions: &common.LocatorOptions{
					HasNotText: "moon",
				},
			})
		n, err := loc.Count()
		require.NoError(t, err)
		require.Equal(t, 1, n)

		text, ok, err := loc.TextContent(common.NewFrameTextContentOptions(loc.Timeout()))
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "land", text)
	})
}

func TestVisibilityWithCORS(t *testing.T) {
	t.Parallel()

	iframeID := "frameB"

	setupCORS := func(t *testing.T) (*testBrowser, *common.Page) {
		t.Helper()

		iframeHTML := `<!DOCTYPE html>
		<html>
		<head></head>
		<body>
			<button id="visibleButton">Hello</button>
			<button id="hiddenButton" hidden>World</button>
		</body>
		</html>`

		tb := newTestBrowser(t, withIFrameContent(iframeHTML, iframeID))

		p := tb.GotoNewPage(tb.url("/iframe"))

		return tb, p
	}

	tests := []struct {
		name string
		do   func(*testBrowser, *common.Page) (bool, error)
		want bool
	}{
		{
			name: "hidden",
			do: func(_ *testBrowser, p *common.Page) (bool, error) {
				frameBContent := p.Locator("#"+iframeID, nil).ContentFrame()
				return frameBContent.Locator("#hiddenButton", nil).IsHidden()
			},
			want: true,
		},
		{
			name: "visible",
			do: func(_ *testBrowser, p *common.Page) (bool, error) {
				frameBContent := p.Locator("#"+iframeID, nil).ContentFrame()
				return frameBContent.Locator("#visibleButton", nil).IsVisible()
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb, p := setupCORS(t)

			got, err := tt.do(tb, p)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLocatorEvaluate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pageFunc string
		args     []any
		expected string
	}{
		{
			name:     "no_args",
			pageFunc: `handle => handle.innerText`,
			args:     nil,
			expected: "Some title",
		},
		{
			name: "with_args",
			pageFunc: `(handle, a, b) => {
				const c = a + b;
				return handle.innerText + " " + c
			}`,
			args:     []any{1, 2},
			expected: "Some title 3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t)
			p := tb.NewPage(nil)

			err := p.SetContent(`<html><head><title data-testid="title">Some title</title></head></html>`, nil)
			require.NoError(t, err)

			result := p.GetByTestID("'title'")
			require.NotNil(t, result)

			got, err := result.Evaluate(tt.pageFunc, tt.args...)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestLocatorEvaluateHandle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pageFunc string
		args     []any
		expected string
	}{
		{
			name: "no_args",
			pageFunc: `handle => {
				return {"innerText": handle.innerText};
			}`,
			args:     nil,
			expected: `{"innerText":"Some title"}`,
		},
		{
			name: "with_args",
			pageFunc: `(handle, a, b) => {
				return {"innerText": handle.innerText, "sum": a + b};
			}`,
			args:     []any{1, 2},
			expected: `{"innerText":"Some title","sum":3}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t)
			p := tb.NewPage(nil)

			err := p.SetContent(`<html><head><title data-testid="title">Some title</title></head></html>`, nil)
			require.NoError(t, err)

			result := p.GetByTestID("'title'")
			require.NotNil(t, result)

			got, err := result.EvaluateHandle(tt.pageFunc, tt.args...)
			require.NoError(t, err)
			assert.NotNil(t, got)

			j, err := got.JSONValue()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, j)
		})
	}
}

func TestFrameLocator(t *testing.T) {
	t.Parallel()

	t.Run("page_frameLocator", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())
		p := tb.NewPage(nil)

		opts := &common.FrameGotoOptions{
			Timeout: common.DefaultTimeout,
		}
		_, err := p.Goto(tb.staticURL("iframe_test_main.html"), opts)
		require.NoError(t, err)

		// Test the new frameLocator() method on Page
		fl := p.FrameLocator("#iframe1")
		require.NotNil(t, fl)

		nestedFL := fl.FrameLocator("#iframe2")
		require.NotNil(t, nestedFL)

		buttonLocator := nestedFL.Locator("#button1", nil)
		require.NotNil(t, buttonLocator)

		err = buttonLocator.Click(common.NewFrameClickOptions(buttonLocator.Timeout()))
		require.NoError(t, err)

		clicked, err := buttonLocator.Evaluate("el => window.buttonClicked")
		require.NoError(t, err)
		assert.True(t, clicked.(bool), "buttonClicked should be true after click")
	})

	t.Run("frame_frameLocator", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())
		p := tb.NewPage(nil)

		opts := &common.FrameGotoOptions{
			Timeout: common.DefaultTimeout,
		}
		_, err := p.Goto(tb.staticURL("iframe_test_main.html"), opts)
		require.NoError(t, err)

		// Test the new frameLocator() method on Frame
		mainFrame := p.MainFrame()
		fl := mainFrame.FrameLocator("#iframe1")
		require.NotNil(t, fl)

		nestedFL := fl.FrameLocator("#iframe2")
		require.NotNil(t, nestedFL)

		buttonLocator := nestedFL.Locator("#button1", nil)
		require.NotNil(t, buttonLocator)

		err = buttonLocator.Click(common.NewFrameClickOptions(buttonLocator.Timeout()))
		require.NoError(t, err)

		clicked, err := buttonLocator.Evaluate("el => window.buttonClicked")
		require.NoError(t, err)
		assert.True(t, clicked.(bool), "buttonClicked should be true after click")
	})

	t.Run("locator_frameLocator", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())
		p := tb.NewPage(nil)

		opts := &common.FrameGotoOptions{
			Timeout: common.DefaultTimeout,
		}
		_, err := p.Goto(tb.staticURL("iframe_test_main.html"), opts)
		require.NoError(t, err)

		bodyLocator := p.Locator("body", nil)
		fl := bodyLocator.FrameLocator("#iframe1")
		require.NotNil(t, fl)

		nestedFL := fl.FrameLocator("#iframe2")
		require.NotNil(t, nestedFL)

		buttonLocator := nestedFL.Locator("#button1", nil)
		require.NotNil(t, buttonLocator)

		err = buttonLocator.Click(common.NewFrameClickOptions(buttonLocator.Timeout()))
		require.NoError(t, err)

		clicked, err := buttonLocator.Evaluate("el => window.buttonClicked")
		require.NoError(t, err)
		assert.True(t, clicked.(bool), "buttonClicked should be true after click")
	})

	t.Run("comparison_with_contentFrame", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())
		p := tb.NewPage(nil)

		opts := &common.FrameGotoOptions{
			Timeout: common.DefaultTimeout,
		}
		_, err := p.Goto(tb.staticURL("iframe_test_main.html"), opts)
		require.NoError(t, err)

		// compare old and new
		oldWay := p.Locator("#iframe1", nil).ContentFrame()
		require.NotNil(t, oldWay)

		newWay := p.FrameLocator("#iframe1")
		require.NotNil(t, newWay)

		oldNested := oldWay.FrameLocator("#iframe2")
		newNested := newWay.FrameLocator("#iframe2")

		oldButton := oldNested.Locator("#button1", nil)
		newButton := newNested.Locator("#button1", nil)

		require.NotNil(t, oldButton)
		require.NotNil(t, newButton)
	})
}
