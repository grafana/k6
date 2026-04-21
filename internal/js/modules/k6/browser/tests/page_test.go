package tests

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
	k6metrics "go.k6.io/k6/metrics"
)

type jsFrameBaseOpts struct {
	Timeout string
	Strict  bool
}

const sampleHTML = `<div><b>Test</b><ol><li><i>One</i></li></ol></div>`

func TestNestedFrames(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("windows take forever to do this test")
	}

	tb := newTestBrowser(t,
		withFileServer(),
	)

	page := tb.NewPage(nil)
	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	_, err := page.Goto(
		tb.staticURL("iframe_test_main.html"),
		opts,
	)
	require.NoError(t, err)

	frame1Handle, err := page.WaitForSelector("iframe[id='iframe1']", common.NewFrameWaitForSelectorOptions(page.MainFrame().Timeout()))
	assert.Nil(t, err)
	assert.NotNil(t, frame1Handle)

	frame1, err := frame1Handle.ContentFrame()
	assert.Nil(t, err)
	assert.NotNil(t, frame1)

	frame2Handle, err := frame1.WaitForSelector("iframe[id='iframe2']", common.NewFrameWaitForSelectorOptions(frame1.Timeout()))
	assert.Nil(t, err)
	assert.NotNil(t, frame2Handle)

	frame2, err := frame2Handle.ContentFrame()
	assert.Nil(t, err)
	assert.NotNil(t, frame2)

	button1Handle, err := frame2.WaitForSelector("button[id='button1']", common.NewFrameWaitForSelectorOptions(frame2.Timeout()))
	assert.Nil(t, err)
	assert.NotNil(t, button1Handle)

	err = button1Handle.Click(common.NewElementHandleClickOptions(button1Handle.Timeout()))
	assert.Nil(t, err)

	v, err := frame2.Evaluate(`() => window.buttonClicked`)
	require.NoError(t, err)
	bv := asBool(t, v)
	assert.True(t, bv, "button hasn't been clicked")
}

func TestPageEmulateMedia(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	popts := &common.PageEmulateMediaOptions{
		Media:         "print",
		ColorScheme:   "dark",
		ReducedMotion: "reduce",
	}
	err := p.EmulateMedia(popts)
	require.NoError(t, err)

	result, err := p.Evaluate(`() => matchMedia('print').matches`)
	require.NoError(t, err)
	assert.IsTypef(t, true, result, "expected media 'print'")

	result, err = p.Evaluate(`() => matchMedia('(prefers-color-scheme: dark)').matches`)
	require.NoError(t, err)
	assert.IsTypef(t, true, result, "expected color scheme 'dark'")

	result, err = p.Evaluate(`() => matchMedia('(prefers-reduced-motion: reduce)').matches`)
	require.NoError(t, err)
	assert.IsTypef(t, true, result, "expected reduced motion setting to be 'reduce'")
}

func TestPageContent(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	content := `<!DOCTYPE html><html><head></head><body><h1>Hello</h1></body></html>`
	err := p.SetContent(content, nil)
	require.NoError(t, err)

	content, err = p.Content()
	require.NoError(t, err)
	assert.Equal(t, content, content)
}

func TestPageEvaluate(t *testing.T) {
	t.Parallel()

	t.Run("ok/func_arg", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)

		got, err := p.Evaluate(
			`(v) => { window.v = v; return window.v }`,
			"test",
		)
		require.NoError(t, err)
		s := asString(t, got)
		assert.Equal(t, "test", s)
	})

	t.Run("ok/void_func", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		h, err := p.EvaluateHandle(`() => console.log("hello")`)
		assert.Nil(t, h, "expected nil handle")
		assert.Error(t, err)
	})

	t.Run("err", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name, js, errMsg string
		}{
			{
				"promise",
				`async () => { return await new Promise((res, rej) => { rej('rejected'); }); }`,
				"evaluating JS: rejected",
			},
			{
				"syntax", `() => {`,
				"evaluating JS: SyntaxError: Unexpected token ')'",
			},
			{"undef", "undef", "evaluating JS: ReferenceError: undef is not defined"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				tb := newTestBrowser(t)
				p := tb.NewPage(nil)
				_, err := p.Evaluate(tc.js)
				require.ErrorContains(t, err, tc.errMsg)
			})
		}
	})
}

func TestPageEvaluateMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		script string
		want   any
	}{
		{
			name:   "arrow",
			script: "() => 0",
			want:   0,
		},
		{
			name:   "full_func",
			script: "function() {return 1}",
			want:   1,
		},
		{
			name:   "arrow_func_no_return",
			script: "() => {2}",
			want:   sobek.Null(),
		},
		{
			name:   "full_func_no_return",
			script: "function() {3}",
			want:   sobek.Null(),
		},
		{
			name:   "async_func",
			script: "async function() {return 4}",
			want:   4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t)
			tb.vu.ActivateVU()
			tb.vu.StartIteration(t)
			defer tb.vu.EndIteration(t)

			// Test script as non string input
			tb.vu.SetVar(t, "p", &sobek.Object{})
			got := tb.vu.RunPromise(t, `
				p = await browser.newPage()
				return await p.evaluate(%s)
			`, tt.script)
			assert.Equal(t, tb.vu.ToSobekValue(tt.want), got.Result())

			// Test script as string input
			got = tb.vu.RunPromise(t,
				`return await p.evaluate("%s")`,
				tt.script,
			)
			assert.Equal(t, tb.vu.ToSobekValue(tt.want), got.Result())
			// Test script as string input
			_ = tb.vu.RunPromise(t, `await p.close()`)
		})
	}
}

func TestPageEvaluateMappingError(t *testing.T) { //nolint:tparallel
	t.Parallel()

	tb := newTestBrowser(t)
	tests := []struct {
		name    string
		script  string
		wantErr string
	}{
		{
			name:    "invalid",
			script:  "5",
			wantErr: "Given expression does not evaluate to a function",
		},
		{
			name:    "invalid_with_brackets",
			script:  "(6)",
			wantErr: "Given expression does not evaluate to a function",
		},
		{
			name:    "void",
			script:  "",
			wantErr: "evaluate requires a page function",
		},
	}

	for _, tt := range tests { //nolint:paralleltest
		t.Run(tt.name, func(t *testing.T) {
			tb.vu.ActivateVU()
			tb.vu.StartIteration(t)
			defer tb.vu.EndIteration(t)
			// Test script as non string input
			tb.vu.SetVar(t, "p", &sobek.Object{})
			_, err := tb.vu.RunAsync(t, `
				p = await browser.newPage()
				await p.evaluate(%s)
			`, tt.script)
			assert.ErrorContains(t, err, tt.wantErr)

			// Test script as string input
			_, err = tb.vu.RunAsync(t, `
				await p.evaluate("%s")
			`, tt.script)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestPageGoto(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t, withFileServer())
	p := b.NewPage(nil)

	url := b.staticURL("empty.html")
	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	r, err := p.Goto(
		url,
		opts,
	)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, url, r.URL(), `expected URL to be %q, result of navigation was %q`, url, r.URL())
}

func TestPageGotoDataURI(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t)
	p := b.NewPage(nil)

	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	r, err := p.Goto(
		"data:text/html,hello",
		opts,
	)
	require.NoError(t, err)
	assert.Nil(t, r, `expected response to be nil`)
	require.NoError(t, err)
}

func TestPageGotoWaitUntilLoad(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip() // no idea but it doesn't work
	}

	b := newTestBrowser(t, withFileServer())
	p := b.NewPage(nil)

	opts := &common.FrameGotoOptions{
		WaitUntil: common.LifecycleEventLoad,
		Timeout:   common.DefaultTimeout,
	}
	_, err := p.Goto(b.staticURL("wait_until.html"), opts)
	require.NoError(t, err)
	results, err := p.Evaluate(`() => window.results`)
	require.NoError(t, err)
	assert.EqualValues(t,
		[]any{"DOMContentLoaded", "load"}, results,
		`expected "load" event to have fired`,
	)
}

func TestPageGotoWaitUntilDOMContentLoaded(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip() // no idea but it doesn't work
	}

	b := newTestBrowser(t, withFileServer())
	p := b.NewPage(nil)

	opts := &common.FrameGotoOptions{
		WaitUntil: common.LifecycleEventDOMContentLoad,
		Timeout:   common.DefaultTimeout,
	}
	_, err := p.Goto(b.staticURL("wait_until.html"), opts)
	require.NoError(t, err)
	v, err := p.Evaluate(`() => window.results`)
	require.NoError(t, err)
	results, ok := v.([]any)
	if !ok {
		t.Fatalf("expected results to be a slice, got %T", v)
	}
	require.True(t, len(results) >= 1, "expected at least one result")
	assert.EqualValues(t,
		"DOMContentLoaded", results[0],
		`expected "DOMContentLoaded" event to have fired`,
	)
}

func TestPageInnerHTML(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		p := newTestBrowser(t).NewPage(nil)
		err := p.SetContent(sampleHTML, nil)
		require.NoError(t, err)
		innerHTML, err := p.InnerHTML("div", common.NewFrameInnerHTMLOptions(p.MainFrame().Timeout()))
		require.NoError(t, err)
		assert.Equal(t, `<b>Test</b><ol><li><i>One</i></li></ol>`, innerHTML)
	})

	t.Run("err_empty_selector", func(t *testing.T) {
		t.Parallel()

		defer func() {
		}()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		_, err := p.InnerHTML("", common.NewFrameInnerHTMLOptions(p.Context().Timeout()))
		require.ErrorContains(t, err, "provided selector is empty")
	})

	t.Run("err_wrong_selector", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		err := p.SetContent(sampleHTML, nil)
		require.NoError(t, err)
		popts := common.NewFrameInnerHTMLOptions(p.MainFrame().Timeout())
		require.NoError(t, popts.Parse(tb.vu.Context(), tb.toSobekValue(jsFrameBaseOpts{Timeout: "100"})))
		_, err = p.InnerHTML("p", popts)
		require.Error(t, err)
	})
}

func TestPageInnerText(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		p := newTestBrowser(t).NewPage(nil)
		err := p.SetContent(sampleHTML, nil)
		require.NoError(t, err)
		innerText, err := p.InnerText("div", common.NewFrameInnerTextOptions(p.MainFrame().Timeout()))
		require.NoError(t, err)
		assert.Equal(t, "Test\nOne", innerText)
	})

	t.Run("err_empty_selector", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		_, err := p.InnerText("", common.NewFrameInnerTextOptions(p.MainFrame().Timeout()))
		require.ErrorContains(t, err, "provided selector is empty")
	})

	t.Run("err_wrong_selector", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		err := p.SetContent(sampleHTML, nil)
		require.NoError(t, err)

		popts := common.NewFrameInnerTextOptions(p.MainFrame().Timeout())
		require.NoError(t, popts.Parse(tb.vu.Context(), tb.toSobekValue(jsFrameBaseOpts{Timeout: "100"})))
		_, err = p.InnerText("p", popts)
		require.Error(t, err)
	})
}

func TestPageTextContent(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		p := newTestBrowser(t).NewPage(nil)
		err := p.SetContent(sampleHTML, nil)
		require.NoError(t, err)
		textContent, ok, err := p.TextContent("div", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, "TestOne", textContent)
	})

	t.Run("err_empty_selector", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		_, _, err := p.TextContent("", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
		require.ErrorContains(t, err, "provided selector is empty")
	})

	t.Run("err_wrong_selector", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		err := p.SetContent(sampleHTML, nil)
		require.NoError(t, err)
		_, _, err = p.TextContent("p", common.NewFrameTextContentOptions(100))
		require.Error(t, err)
	})

	t.Run("err_wrong_selector", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		err := p.SetContent(sampleHTML, nil)
		require.NoError(t, err)
		_, _, err = p.TextContent("p", common.NewFrameTextContentOptions(100))
		require.Error(t, err)
	})
}

func TestPageInputValue(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)

	err := p.SetContent(`
		<input value="hello1">
		<select><option value="hello2" selected></option></select>
		<textarea>hello3</textarea>
     	`, nil)
	require.NoError(t, err)

	inputValue, err := p.InputValue("input", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
	require.NoError(t, err)
	got, want := inputValue, "hello1"
	assert.Equal(t, want, got)

	inputValue, err = p.InputValue("select", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
	require.NoError(t, err)
	got, want = inputValue, "hello2"
	assert.Equal(t, want, got)

	inputValue, err = p.InputValue("textarea", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
	require.NoError(t, err)
	got, want = inputValue, "hello3"
	assert.Equal(t, want, got)
}

// test for: https://go.k6.io/k6/js/modules/k6/browser/issues/132
func TestPageInputSpecialCharacters(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)

	err := p.SetContent(`<input id="special">`, nil)
	require.NoError(t, err)
	el, err := p.Query("#special")
	require.NoError(t, err)

	wants := []string{
		"test@k6.io",
		"<hello WoRlD \\/>",
		"{(hello world!)}",
		"!#$%^&*()+_|~±",
		`¯\_(ツ)_/¯`,
	}
	for _, want := range wants {
		require.NoError(t, el.Fill("", common.NewElementHandleBaseOptions(common.DefaultTimeout)))
		require.NoError(t, el.Type(want, common.NewElementHandleTypeOptions(common.DefaultTimeout)))

		got, err := el.InputValue(common.NewElementHandleBaseOptions(common.DefaultTimeout))
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
}

//nolint:paralleltest
func TestPageFill(t *testing.T) {
	// these tests are not parallel by intention because
	// they're testing the same page instance and they're
	// faster when run sequentially.

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`
		<input id="text" type="text" value="something" />
		<input id="date" type="date" value="2012-03-12"/>
		<input id="number" type="number" value="42"/>
		<input id="unfillable" type="radio" />
	`, nil)
	require.NoError(t, err)

	happy := []struct{ name, selector, value string }{
		{name: "text", selector: "#text", value: "fill me up"},
		{name: "date", selector: "#date", value: "2012-03-13"},
		{name: "number", selector: "#number", value: "42"},
	}
	sad := []struct{ name, selector, value string }{
		{name: "date", selector: "#date", value: "invalid date"},
		{name: "number", selector: "#number", value: "forty two"},
		{name: "unfillable", selector: "#unfillable", value: "can't touch this"},
	}
	for _, tt := range happy {
		t.Run("happy/"+tt.name, func(t *testing.T) {
			err := p.Fill(tt.selector, tt.value, common.NewFrameFillOptions(p.MainFrame().Timeout()))
			require.NoError(t, err)
			inputValue, err := p.InputValue(tt.selector, common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
			require.NoError(t, err)
			require.Equal(t, tt.value, inputValue)
		})
	}
	for _, tt := range sad {
		t.Run("sad/"+tt.name, func(t *testing.T) {
			err := p.Fill(tt.selector, tt.value, common.NewFrameFillOptions(p.MainFrame().Timeout()))
			require.Error(t, err)
		})
	}
}

func TestPageIsChecked(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)

	err := p.SetContent(`<input type="checkbox" checked>`, nil)
	require.NoError(t, err)

	isopts := common.NewFrameIsCheckedOptions(common.DefaultTimeout)
	checked, err := p.IsChecked("input", isopts)
	require.NoError(t, err)
	assert.True(t, checked, "expected checkbox to be checked")

	err = p.SetContent(`<input type="checkbox">`, nil)
	require.NoError(t, err)

	isopts = common.NewFrameIsCheckedOptions(common.DefaultTimeout)
	checked, err = p.IsChecked("input", isopts)
	require.NoError(t, err)
	assert.False(t, checked, "expected checkbox to be unchecked")
}

func TestPageSetChecked(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`<input id="el" type="checkbox">`, nil)
	require.NoError(t, err)

	isopts := common.NewFrameIsCheckedOptions(common.DefaultTimeout)
	checked, err := p.IsChecked("#el", isopts)
	require.NoError(t, err)
	assert.False(t, checked)

	err = p.SetChecked("#el", true, common.NewFrameCheckOptions(p.MainFrame().Timeout()))
	require.NoError(t, err)
	isopts = common.NewFrameIsCheckedOptions(common.DefaultTimeout)
	checked, err = p.IsChecked("#el", isopts)
	require.NoError(t, err)
	assert.True(t, checked)

	err = p.SetChecked("#el", false, common.NewFrameCheckOptions(p.MainFrame().Timeout()))
	require.NoError(t, err)
	isopts = common.NewFrameIsCheckedOptions(common.DefaultTimeout)
	checked, err = p.IsChecked("#el", isopts)
	require.NoError(t, err)
	assert.False(t, checked)
}

func TestPageScreenshotFullpage(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	s := &common.Size{Width: 1280, Height: 800}
	err := p.SetViewportSize(s)
	require.NoError(t, err)

	_, err = p.Evaluate(`
	() => {
		document.body.style.margin = '0';
		document.body.style.padding = '0';
		document.documentElement.style.margin = '0';
		document.documentElement.style.padding = '0';

		const div = document.createElement('div');
		div.style.width = '1280px';
		div.style.height = '800px';
		div.style.background = 'linear-gradient(to bottom, red, blue)';

		document.body.appendChild(div);
	}`)
	require.NoError(t, err)

	opts := common.NewPageScreenshotOptions()
	opts.FullPage = true
	buf, err := p.Screenshot(opts, &mockPersister{})
	require.NoError(t, err)

	reader := bytes.NewReader(buf)
	img, err := png.Decode(reader)
	assert.Nil(t, err)

	assert.Equal(t, 1280, img.Bounds().Max.X, "want: screenshot width is 1280px, got: %dpx", img.Bounds().Max.X)
	assert.Equal(t, 800, img.Bounds().Max.Y, "want: screenshot height is 800px, got: %dpx", img.Bounds().Max.Y)

	// Allow tolerance to account for differences in rendering between
	// different platforms and browsers. The goal is to ensure that the
	// screenshot is mostly red at the top and mostly blue at the bottom.
	r, _, b, _ := img.At(0, 0).RGBA()
	assert.Truef(t, r > b*2, "want: the top pixel to be dominantly red, got R: %d, B: %d", r, b)
	r, _, b, _ = img.At(0, 799).RGBA()
	assert.Truef(t, b > r*2, "want: the bottom pixel to be dominantly blue, got R: %d, B: %d", r, b)
}

func TestPageTitle(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`<html><head><title>Some title</title></head></html>`, nil)
	require.NoError(t, err)
	title, err := p.Title()
	require.NoError(t, err)
	assert.Equal(t, "Some title", title)
}

func TestPageSetExtraHTTPHeaders(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t, withHTTPServer())

	p := b.NewPage(nil)

	headers := map[string]string{
		"Some-Header": "Some-Value",
	}
	err := p.SetExtraHTTPHeaders(headers)
	require.NoError(t, err)

	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	resp, err := p.Goto(
		b.url("/get"),
		opts,
	)
	require.NoError(t, err)
	require.NotNil(t, resp)

	responseBody, err := resp.Body()
	require.NoError(t, err)

	var body struct{ Headers map[string][]string }
	err = json.Unmarshal(responseBody, &body)
	require.NoError(t, err)

	h := body.Headers["Some-Header"]
	require.NotEmpty(t, h)
	assert.Equal(t, "Some-Value", h[0])
}

func TestPageWaitForFunction(t *testing.T) {
	t.Parallel()

	// script is here to test we're not getting an error from the
	// waitForFunction call itself and the tests that use it are
	// testing the polling functionality—not the response from
	// waitForFunction.
	script := `
		page = await browser.newPage();
		let resp = await page.waitForFunction(%s, %s, %s);
		log('ok: '+resp);`

	setup := func(t *testing.T) *testBrowser {
		tb := newTestBrowser(t, withLogCache())
		tb.vu.ActivateVU()
		tb.vu.StartIteration(t)
		t.Cleanup(func() { tb.vu.EndIteration(t) })
		require.NoError(t, tb.vu.Runtime().Set("log", func(s string) { tb.vu.State().Logger.Warn(s) }))
		return tb
	}

	t.Run("ok_func_raf_default", func(t *testing.T) {
		t.Parallel()
		tb := setup(t)

		tb.vu.SetVar(t, "page", &sobek.Object{})
		_, err := tb.vu.RunOnEventLoop(t, `fn = () => {
			if (typeof window._cnt == 'undefined') window._cnt = 0;
			if (window._cnt >= 50) return true;
			window._cnt++;
			return false;
		}`)
		require.NoError(t, err)

		_, err = tb.vu.RunAsync(t, script, "fn", "{}", "null")
		require.NoError(t, err)
		tb.logCache.assertContains(t, "ok: null")
	})

	t.Run("ok_func_raf_default_arg", func(t *testing.T) {
		t.Parallel()

		tb := setup(t)

		_, err := tb.vu.RunOnEventLoop(t, `fn = arg => {
			window._arg = arg;
			return true;
		}`)
		require.NoError(t, err)

		_, err = tb.vu.RunAsync(t, script, "fn", "{}", `"raf_arg"`)
		require.NoError(t, err)
		tb.logCache.contains("ok: null")

		p := tb.vu.RunPromise(t, `return await page.evaluate(() => window._arg);`)
		require.Equal(t, sobek.PromiseStateFulfilled, p.State())
		assert.Equal(t, "raf_arg", p.Result().String())
	})

	t.Run("ok_func_raf_default_args", func(t *testing.T) {
		t.Parallel()

		tb := setup(t)

		_, err := tb.vu.RunOnEventLoop(t, `fn = (...args) => {
			window._args = args;
			return true;
		}`)
		require.NoError(t, err)

		args := []int{1, 2, 3}
		argsJS, err := json.Marshal(args)
		require.NoError(t, err)

		_, err = tb.vu.RunAsync(t, script, "fn", "{}", "..."+string(argsJS))
		require.NoError(t, err)
		tb.logCache.contains("ok: null")

		p := tb.vu.RunPromise(t, `return await page.evaluate(() => window._args);`)
		require.Equal(t, sobek.PromiseStateFulfilled, p.State())
		var gotArgs []int
		_ = tb.vu.Runtime().ExportTo(p.Result(), &gotArgs)
		assert.Equal(t, args, gotArgs)
	})

	t.Run("err_expr_raf_timeout", func(t *testing.T) {
		t.Parallel()

		tb := setup(t)

		_, err := tb.vu.RunAsync(t, script, "false", "{ polling: 'raf', timeout: 500 }", "null")
		require.ErrorContains(t, err, "timed out after 500ms")
	})

	t.Run("err_wrong_polling", func(t *testing.T) {
		t.Parallel()

		tb := setup(t)

		_, err := tb.vu.RunAsync(t, script, "false", "{ polling: 'blah' }", "null")
		require.Error(t, err)
		assert.Contains(t, err.Error(),
			`parsing waitForFunction options: wrong polling option value:`,
			`"blah"; possible values: "raf", "mutation" or number`)
	})

	t.Run("ok_expr_poll_interval", func(t *testing.T) {
		t.Parallel()

		tb := setup(t)
		tb.vu.SetVar(t, "page", &sobek.Object{})
		_, err := tb.vu.RunAsync(t, `
			page = await browser.newPage();
			await page.evaluate(() => {
				setTimeout(() => {
					const el = document.createElement('h1');
					el.innerHTML = 'Hello';
					document.body.appendChild(el);
				}, 1000);
			});`,
		)
		require.NoError(t, err)

		script := `
			let resp = await page.waitForFunction(%s, %s, %s);
			if (resp) {
				log('ok: '+resp.innerHTML());
			} else {
				log('err: '+err);
			}`
		_, err = tb.vu.RunAsync(t, script, `"document.querySelector('h1')"`, "{ polling: 100, timeout: 2000, }", "null")
		require.NoError(t, err)
		tb.logCache.contains("ok: Hello")
	})

	t.Run("ok_func_poll_mutation", func(t *testing.T) {
		t.Parallel()

		tb := setup(t)

		tb.vu.SetVar(t, "page", &sobek.Object{})
		_, err := tb.vu.RunAsync(t, `
			fn = () => document.querySelector('h1') !== null

			page = await browser.newPage();
			await page.evaluate(() => {
				console.log('calling setTimeout...');
				setTimeout(() => {
					console.log('creating element...');
					const el = document.createElement('h1');
					el.innerHTML = 'Hello';
					document.body.appendChild(el);
				}, 1000);
			})`,
		)
		require.NoError(t, err)

		script := `
			let resp = await page.waitForFunction(%s, %s, %s);
			log('ok: '+resp);`

		_, err = tb.vu.RunAsync(t, script, "fn", "{ polling: 'mutation', timeout: 2000, }", "null")
		require.NoError(t, err)
		tb.logCache.assertContains(t, "ok: null")
	})
}

func TestPageWaitForLoadState(t *testing.T) {
	t.Parallel()

	t.Run("err_wrong_event", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		err := p.WaitForLoadState("none", common.NewFrameWaitForLoadStateOptions(p.MainFrame().Timeout()))
		require.ErrorContains(t, err, `invalid lifecycle event: "none"; must be one of: load, domcontentloaded, networkidle`)
	})
}

// See: The issue #187 for details.
func TestPageWaitForNavigationErrOnCtxDone(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t)
	p := b.NewPage(nil)
	go b.cancelContext()
	<-b.context().Done()
	_, err := p.WaitForNavigation(
		common.NewFrameWaitForNavigationOptions(p.Timeout()), nil)
	require.ErrorContains(t, err, "canceled")
}

func TestPagePress(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)

	p := tb.NewPage(nil)

	err := p.SetContent(`<input id="text1">`, nil)
	require.NoError(t, err)

	opts := common.NewFramePressOptions(p.MainFrame().Timeout())
	require.NoError(t, p.Press("#text1", "Shift+KeyA", opts))
	require.NoError(t, p.Press("#text1", "KeyB", opts))
	require.NoError(t, p.Press("#text1", "Shift+KeyC", opts))

	inputValue, err := p.InputValue("#text1", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
	require.NoError(t, err)
	require.Equal(t, "AbC", inputValue)
}

func TestPageURL(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t, withHTTPServer())

	p := b.NewPage(nil)
	uri, err := p.URL()
	require.NoError(t, err)
	assert.Equal(t, common.BlankPage, uri)

	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	resp, err := p.Goto(
		b.url("/get"),
		opts,
	)
	require.NoError(t, err)
	require.NotNil(t, resp)
	uri, err = p.URL()
	require.NoError(t, err)
	assert.Regexp(t, "http://.*/get", uri)
}

func TestPageClose(t *testing.T) {
	t.Parallel()

	t.Run("page_from_browser", func(t *testing.T) {
		t.Parallel()

		b := newTestBrowser(t, withHTTPServer())

		p := b.NewPage(nil)

		err := p.Close()
		assert.NoError(t, err)
	})

	t.Run("page_from_browserContext", func(t *testing.T) {
		t.Parallel()

		b := newTestBrowser(t, withHTTPServer())

		c, err := b.NewContext(nil)
		require.NoError(t, err)
		p, err := c.NewPage()
		require.NoError(t, err)

		err = p.Close()
		assert.NoError(t, err)
	})
}

func TestPageOn(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		fun     string
		wantErr string
	}{
		"nil on('console') handler": {
			fun:     `page.on('console')`,
			wantErr: `TypeError: The "listener" argument must be a function`,
		},
		"valid on('console') handler": {
			fun: `page.on('console', () => {})`,
		},
		"nil on('metric') handler": {
			fun:     `page.on('metric')`,
			wantErr: `TypeError: The "listener" argument must be a function`,
		},
		"valid on('metric') handler": {
			fun: `page.on('metric', () => {})`,
		},
		"nil on('request') handler": {
			fun:     `page.on('request')`,
			wantErr: `TypeError: The "listener" argument must be a function`,
		},
		"valid on('request') handler": {
			fun: `page.on('request', () => {})`,
		},
		"nil on('response') handler": {
			fun:     `page.on('response')`,
			wantErr: `TypeError: The "listener" argument must be a function`,
		},
		"valid on('response') handler": {
			fun: `page.on('response', () => {})`,
		},
		"nil on('requestfailed') handler": {
			fun:     `page.on('requestfailed')`,
			wantErr: `TypeError: The "listener" argument must be a function`,
		},
		"valid on('requestfailed') handler": {
			fun: `page.on('requestfailed', () => {})`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())

			tb.vu.ActivateVU()
			tb.vu.StartIteration(t)
			defer tb.vu.EndIteration(t)

			gv, err := tb.vu.RunAsync(t, `
				const page = await browser.newPage();
				try {
					%s        // e.g. page.on('console', handler);
				} finally {
					await page.close();
				}
 			`, tt.fun)

			got := k6test.ToPromise(t, gv)

			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Equal(t, sobek.PromiseStateRejected, got.State())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, sobek.PromiseStateFulfilled, got.State())
			}
		})
	}
}

func TestPageOnConsole(t *testing.T) {
	t.Parallel()

	const blankPage = "about:blank"

	testCases := []struct {
		name      string
		consoleFn string
		assertFn  func(*testing.T, *common.ConsoleMessage)
	}{
		{
			name:      "on console.log",
			consoleFn: "() => console.log('this is a log message')",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "log", cm.Type)
				assert.Equal(t, "this is a log message", cm.Text)
				val, err := cm.Args[0].JSONValue()
				assert.NoError(t, err)
				assert.Equal(t, "this is a log message", val)
				uri, err := cm.Page.URL()
				require.NoError(t, err)
				assert.True(t, uri == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.debug",
			consoleFn: "() => console.debug('this is a debug message')",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "debug", cm.Type)
				assert.Equal(t, "this is a debug message", cm.Text)
				val, err := cm.Args[0].JSONValue()
				assert.NoError(t, err)
				assert.Equal(t, "this is a debug message", val)
				uri, err := cm.Page.URL()
				require.NoError(t, err)
				assert.True(t, uri == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.info",
			consoleFn: "() => console.info('this is an info message')",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "info", cm.Type)
				assert.Equal(t, "this is an info message", cm.Text)
				val, err := cm.Args[0].JSONValue()
				assert.NoError(t, err)
				assert.Equal(t, "this is an info message", val)
				uri, err := cm.Page.URL()
				require.NoError(t, err)
				assert.True(t, uri == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.error",
			consoleFn: "() => console.error('this is an error message')",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "error", cm.Type)
				assert.Equal(t, "this is an error message", cm.Text)
				val, err := cm.Args[0].JSONValue()
				assert.NoError(t, err)
				assert.Equal(t, "this is an error message", val)
				uri, err := cm.Page.URL()
				require.NoError(t, err)
				assert.True(t, uri == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.warn",
			consoleFn: "() => console.warn('this is a warning message')",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "warning", cm.Type)
				assert.Equal(t, "this is a warning message", cm.Text)
				val, err := cm.Args[0].JSONValue()
				assert.NoError(t, err)
				assert.Equal(t, "this is a warning message", val)
				uri, err := cm.Page.URL()
				require.NoError(t, err)
				assert.True(t, uri == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.dir",
			consoleFn: "() => console.dir(document.location)",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "dir", cm.Type)
				assert.Equal(t, "Location", cm.Text)
				uri, err := cm.Page.URL()
				require.NoError(t, err)
				assert.True(t, uri == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.dirxml",
			consoleFn: "() => console.dirxml(document.location)",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "dirxml", cm.Type)
				assert.Equal(t, "Location", cm.Text)
				uri, err := cm.Page.URL()
				require.NoError(t, err)
				assert.True(t, uri == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.table",
			consoleFn: "() => console.table([['Grafana', 'k6'], ['Grafana', 'Mimir']])",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "table", cm.Type)
				assert.Equal(t, "Array(2)", cm.Text)
				val, err := cm.Args[0].JSONValue()
				assert.NoError(t, err)
				assert.Equal(t, `[["Grafana","k6"],["Grafana","Mimir"]]`, val)
				uri, err := cm.Page.URL()
				require.NoError(t, err)
				assert.True(t, uri == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.trace",
			consoleFn: "() => console.trace('trace example')",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "trace", cm.Type)
				assert.Equal(t, "trace example", cm.Text)
				val, err := cm.Args[0].JSONValue()
				assert.NoError(t, err)
				assert.Equal(t, "trace example", val)
				uri, err := cm.Page.URL()
				require.NoError(t, err)
				assert.True(t, uri == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.clear",
			consoleFn: "() => console.clear()",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "clear", cm.Type)
				uri, err := cm.Page.URL()
				require.NoError(t, err)
				assert.True(t, uri == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.group",
			consoleFn: "() => console.group()",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "startGroup", cm.Type)
				assert.Equal(t, "console.group", cm.Text)
				uri, err := cm.Page.URL()
				require.NoError(t, err)
				assert.True(t, uri == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.groupCollapsed",
			consoleFn: "() => console.groupCollapsed()",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "startGroupCollapsed", cm.Type)
				assert.Equal(t, "console.groupCollapsed", cm.Text)
				uri, err := cm.Page.URL()
				require.NoError(t, err)
				assert.True(t, uri == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.groupEnd",
			consoleFn: "() => console.groupEnd()",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "endGroup", cm.Type)
				assert.Equal(t, "console.groupEnd", cm.Text)
				uri, err := cm.Page.URL()
				require.NoError(t, err)
				assert.True(t, uri == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.assert",
			consoleFn: "() => console.assert(2 == 3)", // Only writes to console if assertion is false
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "assert", cm.Type)
				assert.Equal(t, "console.assert", cm.Text)
				uri, err := cm.Page.URL()
				require.NoError(t, err)
				assert.True(t, uri == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.count (default label)",
			consoleFn: "() => console.count()", // default label
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "count", cm.Type)
				assert.Equal(t, "default: 1", cm.Text)
				uri, err := cm.Page.URL()
				require.NoError(t, err)
				assert.True(t, uri == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.count",
			consoleFn: "() => console.count('k6')",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "count", cm.Type)
				assert.Equal(t, "k6: 1", cm.Text)
				uri, err := cm.Page.URL()
				require.NoError(t, err)
				assert.True(t, uri == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.time",
			consoleFn: "() => { console.time('k6'); console.timeEnd('k6'); }",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "timeEnd", cm.Type)
				assert.Regexp(t, `^k6: [0-9]+\.[0-9]+`, cm.Text, `expected prefix "k6: <a float>" but got %q`, cm.Text)
				uri, err := cm.Page.URL()
				require.NoError(t, err)
				assert.True(t, uri == blankPage, "url is not %s", blankPage)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Use withSkipClose() opt as we will close it
			// manually to force the page.TaskQueue closing
			tb := newTestBrowser(t)
			p := tb.NewPage(nil)

			var (
				done1 = make(chan bool)
				done2 = make(chan bool)

				testTO = 2500 * time.Millisecond
			)

			// Console Messages should be multiplexed for every registered handler
			eventHandlerOne := func(event common.PageEvent) error {
				defer close(done1)
				tc.assertFn(t, event.ConsoleMessage)
				return nil
			}

			eventHandlerTwo := func(event common.PageEvent) error {
				defer close(done2)
				tc.assertFn(t, event.ConsoleMessage)
				return nil
			}

			// eventHandlerOne and eventHandlerTwo will be called from a
			// separate goroutine from within the page's async event loop.
			// This is why we need to wait on done1 and done2 to be closed.
			err := p.On("console", eventHandlerOne)
			require.NoError(t, err)

			err = p.On("console", eventHandlerTwo)
			require.NoError(t, err)

			_, err = p.Evaluate(tc.consoleFn)
			require.NoError(t, err)

			select {
			case <-done1:
			case <-time.After(testTO):
				assert.Fail(t, "test timed out before eventHandlerOne completed")
			}

			select {
			case <-done2:
			case <-time.After(testTO):
				assert.Fail(t, "test timed out before eventHandlerTwo completed")
			}
		})
	}
}

func TestPageTimeout(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                     string
		defaultTimeout           time.Duration
		defaultNavigationTimeout time.Duration
	}{
		{
			name:           "fail when timeout exceeds default timeout",
			defaultTimeout: 1 * time.Millisecond,
		},
		{
			name:                     "fail when timeout exceeds default navigation timeout",
			defaultNavigationTimeout: 1 * time.Millisecond,
		},
		{
			name:                     "default navigation timeout supersedes default timeout",
			defaultTimeout:           30 * time.Second,
			defaultNavigationTimeout: 1 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withHTTPServer())

			tb.withHandler("/slow", func(w http.ResponseWriter, _ *http.Request) {
				time.Sleep(100 * time.Millisecond)
				_, err := fmt.Fprintf(w, `sorry for being so slow`)
				require.NoError(t, err)
			})

			p := tb.NewPage(nil)

			var timeout time.Duration
			if tc.defaultTimeout != 0 {
				timeout = tc.defaultTimeout
				p.SetDefaultTimeout(tc.defaultTimeout.Milliseconds())
			}
			if tc.defaultNavigationTimeout != 0 {
				timeout = tc.defaultNavigationTimeout
				p.SetDefaultNavigationTimeout(tc.defaultNavigationTimeout.Milliseconds())
			}

			opts := &common.FrameGotoOptions{
				Timeout: timeout,
			}
			res, err := p.Goto(
				tb.url("/slow"),
				opts,
			)
			require.Nil(t, res)
			assert.ErrorContains(t, err, "timed out after")
		})
	}
}

func TestPageWaitForSelector(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip() // no idea but it doesn't work
	}

	testCases := []struct {
		name          string
		url           string
		opts          map[string]any
		customTimeout time.Duration
		selector      string
		errAssert     func(*testing.T, error)
	}{
		{
			name:     "should wait for selector",
			url:      "wait_for.html",
			selector: "#my-div",
			errAssert: func(t *testing.T, e error) {
				t.Helper()
				assert.Nil(t, e)
			},
		},
		{
			name: "should TO waiting for selector",
			url:  "wait_for.html",
			// set a timeout smaller than the time
			// it takes the element to show up
			customTimeout: time.Nanosecond,
			selector:      "#my-div",
			errAssert: func(t *testing.T, e error) {
				t.Helper()
				assert.ErrorContains(t, e, "timed out after")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())

			page := tb.NewPage(nil)
			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err := page.Goto(
				tb.staticURL(tc.url),
				opts,
			)
			require.NoError(t, err)

			timeout := page.MainFrame().Timeout()
			if tc.customTimeout != 0 {
				timeout = tc.customTimeout
			}

			_, err = page.WaitForSelector(tc.selector, common.NewFrameWaitForSelectorOptions(timeout))
			tc.errAssert(t, err)
		})
	}
}

func TestPageThrottleNetwork(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip() // windows timeouts
	}

	testCases := []struct {
		name                     string
		networkProfile           common.NetworkProfile
		wantMinRoundTripDuration int64
	}{
		{
			name: "none",
			networkProfile: common.NetworkProfile{
				Latency:  0,
				Download: -1,
				Upload:   -1,
			},
		},
		{
			// In the ping.html file, an async ping request is made. The time it takes
			// to perform the roundtrip of calling ping and getting the response is
			// measured and used to assert that Latency has been correctly used.
			name: "latency",
			networkProfile: common.NetworkProfile{
				Latency:  100,
				Download: -1,
				Upload:   -1,
			},
			wantMinRoundTripDuration: 100,
		},
		{
			// In the ping.html file, an async ping request is made, the ping response
			// returns the request body (around a 1MB). The time it takes to perform the
			// roundtrip of calling ping and getting the response body is measured and
			// used to assert that Download has been correctly used.
			name: "download",
			networkProfile: common.NetworkProfile{
				Latency:  0,
				Download: 1000,
				Upload:   -1,
			},
			wantMinRoundTripDuration: 1000,
		},
		{
			// In the ping.html file, an async ping request is made with around a 1MB body.
			// The time it takes to perform the roundtrip of calling ping is measured
			// and used to assert that Upload has been correctly used.
			name: "upload",
			networkProfile: common.NetworkProfile{
				Latency:  0,
				Download: -1,
				Upload:   1000,
			},
			wantMinRoundTripDuration: 1000,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())

			tb.withHandler("/ping", func(w http.ResponseWriter, req *http.Request) {
				defer func() {
					err := req.Body.Close()
					require.NoError(t, err)
				}()
				bb, err := io.ReadAll(req.Body)
				require.NoError(t, err)

				_, err = fmt.Fprint(w, string(bb))
				require.NoError(t, err)
			})

			page := tb.NewPage(nil)

			err := page.ThrottleNetwork(tc.networkProfile)
			require.NoError(t, err)

			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err = page.Goto(
				tb.staticURL("ping.html"),
				opts,
			)
			require.NoError(t, err)

			selector := `div[id="result"]`

			// result selector only appears once the page gets a response
			// from the async ping request.
			_, err = page.WaitForSelector(selector, common.NewFrameWaitForSelectorOptions(page.MainFrame().Timeout()))
			require.NoError(t, err)

			resp, err := page.InnerText(selector, common.NewFrameInnerTextOptions(page.MainFrame().Timeout()))
			require.NoError(t, err)
			ms, err := strconv.ParseInt(resp, 10, 64)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, ms, tc.wantMinRoundTripDuration)
		})
	}
}

// This test will first navigate to the ping.html site a few times, record the
// average time it takes to run the test. Next it will repeat the steps but
// first apply CPU throttling. The average duration with CPU throttling
// enabled should be longer than without it.
func TestPageThrottleCPU(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip() // windows timeouts
	}
	tb := newTestBrowser(t, withFileServer())

	tb.withHandler("/ping", func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			err := req.Body.Close()
			require.NoError(t, err)
		}()
		bb, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		_, err = fmt.Fprint(w, string(bb))
		require.NoError(t, err)
	})

	page := tb.NewPage(nil)
	const iterations = 5

	noCPUThrottle := performPingTest(t, tb, page, iterations)

	err := page.ThrottleCPU(common.CPUProfile{
		Rate: 50,
	})
	require.NoError(t, err)

	withCPUThrottle := performPingTest(t, tb, page, iterations)

	assert.Greater(t, withCPUThrottle, noCPUThrottle)
}

func performPingTest(t *testing.T, tb *testBrowser, page *common.Page, iterations int) int64 {
	t.Helper()

	var ms int64
	for range iterations {
		start := time.Now()

		opts := &common.FrameGotoOptions{
			Timeout: common.DefaultTimeout,
		}
		_, err := page.Goto(
			tb.staticURL("ping.html"),
			opts,
		)
		require.NoError(t, err)

		selector := `div[id="result"]`

		// result selector only appears once the page gets a response
		// from the async ping request.
		_, err = page.WaitForSelector(selector, common.NewFrameWaitForSelectorOptions(page.MainFrame().Timeout()))
		require.NoError(t, err)

		ms += time.Since(start).Abs().Milliseconds()
	}

	return ms / int64(iterations)
}

func TestPageIsVisible(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip() // timeouts
	}

	testCases := []struct {
		name     string
		selector string
		options  common.FrameIsVisibleOptions
		want     bool
		wantErr  string
	}{
		{
			name:     "visible",
			selector: "div[id=my-div]",
			want:     true,
		},
		{
			name:     "not_visible",
			selector: "div[id=my-div-3]",
			want:     false,
		},
		{
			name:     "not_found",
			selector: "div[id=does-not-exist]",
			want:     false,
		},
		{
			name:     "first_div",
			selector: "div",
			want:     true,
		},
		{
			name:     "first_div",
			selector: "div",
			options: common.FrameIsVisibleOptions{
				Strict: true,
			},
			wantErr: "error:strictmodeviolation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())

			page := tb.NewPage(nil)

			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err := page.Goto(
				tb.staticURL("visible.html"),
				opts,
			)
			require.NoError(t, err)

			got, err := page.IsVisible(tc.selector, &tc.options)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPageIsHidden(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip() // timeouts
	}

	testCases := []struct {
		name     string
		selector string
		options  common.FrameIsHiddenOptions
		want     bool
		wantErr  string
	}{
		{
			name:     "hidden",
			selector: "div[id=my-div-3]",
			want:     true,
		},
		{
			name:     "visible",
			selector: "div[id=my-div]",
			want:     false,
		},
		{
			name:     "not_found",
			selector: "div[id=does-not-exist]",
			want:     true,
		},
		{
			name:     "first_div",
			selector: "div",
			want:     false,
		},
		{
			name:     "first_div",
			selector: "div",
			options: common.FrameIsHiddenOptions{
				Strict: true,
			},
			wantErr: "error:strictmodeviolation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())

			page := tb.NewPage(nil)

			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err := page.Goto(
				tb.staticURL("visible.html"),
				opts,
			)
			require.NoError(t, err)

			got, err := page.IsHidden(tc.selector, &tc.options)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestShadowDOMAndDocumentFragment(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("windows timeouts on these tests")
	}
	tests := []struct {
		name     string
		selector string
		want     string
	}{
		{
			// This test waits for an element that is in the DocumentFragment.
			name:     "waitFor_DocumentFragment",
			selector: `//p[@id="inDocFrag"]`,
			want:     "This text is added via a document fragment!",
		},
		{
			// This test waits for an element that is in the DocumentFragment
			// that is within an open shadow root.
			name:     "waitFor_ShadowRoot_DocumentFragment",
			selector: `//p[@id="inShadowRootDocFrag"]`,
			want:     "This is inside Shadow DOM, added via a DocumentFragment!",
		},
		{
			// This test waits for an element that is in the original Document.
			name:     "waitFor_done",
			selector: `//div[@id="done"]`,
			want:     "All additions to page completed (i'm in the original document)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			tb.vu.ActivateVU()
			tb.vu.StartIteration(t)
			defer tb.vu.EndIteration(t)

			got := tb.vu.RunPromise(t, `
				const p = await browser.newPage()
				await p.goto("%s")

				const s = p.locator('%s')
				await s.waitFor({
					timeout: 1000,
					state: 'attached',
				});

				const text = await s.innerText();
				await p.close()
				return text;
 			`, tb.staticURL("shadow_and_doc_frag.html"), tt.selector)
			assert.Equal(t, tt.want, got.Result().String())
		})
	}
}

func TestPageTargetBlank(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())
	tb.withHandler("/home", func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(
			`<!DOCTYPE html><html><head></head><body>
				<a href="/link" target="_blank">click me</a>
			</body></html>`,
		))
		require.NoError(t, err)
	})
	tb.withHandler("/link", func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write(
			[]byte(`<!DOCTYPE html><html><head></head><body><h1>you clicked!</h1></body></html>`),
		)
		require.NoError(t, err)
	})

	p := tb.NewPage(nil)

	// Navigate to the page with a link that opens a new page.
	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	resp, err := p.Goto(tb.url("/home"), opts)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Current page count should be 1.
	pp := p.Context().Pages()
	assert.Equal(t, 1, len(pp))

	// This link should open the link on a new page.
	err = p.Click("a[href='/link']", common.NewFrameClickOptions(p.Timeout()))
	require.NoError(t, err)

	// Wait for the page to be created and for it to navigate to the link.
	obj, err := p.Context().WaitForEvent("page", nil, common.DefaultTimeout)
	require.NoError(t, err)
	p2, ok := obj.(*common.Page)
	require.True(t, ok, "return from WaitForEvent is not a Page")

	err = p2.WaitForLoadState(common.LifecycleEventLoad.String(), common.NewFrameWaitForLoadStateOptions(p.MainFrame().Timeout()))
	require.NoError(t, err)

	// Now there should be 2 pages.
	pp = p.Context().Pages()
	assert.Equal(t, 2, len(pp))

	// Make sure the new page contains the correct page.
	got, err := p2.InnerHTML("h1", common.NewFrameInnerHTMLOptions(p.MainFrame().Timeout()))
	require.NoError(t, err)
	assert.Equal(t, "you clicked!", got)
}

func TestPageGetAttribute(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`<a id="el" href="null">Something</a>`, nil)
	require.NoError(t, err)

	got, ok, err := p.GetAttribute("#el", "href", common.NewFrameBaseOptions(p.MainFrame().Timeout()))
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "null", got)
}

func TestPageGetAttributeMissing(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`<a id="el">Something</a>`, nil)
	require.NoError(t, err)

	got, ok, err := p.GetAttribute("#el", "missing", common.NewFrameBaseOptions(p.MainFrame().Timeout()))
	require.NoError(t, err)
	require.False(t, ok)
	assert.Equal(t, "", got)
}

func TestPageGetAttributeEmpty(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`<a id="el" empty>Something</a>`, nil)
	require.NoError(t, err)

	got, ok, err := p.GetAttribute("#el", "empty", common.NewFrameBaseOptions(p.MainFrame().Timeout()))
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "", got)
}

func TestPageOnMetric(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fun       string
		want      string
		wantRegex string
		wantErr   string
	}{
		{
			// Just a single page.on.
			name: "single_page.on",
			fun: `page.on('metric', (metric) => {
				metric.tag({
				  	name:'ping-1',
					matches: [
						{url: /^http:\/\/127\.0\.0\.1\:[0-9]+\/ping\?h=[0-9a-z]+$/},
					]
				});
			});`,
			want: "ping-1",
		},
		{
			// A single page.on but with multiple calls to Tag.
			name: "multi_tag",
			fun: `page.on('metric', (metric) => {
				metric.tag({
					name:'ping-1',
					matches: [
						{url: /^http:\/\/127\.0\.0\.1\:[0-9]+\/ping\?h=[0-9a-z]+$/},
					]
				});
				metric.tag({
					name:'ping-2',
					matches: [
						{url: /^http:\/\/127\.0\.0\.1\:[0-9]+\/ping\?h=[0-9a-z]+$/},
					]
				  });
			});`,
			want: "ping-2",
		},
		{
			// Two page.on and in one of them multiple calls to Tag.
			name: "multi_tag_page.on",
			fun: `page.on('metric', (metric) => {
				metric.tag({
					name:'ping-1',
					matches: [
						{url: /^http:\/\/127\.0\.0\.1\:[0-9]+\/ping\?h=[0-9a-z]+$/},
					]
				});
				metric.tag({
					name:'ping-2',
					matches: [
						  {url: /^http:\/\/127\.0\.0\.1\:[0-9]+\/ping\?h=[0-9a-z]+$/},
					  ]
				  });
			});
			page.on('metric', (metric) => {
				metric.tag({
					name:'ping-3',
					matches: [
						{url: /^http:\/\/127\.0\.0\.1\:[0-9]+\/ping\?h=[0-9a-z]+$/},
					]
				});
			});`,
			want: "ping-3",
		},
		{
			// A single page.on but within it another page.on.
			name: "multi_page.on_call",
			fun: `page.on('metric', (metric) => {
				metric.tag({
					name:'ping-1',
					matches: [
						{url: /^http:\/\/127\.0\.0\.1\:[0-9]+\/ping\?h=[0-9a-z]+$/},
					]
				});
				page.on('metric', (metric) => {
					metric.tag({
						name:'ping-4',
						matches: [
							{url: /^http:\/\/127\.0\.0\.1\:[0-9]+\/ping\?h=[0-9a-z]+$/},
						]
					});
				});
			});`,
			want: "ping-4",
		},
		{
			// With method field GET, which is the correct method for the request.
			name: "with_method",
			fun: `page.on('metric', (metric) => {
				metric.tag({
					name:'ping-1',
					matches: [
						{url: /^http:\/\/127\.0\.0\.1\:[0-9]+\/ping\?h=[0-9a-z]+$/, method: 'GET'},
					]
				});
			});`,
			want: "ping-1",
		},
		{
			// With method field " get ", which is to ensure it is internally
			// converted to "GET" before comparing.
			name: "lowercase_needs_trimming",
			fun: `page.on('metric', (metric) => {
				metric.tag({
					name:'ping-1',
					matches: [
						{url: /^http:\/\/127\.0\.0\.1\:[0-9]+\/ping\?h=[0-9a-z]+$/, method: ' get '},
					]
				});
			});`,
			want: "ping-1",
		},
		{
			// When supplying the wrong request method (POST) when it should be GET.
			// In this case the URLs aren't grouped.
			name: "wrong_method_should_skip_method_comparison",
			fun: `page.on('metric', (metric) => {
				metric.tag({
					name:'ping-1',
					matches: [
						{url: /^http:\/\/127\.0\.0\.1\:[0-9]+\/ping\?h=[0-9a-z]+$/, method: 'POST'},
					]
				});
			});`,
			wantRegex: `http://127\.0\.0\.1:[0-9]+/ping\?h=[0-9a-z]+`,
		},
		{
			// We should get an error back when the name is invalid (empty string)
			name: "with_invalid_name",
			fun: `page.on('metric', (metric) => {
				metric.tag({
					name:'  ',
					matches: [
						{url: /^http:\/\/127\.0\.0\.1\:[0-9]+\/ping\?h=[0-9a-z]+$/, method: 'GET'},
					]
				});
			});`,
			wantRegex: `http://127\.0\.0\.1:[0-9]+/ping\?h=[0-9a-z]+`,
			wantErr:   `name "  " is invalid`,
		},
		{
			// We should get an error back when the method is invalid.
			name: "with_invalid_name",
			fun: `page.on('metric', (metric) => {
				metric.tag({
					name:'ping-1',
					matches: [
						{url: /^http:\/\/127\.0\.0\.1\:[0-9]+\/ping\?h=[0-9a-z]+$/, method: 'foo'},
					]
				});
			});`,
			wantRegex: `http://127\.0\.0\.1:[0-9]+/ping\?h=[0-9a-z]+`,
			wantErr:   `method "foo" is invalid`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var foundAmended atomic.Int32
			var foundUnamended atomic.Int32

			done := make(chan bool)

			samples := make(chan k6metrics.SampleContainer)
			// This page will perform many pings with a changing h query parameter.
			// This URL should be grouped according to how page.on('metric') is used.
			tb := newTestBrowser(t, withHTTPServer(), withSamples(samples))
			tb.withHandler("/home", func(w http.ResponseWriter, r *http.Request) {
				_, err := fmt.Fprintf(w, `
		<html>
			<head></head>
			<body>
				<script type="module">
					await ping();
					async function ping() {
						await fetch('/ping?h=2kq2lo6n06');
						await fetch('/ping?h=ej0ypprcjk');
					}
				</script>
			</body>
		</html>`)
				require.NoError(t, err)
			})
			tb.withHandler("/ping", func(w http.ResponseWriter, r *http.Request) {
				_, err := fmt.Fprintf(w, `pong`)
				require.NoError(t, err)
			})

			ignoreURLs := map[string]any{
				tb.url("/home"):        nil,
				tb.url("/favicon.ico"): nil,
			}
			go func() {
				defer close(done)
				for e := range samples {
					ss := e.GetSamples()
					for _, s := range ss {
						// At the moment all metrics that the browser emits contains
						// both a url and name tag on each metric.
						u, ok := s.Tags.Get("url")
						assert.True(t, ok)
						n, ok := s.Tags.Get("name")
						assert.True(t, ok)

						// The name and url tags should have the same value.
						assert.Equal(t, u, n)

						// If the url is in the ignoreURLs map then this will
						// not have been matched on by the regex, so continue.
						if _, ok := ignoreURLs[u]; ok {
							foundUnamended.Add(1)
							continue
						}

						// Url shouldn't contain any of the hash values, and should
						// instead take the name that was supplied in the Tag
						// function on metric in page.on.
						if tt.wantRegex != "" {
							assert.Regexp(t, tt.wantRegex, u)
						} else {
							assert.Equal(t, tt.want, u)
						}

						foundAmended.Add(1)
					}
				}
			}()

			tb.vu.ActivateVU()
			tb.vu.StartIteration(t)
			defer tb.vu.EndIteration(t)

			// Some of the business logic is in the mapping layer unfortunately.
			// To test everything is wried up correctly, we're required to work
			// with RunPromise.
			gv, err := tb.vu.RunAsync(t, `
				const page = await browser.newPage()

				%s

				await page.goto('%s', {waitUntil: 'networkidle'});

				await page.close()
			`, tt.fun, tb.url("/home"))

			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}

			got := k6test.ToPromise(t, gv)

			assert.True(t, got.Result().Equals(sobek.Null()))

			close(samples)

			<-done

			// We want to make sure that we found at least one occurrence
			// of a metric which matches our expectations.
			assert.True(t, foundAmended.Load() > 0)

			// We want to make sure that we found at least one occurrence
			// of a metric which didn't match our expectations.
			assert.True(t, foundUnamended.Load() > 0)
		})
	}
}

func TestPageOnRequest(t *testing.T) {
	t.Parallel()

	// Start and setup a webserver to test the page.on('request') handler.
	tb := newTestBrowser(t, withHTTPServer())

	tb.withHandler("/home", func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <link rel="stylesheet" href="/style.css">
</head>
<body>
    <script>fetch('/api', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({name: 'tester'})
    })</script>
</body>
</html>`)
		require.NoError(t, err)
	})
	tb.withHandler("/api", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var data struct {
			Name string `json:"name"`
		}
		err = json.Unmarshal(body, &data)
		require.NoError(t, err)

		_, err = fmt.Fprintf(w, `{"message": "Hello %s!"}`, data.Name)
		require.NoError(t, err)
	})
	tb.withHandler("/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		_, err := fmt.Fprintf(w, `body { background-color: #f0f0f0; }`)
		require.NoError(t, err)
	})

	tb.vu.ActivateVU()
	tb.vu.StartIteration(t)
	defer tb.vu.EndIteration(t)

	// Some of the business logic is in the mapping layer unfortunately.
	// To test everything is wried up correctly, we're required to work
	// with RunPromise.
	//
	// The code below is the JavaScript code that is executed in the k6 iteration.
	// It will wait for all requests to be captured in returnValue, before returning.
	gv, err := tb.vu.RunAsync(t, `
		const context = await browser.newContext({locale: 'en-US', userAgent: 'some-user-agent'});
		const page = await context.newPage();

		var returnValue = [];
		page.on('request', async (request) => {
			returnValue.push({
				allHeaders: await request.allHeaders(),
				frameUrl: request.frame().url(),
				acceptLanguageHeader: await request.headerValue('Accept-Language'),
				headers: request.headers(),
				headersArray: await request.headersArray(),
				isNavigationRequest: request.isNavigationRequest(),
				method: request.method(),
				postData: request.postData(),
				postDataBuffer: request.postDataBuffer() ? String.fromCharCode.apply(null, new Uint8Array(request.postDataBuffer())) : null,
				resourceType: request.resourceType(),
				// Ignoring response for now since it is not reliable as we don't explicitly wait for the request to finish.
				// response: await request.response(),
				size: request.size(),
				// Ignoring timing for now since it is not reliable as we don't explicitly wait for the request to finish.
				// timing: request.timing(),
				url: request.url()
			});
		});

		await page.goto('%s', {waitUntil: 'networkidle'});

		await page.close();

		return JSON.stringify(returnValue, null, 2);
	`, tb.url("/home"))
	assert.NoError(t, err)

	got := k6test.ToPromise(t, gv)

	// Convert the result to a string and then to a slice of requests.
	var requests []request
	err = json.Unmarshal([]byte(got.Result().String()), &requests)
	require.NoError(t, err)

	expected := []request{
		{
			AllHeaders: map[string]string{
				"accept-language":           "en-US",
				"upgrade-insecure-requests": "1",
				"user-agent":                "some-user-agent",
			},
			FrameURL:             "about:blank",
			AcceptLanguageHeader: "en-US",
			Headers: map[string]string{
				"Accept-Language":           "en-US",
				"Upgrade-Insecure-Requests": "1",
				"User-Agent":                "some-user-agent",
			},
			HeadersArray: []map[string]string{
				{"name": "Upgrade-Insecure-Requests", "value": "1"},
				{"name": "User-Agent", "value": "some-user-agent"},
				{"name": "Accept-Language", "value": "en-US"},
			},
			IsNavigationRequest: true,
			Method:              "GET",
			PostData:            "",
			PostDataBuffer:      "",
			ResourceType:        "Document",
			Size: map[string]int{
				"body":    0,
				"headers": 103,
			},
			URL: tb.url("/home"),
		},
		{
			AllHeaders: map[string]string{
				"accept-language": "en-US",
				"referer":         tb.url("/home"),
				"user-agent":      "some-user-agent",
			},
			FrameURL:             tb.url("/home"),
			AcceptLanguageHeader: "en-US",
			Headers: map[string]string{
				"Accept-Language": "en-US",
				"Referer":         tb.url("/home"),
				"User-Agent":      "some-user-agent",
			},
			HeadersArray: []map[string]string{
				{"name": "User-Agent", "value": "some-user-agent"},
				{"name": "Accept-Language", "value": "en-US"},
				{"name": "Referer", "value": tb.url("/home")},
			},
			IsNavigationRequest: false,
			Method:              "GET",
			PostData:            "",
			PostDataBuffer:      "",
			ResourceType:        "Stylesheet",
			Size: map[string]int{
				"body":    0,
				"headers": 116,
			},
			URL: tb.url("/style.css"),
		},
		{
			AllHeaders: map[string]string{
				"accept-language": "en-US",
				"content-type":    "application/json",
				"referer":         tb.url("/home"),
				"user-agent":      "some-user-agent",
			},
			FrameURL:             tb.url("/home"),
			AcceptLanguageHeader: "en-US",
			Headers: map[string]string{
				"Accept-Language": "en-US",
				"Content-Type":    "application/json",
				"Referer":         tb.url("/home"),
				"User-Agent":      "some-user-agent",
			},
			HeadersArray: []map[string]string{
				{"name": "Referer", "value": tb.url("/home")},
				{"name": "User-Agent", "value": "some-user-agent"},
				{"name": "Accept-Language", "value": "en-US"},
				{"name": "Content-Type", "value": "application/json"},
			},
			IsNavigationRequest: false,
			Method:              "POST",
			PostData:            `{"name":"tester"}`,
			PostDataBuffer:      `{"name":"tester"}`,
			ResourceType:        "Fetch",
			Size: map[string]int{
				"body":    17,
				"headers": 143,
			},
			URL: tb.url("/api"),
		},
		{
			AllHeaders: map[string]string{
				"accept-language": "en-US",
				"referer":         tb.url("/home"),
				"user-agent":      "some-user-agent",
			},
			FrameURL:             tb.url("/home"),
			AcceptLanguageHeader: "en-US",
			Headers: map[string]string{
				"Accept-Language": "en-US",
				"Referer":         tb.url("/home"),
				"User-Agent":      "some-user-agent",
			},
			HeadersArray: []map[string]string{
				{"name": "Accept-Language", "value": "en-US"},
				{"name": "Referer", "value": tb.url("/home")},
				{"name": "User-Agent", "value": "some-user-agent"},
			},
			IsNavigationRequest: false,
			Method:              "GET",
			PostData:            "",
			PostDataBuffer:      "",
			ResourceType:        "Other",
			Size: map[string]int{
				"body":    0,
				"headers": 118,
			},
			URL: tb.url("/favicon.ico"),
		},
	}

	// Compare each request one by one for better test failure visibility
	for _, req := range requests {
		i := slices.IndexFunc(expected, func(r request) bool { return req.URL == r.URL })
		assert.NotEqual(t, -1, i, "failed to find expected request with URL %s", req.URL)

		sortByName := func(m1, m2 map[string]string) int {
			return strings.Compare(m1["name"], m2["name"])
		}
		slices.SortFunc(req.HeadersArray, sortByName)
		slices.SortFunc(expected[i].HeadersArray, sortByName)
		assert.Equal(t, expected[i], req)
	}
}

type request struct {
	AllHeaders           map[string]string   `json:"allHeaders"`
	FrameURL             string              `json:"frameUrl"`
	AcceptLanguageHeader string              `json:"acceptLanguageHeader"`
	Headers              map[string]string   `json:"headers"`
	HeadersArray         []map[string]string `json:"headersArray"`
	IsNavigationRequest  bool                `json:"isNavigationRequest"`
	Method               string              `json:"method"`
	PostData             string              `json:"postData"`
	PostDataBuffer       string              `json:"postDataBuffer"`
	ResourceType         string              `json:"resourceType"`
	Size                 map[string]int      `json:"size"`
	URL                  string              `json:"url"`
}

type response struct {
	AllHeaders            map[string]string      `json:"allHeaders"`
	Body                  string                 `json:"body"`
	FrameURL              string                 `json:"frameUrl"`
	AcceptLanguageHeader  string                 `json:"acceptLanguageHeader"`
	AcceptLanguageHeaders []string               `json:"acceptLanguageHeaders"`
	Headers               map[string]string      `json:"headers"`
	HeadersArray          []map[string]string    `json:"headersArray"`
	JSON                  string                 `json:"json"`
	OK                    bool                   `json:"ok"`
	RequestURL            string                 `json:"requestUrl"`
	SecurityDetails       common.SecurityDetails `json:"securityDetails"`
	ServerAddr            common.RemoteAddress   `json:"serverAddr"`
	Size                  map[string]int         `json:"size"`
	Status                int64                  `json:"status"`
	StatusText            string                 `json:"statusText"`
	URL                   string                 `json:"url"`
	Text                  string                 `json:"text"`
}

func TestPageOnResponse(t *testing.T) {
	t.Parallel()

	// Start and setup a webserver to test the page.on('request') handler.
	tb := newTestBrowser(t, withHTTPServer())

	tb.withHandler("/home", func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <link rel="stylesheet" href="/style.css">
</head>
<body>
    <script>fetch('/api', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({name: 'tester'})
    })</script>
</body>
</html>`)
		require.NoError(t, err)
	})
	tb.withHandler("/api", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		defer require.NoError(t, r.Body.Close())

		var data struct {
			Name string `json:"name"`
		}
		err = json.Unmarshal(body, &data)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		_, err = fmt.Fprintf(w, `{"message": "Hello %s!"}`, data.Name)
		require.NoError(t, err)
	})
	tb.withHandler("/style.css", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		_, err := fmt.Fprintf(w, `body { background-color: #f0f0f0; }`)
		require.NoError(t, err)
	})

	tb.vu.ActivateVU()
	tb.vu.StartIteration(t)
	defer tb.vu.EndIteration(t)
	// Some of the business logic is in the mapping layer unfortunately.
	// To test everything is wried up correctly, we're required to work
	// with RunPromise.
	//
	// The code below is the JavaScript code that is executed in the k6 iteration.
	// It will wait for all requests to be captured in returnValue, before returning.
	gv, err := tb.vu.RunAsync(t, `
		const context = await browser.newContext({locale: 'en-US', userAgent: 'some-user-agent'});
		const page = await context.newPage();

		var returnValue = [];
		page.on('response', async (response) => {
			// We need to check if the response is JSON before calling json()
			const allHeaders = await response.allHeaders();
			var json = null;
			if (allHeaders["content-type"] === "application/json") {
				json = await response.json();
			}

			returnValue.push({
				allHeaders: allHeaders,
				body: await response.body() ? String.fromCharCode.apply(null, new Uint8Array(await response.body())) : null,
				frameUrl: response.frame().url(),
				acceptLanguageHeader: await response.headerValue('Accept-Language'),
				acceptLanguageHeaders: await response.headerValues('Accept-Language'),
				headers: response.headers(),
				headersArray: await response.headersArray(),
				json: JSON.stringify(json),
				ok: response.ok(),
				requestUrl: response.request().url(),
				securityDetails: await response.securityDetails(),
				serverAddr: await response.serverAddr(),
				size: await response.size(),
				status: response.status(),
				statusText: response.statusText(),
				url: response.url(),
				text: await response.text()
			});
		})

		await page.goto('%s', {waitUntil: 'networkidle'});

		await page.close();

		return JSON.stringify(returnValue, null, 2);
	`, tb.url("/home"))
	require.NoError(t, err)

	got := k6test.ToPromise(t, gv)

	// Convert the result to a string and then to a slice of requests.
	var responses []response
	err = json.Unmarshal([]byte(got.Result().String()), &responses)
	require.NoError(t, err)

	// Normalize the date
	for i := range responses {
		for k := range responses[i].AllHeaders {
			if strings.Contains(strings.ToLower(k), "date") {
				responses[i].AllHeaders[k] = "Wed, 29 Jan 2025 09:00:00 GMT"
			}
		}
		for k := range responses[i].Headers {
			if strings.Contains(strings.ToLower(k), "date") {
				responses[i].Headers[k] = "Wed, 29 Jan 2025 09:00:00 GMT"
			}
		}
		for k, header := range responses[i].HeadersArray {
			if strings.Contains(strings.ToLower(header["name"]), "date") {
				responses[i].HeadersArray[k]["value"] = "Wed, 29 Jan 2025 09:00:00 GMT"
			}
		}
	}

	serverURL := tb.http.ServerHTTP.URL
	host, p, err := net.SplitHostPort(strings.TrimPrefix(serverURL, "http://"))
	require.NoError(t, err)

	port, err := strconv.ParseInt(p, 10, 64)
	require.NoError(t, err)

	expected := []response{
		{
			AllHeaders: map[string]string{
				"content-length": "286",
				"content-type":   "text/html; charset=utf-8",
				"date":           "Wed, 29 Jan 2025 09:00:00 GMT",
			},
			Body:                  "<!DOCTYPE html>\n<html>\n<head>\n    <link rel=\"stylesheet\" href=\"/style.css\">\n</head>\n<body>\n    <script>fetch('/api', {\n      method: 'POST',\n      headers: {\n        'Content-Type': 'application/json'\n      },\n      body: JSON.stringify({name: 'tester'})\n    })</script>\n</body>\n</html>",
			FrameURL:              tb.url("/home"),
			AcceptLanguageHeader:  "",
			AcceptLanguageHeaders: []string{""},
			Headers: map[string]string{
				"Content-Length": "286",
				"Content-Type":   "text/html; charset=utf-8",
				"Date":           "Wed, 29 Jan 2025 09:00:00 GMT",
			},
			HeadersArray: []map[string]string{
				{"name": "Content-Length", "value": "286"},
				{"name": "Content-Type", "value": "text/html; charset=utf-8"},
				{"name": "Date", "value": "Wed, 29 Jan 2025 09:00:00 GMT"},
			},
			JSON:            "null",
			OK:              true,
			RequestURL:      tb.url("/home"),
			SecurityDetails: common.SecurityDetails{},
			ServerAddr:      common.RemoteAddress{IPAddress: host, Port: port},
			Size:            map[string]int{"body": 286, "headers": 117},
			Status:          200,
			StatusText:      "OK",
			URL:             tb.url("/home"),
			Text:            "<!DOCTYPE html>\n<html>\n<head>\n    <link rel=\"stylesheet\" href=\"/style.css\">\n</head>\n<body>\n    <script>fetch('/api', {\n      method: 'POST',\n      headers: {\n        'Content-Type': 'application/json'\n      },\n      body: JSON.stringify({name: 'tester'})\n    })</script>\n</body>\n</html>",
		},
		{
			AllHeaders: map[string]string{
				"content-length": "35",
				"content-type":   "text/css",
				"date":           "Wed, 29 Jan 2025 09:00:00 GMT",
			},
			Body:                  "body { background-color: #f0f0f0; }",
			FrameURL:              tb.url("/home"),
			AcceptLanguageHeader:  "",
			AcceptLanguageHeaders: []string{""},
			Headers: map[string]string{
				"Content-Length": "35",
				"Content-Type":   "text/css",
				"Date":           "Wed, 29 Jan 2025 09:00:00 GMT",
			},
			HeadersArray: []map[string]string{
				{"name": "Date", "value": "Wed, 29 Jan 2025 09:00:00 GMT"},
				{"name": "Content-Type", "value": "text/css"},
				{"name": "Content-Length", "value": "35"},
			},
			JSON:            "null",
			OK:              true,
			RequestURL:      tb.url("/style.css"),
			SecurityDetails: common.SecurityDetails{},
			ServerAddr:      common.RemoteAddress{IPAddress: host, Port: port},
			Size:            map[string]int{"body": 35, "headers": 100},
			Status:          200,
			StatusText:      "OK",
			URL:             tb.url("/style.css"),
			Text:            "body { background-color: #f0f0f0; }",
		},
		{
			AllHeaders: map[string]string{
				"access-control-allow-credentials": "true",
				"access-control-allow-origin":      "*",
				"content-length":                   "19",
				"content-type":                     "text/plain; charset=utf-8",
				"date":                             "Wed, 29 Jan 2025 09:00:00 GMT",
				"x-content-type-options":           "nosniff",
			},
			Body:                  "404 page not found\n",
			FrameURL:              tb.url("/home"),
			AcceptLanguageHeader:  "",
			AcceptLanguageHeaders: []string{""},
			Headers: map[string]string{
				"Access-Control-Allow-Credentials": "true",
				"Access-Control-Allow-Origin":      "*",
				"Content-Length":                   "19",
				"Content-Type":                     "text/plain; charset=utf-8",
				"Date":                             "Wed, 29 Jan 2025 09:00:00 GMT",
				"X-Content-Type-Options":           "nosniff",
			},
			HeadersArray: []map[string]string{
				{"name": "Date", "value": "Wed, 29 Jan 2025 09:00:00 GMT"},
				{"name": "Content-Type", "value": "text/plain; charset=utf-8"},
				{"name": "Access-Control-Allow-Credentials", "value": "true"},
				{"name": "X-Content-Type-Options", "value": "nosniff"},
				{"name": "Access-Control-Allow-Origin", "value": "*"},
				{"name": "Content-Length", "value": "19"},
			},
			JSON:            "null",
			OK:              false,
			RequestURL:      tb.url("/favicon.ico"),
			SecurityDetails: common.SecurityDetails{},
			ServerAddr:      common.RemoteAddress{IPAddress: host, Port: port},
			Size:            map[string]int{"body": 19, "headers": 229},
			Status:          404,
			StatusText:      "Not Found",
			URL:             tb.url("/favicon.ico"),
			Text:            "404 page not found\n",
		},
		{
			AllHeaders: map[string]string{
				"content-length": "28",
				"content-type":   "application/json",
				"date":           "Wed, 29 Jan 2025 09:00:00 GMT",
			},
			Body:                  "{\"message\": \"Hello tester!\"}",
			FrameURL:              tb.url("/home"),
			AcceptLanguageHeader:  "",
			AcceptLanguageHeaders: []string{""},
			Headers: map[string]string{
				"Content-Length": "28",
				"Content-Type":   "application/json",
				"Date":           "Wed, 29 Jan 2025 09:00:00 GMT",
			},
			HeadersArray: []map[string]string{
				{"name": "Date", "value": "Wed, 29 Jan 2025 09:00:00 GMT"},
				{"name": "Content-Type", "value": "application/json"},
				{"name": "Content-Length", "value": "28"},
			},
			JSON:            "{\"message\":\"Hello tester!\"}",
			OK:              true,
			RequestURL:      tb.url("/api"),
			SecurityDetails: common.SecurityDetails{},
			ServerAddr:      common.RemoteAddress{IPAddress: host, Port: port},
			Size:            map[string]int{"body": 28, "headers": 108},
			Status:          200,
			StatusText:      "OK",
			URL:             tb.url("/api"),
			Text:            "{\"message\": \"Hello tester!\"}",
		},
	}

	// Compare each response one by one for better test failure visibility
	for _, resp := range responses {
		i := slices.IndexFunc(expected, func(r response) bool { return resp.URL == r.URL })
		assert.NotEqual(t, -1, i, "failed to find expected request with URL %s", resp.URL)

		sortByName := func(m1, m2 map[string]string) int {
			return strings.Compare(m1["name"], m2["name"])
		}
		slices.SortFunc(resp.HeadersArray, sortByName)
		slices.SortFunc(expected[i].HeadersArray, sortByName)
		assert.Equal(t, expected[i], resp)
	}
}

// TestPageOnRequestFinished tests that the requestfinished event fires when requests complete successfully.
func TestPageOnRequestFinished(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())
	tb.withHandler("/home", func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <link rel="stylesheet" href="/style.css">
</head>
<body>
    <script>fetch('/api', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({name: 'tester'})
    })</script>
</body>
</html>`)
		require.NoError(t, err)
	})
	tb.withHandler("/api", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		defer require.NoError(t, r.Body.Close())

		var data struct {
			Name string `json:"name"`
		}
		err = json.Unmarshal(body, &data)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		_, err = fmt.Fprintf(w, `{"message": "Hello %s!"}`, data.Name)
		require.NoError(t, err)
	})
	tb.withHandler("/style.css", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		_, err := fmt.Fprintf(w, `body { background-color: #f0f0f0; }`)
		require.NoError(t, err)
	})

	tb.vu.ActivateVU()
	tb.vu.StartIteration(t)
	defer tb.vu.EndIteration(t)

	gv, err := tb.vu.RunAsync(t, `
		const context = await browser.newContext();
		const page = await context.newPage();

		var finishedRequests = [];
		page.on('requestfinished', (request) => {
			finishedRequests.push({
				url: request.url(),
				method: request.method(),
				resourceType: request.resourceType(),
				isNavigationRequest: request.isNavigationRequest(),
			});
		});

		await page.goto('%s', {waitUntil: 'networkidle'});
		await page.close();
		return JSON.stringify(finishedRequests, null, 2);
	`, tb.url("/home"))
	require.NoError(t, err)

	got := k6test.ToPromise(t, gv)
	require.Equal(t, sobek.PromiseStateFulfilled, got.State())

	var finishedRequests []struct {
		URL                 string `json:"url"`
		Method              string `json:"method"`
		ResourceType        string `json:"resourceType"`
		IsNavigationRequest bool   `json:"isNavigationRequest"`
	}
	err = json.Unmarshal([]byte(got.Result().String()), &finishedRequests)
	require.NoError(t, err)

	// Verify we captured some finished requests
	require.NotEmpty(t, finishedRequests, "expected to capture at least one finished request")

	var foundHome, foundAPI, foundCSS bool
	for _, req := range finishedRequests {
		switch {
		case strings.HasSuffix(req.URL, "/home"):
			foundHome = true
			assert.Equal(t, "GET", req.Method)
			assert.Equal(t, "Document", req.ResourceType)
			assert.True(t, req.IsNavigationRequest)
		case strings.HasSuffix(req.URL, "/api"):
			foundAPI = true
			assert.Equal(t, "POST", req.Method)
			assert.Equal(t, "Fetch", req.ResourceType)
			assert.False(t, req.IsNavigationRequest)
		case strings.HasSuffix(req.URL, "/style.css"):
			foundCSS = true
			assert.Equal(t, "GET", req.Method)
			assert.Equal(t, "Stylesheet", req.ResourceType)
			assert.False(t, req.IsNavigationRequest)
		}
	}

	assert.True(t, foundHome, "expected to find /home request in finished requests")
	assert.True(t, foundAPI, "expected to find /api request in finished requests")
	assert.True(t, foundCSS, "expected to find /style.css request in finished requests")
}

// TestPageOnRequestFinishedRedirect tests that the requestfinished event fires
// for each request in a redirect chain, not just the final one.
func TestPageOnRequestFinishedRedirect(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())
	tb.withHandler("/redir-a", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, tb.url("/redir-b"), http.StatusFound)
	})
	tb.withHandler("/redir-b", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, tb.url("/redir-final"), http.StatusFound)
	})
	tb.withHandler("/redir-final", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, err := fmt.Fprint(w, "<html><body>done</body></html>")
		require.NoError(t, err)
	})

	tb.vu.ActivateVU()
	tb.vu.StartIteration(t)
	defer tb.vu.EndIteration(t)

	gv, err := tb.vu.RunAsync(t, `
		const context = await browser.newContext();
		const page = await context.newPage();

		const expectedCount = 3; // redir-a, redir-b, redir-final
		const finishedRequests = [];
		let resolveAll;
		const allFinished = new Promise(r => { resolveAll = r; });

		page.on('requestfinished', (request) => {
			const url = request.url();
			if (url.includes('/redir-')) {
				finishedRequests.push({ url: url });
				if (finishedRequests.length >= expectedCount) {
					resolveAll();
				}
			}
		});

		await page.goto('%s', {waitUntil: 'networkidle'});
		await allFinished;
		await page.close();
		return JSON.stringify(finishedRequests, null, 2);
	`, tb.url("/redir-a"))
	require.NoError(t, err)

	got := k6test.ToPromise(t, gv)
	require.Equal(t, sobek.PromiseStateFulfilled, got.State())

	var finishedRequests []struct {
		URL string `json:"url"`
	}
	err = json.Unmarshal([]byte(got.Result().String()), &finishedRequests)
	require.NoError(t, err)

	// Verify that requestfinished fired for the redirect requests,
	// not just the final response.
	var foundRedirA, foundRedirB, foundFinal bool
	for _, req := range finishedRequests {
		switch {
		case strings.HasSuffix(req.URL, "/redir-a"):
			foundRedirA = true
		case strings.HasSuffix(req.URL, "/redir-b"):
			foundRedirB = true
		case strings.HasSuffix(req.URL, "/redir-final"):
			foundFinal = true
		}
	}

	assert.True(t, foundRedirA, "expected requestfinished to fire for /redir-a (first redirect)")
	assert.True(t, foundRedirB, "expected requestfinished to fire for /redir-b (second redirect)")
	assert.True(t, foundFinal, "expected requestfinished to fire for /redir-final (final response)")
}

// TestPageOnResponseRedirect tests that the response event fires
// for each response in a redirect chain, not just the final one.
func TestPageOnResponseRedirect(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())
	tb.withHandler("/redir-a", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, tb.url("/redir-b"), http.StatusFound)
	})
	tb.withHandler("/redir-b", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, tb.url("/redir-final"), http.StatusFound)
	})
	tb.withHandler("/redir-final", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, err := fmt.Fprint(w, "<html><body>done</body></html>")
		require.NoError(t, err)
	})

	tb.vu.ActivateVU()
	tb.vu.StartIteration(t)
	defer tb.vu.EndIteration(t)

	gv, err := tb.vu.RunAsync(t, `
		const context = await browser.newContext();
		const page = await context.newPage();

		const expectedCount = 3; // redir-a, redir-b, redir-final
		const responses = [];
		let resolveAll;
		const allReceived = new Promise(r => { resolveAll = r; });

		page.on('response', (response) => {
			const url = response.url();
			if (url.includes('/redir-')) {
				responses.push({ url: url, status: response.status() });
				if (responses.length >= expectedCount) {
					resolveAll();
				}
			}
		});

		await page.goto('%s', {waitUntil: 'networkidle'});
		await allReceived;
		await page.close();
		return JSON.stringify(responses, null, 2);
	`, tb.url("/redir-a"))
	require.NoError(t, err)

	got := k6test.ToPromise(t, gv)
	require.Equal(t, sobek.PromiseStateFulfilled, got.State())

	var responses []struct {
		URL    string `json:"url"`
		Status int    `json:"status"`
	}
	err = json.Unmarshal([]byte(got.Result().String()), &responses)
	require.NoError(t, err)

	// Verify that response event fired for each redirect response,
	// not just the final one.
	var foundRedirA, foundRedirB, foundFinal bool
	for _, resp := range responses {
		switch {
		case strings.HasSuffix(resp.URL, "/redir-a"):
			foundRedirA = true
			assert.Equal(t, http.StatusFound, resp.Status, "/redir-a should be a 302")
		case strings.HasSuffix(resp.URL, "/redir-b"):
			foundRedirB = true
			assert.Equal(t, http.StatusFound, resp.Status, "/redir-b should be a 302")
		case strings.HasSuffix(resp.URL, "/redir-final"):
			foundFinal = true
			assert.Equal(t, http.StatusOK, resp.Status, "/redir-final should be a 200")
		}
	}

	assert.True(t, foundRedirA, "expected response event to fire for /redir-a (first redirect)")
	assert.True(t, foundRedirB, "expected response event to fire for /redir-b (second redirect)")
	assert.True(t, foundFinal, "expected response event to fire for /redir-final (final response)")
}

// TestPageOnRequestFailed tests that the requestfailed event fires when requests fail.
func TestPageOnRequestFailed(t *testing.T) {
	t.Parallel()

	t.Run("server_aborted_request", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withHTTPServer())

		tb.withHandler("/home", func(w http.ResponseWriter, _ *http.Request) {
			_, err := fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<body>
    <h1>Test Page</h1>
    <script>
        fetch('/api/data')
            .then(() => { window.fetchResult = 'success'; })
            .catch(() => { window.fetchResult = 'failed'; });
    </script>
</body>
</html>`)
			require.NoError(t, err)
		})

		tb.withHandler("/api/data", func(w http.ResponseWriter, _ *http.Request) {
			panic(http.ErrAbortHandler)
		})

		p := tb.NewPage(nil)

		var failedRequests []map[string]string
		err := p.On(common.PageEventRequestFailed, func(ev common.PageEvent) error {
			req := ev.Request
			failure := req.Failure()
			errorText := ""
			if failure != nil {
				errorText = failure.ErrorText
			}
			failedRequests = append(failedRequests, map[string]string{
				"url":          req.URL(),
				"method":       req.Method(),
				"resourceType": req.ResourceType(),
				"errorText":    errorText,
			})
			return nil
		})
		require.NoError(t, err)

		opts := &common.FrameGotoOptions{
			WaitUntil: common.LifecycleEventNetworkIdle,
			Timeout:   common.DefaultTimeout,
		}
		_, err = p.Goto(tb.url("/home"), opts)
		require.NoError(t, err)

		require.Len(t, failedRequests, 1, "expected exactly one failed request")

		failedReq := failedRequests[0]
		assert.Contains(t, failedReq["url"], "/api/data", "failed request URL should contain /api/data")
		assert.Equal(t, "GET", failedReq["method"], "failed request method should be GET")
		assert.Equal(t, "Fetch", failedReq["resourceType"], "failed request resourceType should be Fetch")
		assert.NotEmpty(t, failedReq["errorText"], "failed request should have error text")
	})

	t.Run("server_aborted_multiple_requests", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withHTTPServer())

		tb.withHandler("/home", func(w http.ResponseWriter, _ *http.Request) {
			_, err := fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<body>
    <script>
        Promise.allSettled([
            fetch('/api/first'),
            fetch('/api/second')
        ]).then(() => { window.allDone = true; });
    </script>
</body>
</html>`)
			require.NoError(t, err)
		})

		tb.withHandler("/api/first", func(w http.ResponseWriter, _ *http.Request) {
			panic(http.ErrAbortHandler)
		})

		tb.withHandler("/api/second", func(w http.ResponseWriter, _ *http.Request) {
			panic(http.ErrAbortHandler)
		})

		p := tb.NewPage(nil)

		var failedRequests []string
		err := p.On(common.PageEventRequestFailed, func(ev common.PageEvent) error {
			failedRequests = append(failedRequests, ev.Request.URL())
			return nil
		})
		require.NoError(t, err)

		opts := &common.FrameGotoOptions{
			WaitUntil: common.LifecycleEventNetworkIdle,
			Timeout:   common.DefaultTimeout,
		}
		_, err = p.Goto(tb.url("/home"), opts)
		require.NoError(t, err)

		// Verify that both requests failed
		require.Len(t, failedRequests, 2, "expected two failed requests")

		var hasFirst, hasSecond bool
		for _, url := range failedRequests {
			if strings.Contains(url, "/api/first") {
				hasFirst = true
			}
			if strings.Contains(url, "/api/second") {
				hasSecond = true
			}
		}
		assert.True(t, hasFirst, "expected /api/first to be in failed requests")
		assert.True(t, hasSecond, "expected /api/second to be in failed requests")
	})
}

func TestPageMustUseNativeJavaScriptObjects(t *testing.T) {
	t.Parallel()

	// Add an element to query for later.
	tb := newTestBrowser(t)
	page := tb.NewPage(nil)
	require.NoError(t, page.SetContent(`
		<!DOCTYPE html>
		<html>
		<head></head>
		<body>
			<div id='textField'>Hello World</div>
		</div>
		</body>
		</html>
	`, nil))

	// Override the native objects using the test page.
	//
	// WARNING: Keep the function names and the native
	// type names in sync for isOverwritten() to work.
	// E.g.: Set() and window.overrides.Set are the same.
	_, err := page.Evaluate(`() => {
		window.overrides = {};
		Set = () => window.overrides.Set = true;
		Map = () => window.overrides.Map = true;
		// Add other native objects here as needed.
	}`)
	require.NoError(t, err)

	// Ensure that our test page has overridden the
	// native Set and Map JavaScript objects.
	isOverwritten := func(page *common.Page, objectName string) bool {
		v, err := page.Evaluate(fmt.Sprintf(`
			() => { %s(); return window.overrides.%[1]s; }`,
			objectName,
		), nil)
		require.NoErrorf(t, err, "page should not have thrown an error: %s", err)
		require.IsTypef(t, v, true, "expected %s to be a boolean", objectName)
		return v.(bool)
	}
	require.True(t, isOverwritten(page, "Set"), "page should override the native Set")
	require.True(t, isOverwritten(page, "Map"), "page should override the native Map")

	// Ensure that we can still use the native Set and
	// Map, even if the page under test has overridden
	// them. QueryAll calls injected script, which
	// requires Set and Map.
	_, err = page.QueryAll("#textField")
	require.NoErrorf(t, err, "page should not override the native objects, but it did")
}

func TestWaitForNavigationWithURL(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipped due to https://github.com/grafana/k6/issues/4937")
	}

	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	tb.vu.ActivateVU()
	tb.vu.StartIteration(t)

	// Setup
	tb.vu.SetVar(t, "page", &sobek.Object{})
	tb.vu.SetVar(t, "testURL", tb.staticURL("waitfornavigation_test.html"))
	tb.vu.SetVar(t, "page1URL", tb.staticURL("page1.html"))
	_, err := tb.vu.RunAsync(t, `
			page = await browser.newPage();
		`)
	require.NoError(t, err)

	// Test exact URL match
	got := tb.vu.RunPromise(t, `
		await page.goto(testURL);

		await Promise.all([
			page.waitForNavigation({ url: page1URL }),
			page.locator('#page1').click()
		]);
		return page.url();
	`,
	)
	assert.Equal(t, tb.staticURL("page1.html"), got.Result().String())

	// Test regex pattern - matches any page with .html extension
	got = tb.vu.RunPromise(t, `
		await page.goto(testURL);

		await Promise.all([
			page.waitForNavigation({ url: /.*2\.html$/ }),
			page.locator('#page2').click()
		]);
		return page.url();
	`,
	)
	assert.Equal(t, tb.staticURL("page2.html"), got.Result().String())

	// Test timeout when URL doesn't match
	_, err = tb.vu.RunAsync(t, `
		await page.goto(testURL);

		await Promise.all([
			page.waitForNavigation({ url: /.*nonexistent.html$/, timeout: 500 }),
			page.locator('#page1').click()  // This goes to page1.html, not nonexistent.html
		]);
	`,
	)
	assert.ErrorContains(t, err, "timed out after 500ms")

	// Test empty pattern (matches any navigation)
	got = tb.vu.RunPromise(t, `
		await page.goto(testURL);

		await Promise.all([
			page.waitForNavigation({ url: '' }),
			page.locator('#page2').click()
		]);
		return page.url();
	`,
	)
	assert.Equal(t, tb.staticURL("page2.html"), got.Result().String())

	// Test regex pattern with invalid regex
	_, err = tb.vu.RunAsync(t, `
		await page.goto(testURL);

		await Promise.all([
			page.waitForNavigation({ url: /^.*/my_messages.*$/ }),
			page.locator('#page2').click()
		]);
	`,
	)
	assert.ErrorContains(t, err, "Unexpected token *")
}

func TestPageWaitForURLSuccess(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Skipped due to https://github.com/grafana/k6/issues/4937")
	}

	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name:     "when_already_at_matching_url",
			code:     `await page.waitForURL(/.*waitfornavigation_test\.html$/);`,
			expected: []string{"waitfornavigation_test.html"},
		},
		{
			name: "exact_url_match",
			code: `
				await Promise.all([
					page.waitForURL(page1URL),
					page.locator('#page1').click()
				]);
			`,
			expected: []string{"page1.html"},
		},
		{
			name: "regex_pattern_match",
			code: `
				await Promise.all([
					page.waitForURL(/.*2\.html$/),
					page.locator('#page2').click()
				]);
			`,
			expected: []string{"page2.html"},
		},
		{
			name: "empty_pattern_match",
			code: `
				await Promise.all([
					page.waitForURL(''),
					page.locator('#page2').click()
				]);
			`,
			expected: []string{"page2.html", "waitfornavigation_test.html"},
		},
		{
			name: "waitUntil_domcontentloaded",
			code: `
				await Promise.all([
					page.waitForURL(/.*page1\.html$/, { waitUntil: 'domcontentloaded' }),
					page.locator('#page1').click()
				]);
			`,
			expected: []string{"page1.html"},
		},
		{
			name: "already_at_url_with_regex_pattern",
			code: `
				await page.waitForURL(/.*\/waitfornavigation_test\.html$/);
			`,
			expected: []string{"waitfornavigation_test.html"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			tb.vu.ActivateVU()
			tb.vu.StartIteration(t)

			tb.vu.SetVar(t, "page", &sobek.Object{})
			tb.vu.SetVar(t, "testURL", tb.staticURL("waitfornavigation_test.html"))
			tb.vu.SetVar(t, "page1URL", tb.staticURL("page1.html"))
			_, err := tb.vu.RunAsync(t, `
				page = await browser.newPage();
			`)
			require.NoError(t, err)

			result := tb.vu.RunPromise(t, `
				await page.goto(testURL);
				%s
				return page.url();
			`, tt.code)
			got := strings.ReplaceAll(result.Result().String(), tb.staticURL(""), "")
			assert.Contains(t, tt.expected, got)
		})
	}
}

func TestPageWaitForURLFailure(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Skipped due to https://github.com/grafana/k6/issues/4937")
	}

	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name: "timeout_on_mismatched_url",
			code: `
				await Promise.all([
					page.waitForURL(/.*nonexistent\.html$/, { timeout: 500 }),
					page.locator('#page1').click()  // This goes to page1.html, not nonexistent.html
				]);
			`,
			expected: "timed out after 500ms",
		},
		{
			name: "missing_required_argument",
			code: `
				await page.waitForURL();
			`,
			expected: "missing required argument 'url'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			tb.vu.ActivateVU()
			tb.vu.StartIteration(t)

			tb.vu.SetVar(t, "page", &sobek.Object{})
			tb.vu.SetVar(t, "testURL", tb.staticURL("waitfornavigation_test.html"))
			_, err := tb.vu.RunAsync(t, `
				page = await browser.newPage();
			`)
			require.NoError(t, err)

			_, err = tb.vu.RunAsync(t, `
				await page.goto(testURL);
				%s
			`, tt.code)
			assert.ErrorContains(t, err, tt.expected)
		})
	}
}

func TestPageWaitForResponse(t *testing.T) {
	t.Parallel()

	t.Run("ok/correct_response_matches", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withHTTPServer())
		tb.withHandler("/api/users", func(w http.ResponseWriter, r *http.Request) {
			_, err := fmt.Fprintf(w, `{"users": []}`)
			require.NoError(t, err)
		})
		tb.withHandler("/api/posts", func(w http.ResponseWriter, r *http.Request) {
			_, err := fmt.Fprintf(w, `{"posts": []}`)
			require.NoError(t, err)
		})
		tb.withHandler("/page", func(w http.ResponseWriter, _ *http.Request) {
			_, err := fmt.Fprintf(w, `
				<!doctype html>
				<html><body><script>
					setTimeout(() => fetch('/api/posts'), 50);
					setTimeout(() => fetch('/api/users'), 100);
				</script></body></html>
			`)
			require.NoError(t, err)
		})

		p := tb.NewPage(nil)

		gotoPage := func() error {
			_, err := p.Goto(tb.url("/page"), &common.FrameGotoOptions{
				WaitUntil: common.LifecycleEventDOMContentLoad,
				Timeout:   common.DefaultTimeout,
			})
			return err
		}

		waitForUsers := func() error {
			opts := common.NewPageWaitForResponseOptions(p.Timeout())
			mockRegexChecker := func(pattern, url string) (bool, error) {
				return strings.Contains(url, "/users"), nil
			}
			resp, err := p.WaitForResponse(".*users.*", opts, mockRegexChecker)
			if err != nil {
				return err
			}
			require.Contains(t, resp.URL(), "/users")
			return nil
		}

		err := tb.run(tb.context(), gotoPage, waitForUsers)
		require.NoError(t, err)
	})

	t.Run("err/canceled", func(t *testing.T) {
		t.Parallel()
		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		opts := common.NewPageWaitForResponseOptions(p.Timeout())
		go tb.cancelContext()
		<-tb.context().Done()

		mockRegexChecker := func(pattern, url string) (bool, error) {
			return strings.Contains(url, "page"), nil
		}

		_, err := p.WaitForResponse("/page", opts, mockRegexChecker)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("err/timeout", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withHTTPServer())

		p := tb.NewPage(nil)

		waitForApi := func() error {
			opts := common.NewPageWaitForResponseOptions(500 * time.Millisecond)
			mockRegexChecker := func(pattern, url string) (bool, error) {
				return strings.Contains(url, "/api"), nil
			}

			resp, err := p.WaitForResponse(".*api.*", opts, mockRegexChecker)
			if err != nil {
				return err
			}
			require.NotNil(t, resp)
			require.Contains(t, resp.URL(), "/api")
			return nil
		}

		err := tb.run(tb.context(), waitForApi)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}

func TestPageWaitForRequest(t *testing.T) {
	t.Parallel()

	t.Run("ok/waits_for_request", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())
		p := tb.GotoNewPage(tb.staticURL("/usual.html"))

		var req *common.Request
		err := tb.run(tb.context(),
			// We make three requests for detecting edge cases,
			// such as if WaitForRequest stops at the first match,
			// or misses requests made in quick succession.
			func() error {
				_, err := p.Evaluate(`() => {
					fetch('fetch-request-1');
					fetch('fetch-request-2');
					fetch('fetch-request-3');
				}`, nil)
				return err
			},
			// Waits until the request we're looking for is made.
			func() error {
				var werr error
				req, werr = p.WaitForRequest(
					"fetch-request-2",
					&common.PageWaitForRequestOptions{
						Timeout: p.Timeout(),
					},
					func(pattern, url string) (bool, error) {
						return strings.Contains(url, pattern), nil
					},
				)
				return werr
			},
		)
		require.NoError(t, err)
		require.NotNilf(t, req, "must have returned the request")
		require.Contains(t, req.URL(), "fetch-request-2", "must return the correct request")
	})

	t.Run("err/pattern-func-error", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer(), withLogCache())
		p := tb.GotoNewPage(tb.staticURL("/usual.html"))

		patternFuncError := errors.New("pattern func error")
		err := tb.run(tb.context(), func() error {
			_, werr := p.WaitForRequest(
				"usual.html",
				&common.PageWaitForRequestOptions{
					Timeout: p.Timeout(),
				},
				func(pattern, url string) (bool, error) {
					return false, patternFuncError
				},
			)
			return werr
		})
		require.ErrorIs(t, err, patternFuncError)
		tb.logCache.assertContains(t, patternFuncError.Error())
	})

	t.Run("err/canceled", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)

		go tb.cancelContext()
		<-tb.context().Done()

		req, err := p.WaitForRequest(
			"/does-not-matter",
			&common.PageWaitForRequestOptions{Timeout: p.Timeout()},
			func(pattern, url string) (bool, error) {
				return true, nil
			},
		)
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, req)
	})

	t.Run("err/timeout", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)

		var req *common.Request
		err := tb.run(tb.context(), func() error {
			var werr error
			req, werr = tb.NewPage(nil).WaitForRequest(
				"/does-not-exist",
				&common.PageWaitForRequestOptions{
					Timeout: 500 * time.Millisecond,
				},
				func(pattern, url string) (bool, error) {
					return true, nil
				},
			)
			return werr
		})
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Nil(t, req)
	})
}

// TestClickInNestedFramesCORS tests clicking on buttons within nested frames
// which are from different origins. At the end of the test the counter in
// each frame should be "1".
func TestClickInNestedFramesCORS(t *testing.T) {
	t.Parallel()

	// Origin C: innermost frame with counter button and nested same-origin iframe
	originCHTML := `<!DOCTYPE html>
	<html>
	<head></head>
	<body>
	  <p>Counter: <span id="count">0</span></p>
	  <button id="increment">Increment Counter</button>
	  <iframe id="frameD" src="/innerC" width="300" height="100"></iframe>
	  <script>
		let count = 0;
		document.getElementById('increment').addEventListener('click', () => {
		  count++;
		  document.getElementById('count').textContent = count;
		});
	  </script>
	</body>
	</html>`

	// Nested same-origin frame content served at /innerC on origin C
	innerCHTML := `<!DOCTYPE html>
	<html>
	<head></head>
	<body>
	  <p>Counter D: <span id="countD">0</span></p>
	  <button id="incrementD">Increment Counter D</button>
	  <script>
		let countD = 0;
		document.getElementById('incrementD').addEventListener('click', () => {
		  countD++;
		  document.getElementById('countD').textContent = countD;
		});
	  </script>
	</body>
	</html>`

	// Nested same-origin frame content served at /innerA on origin A
	innerAHTML := `<!DOCTYPE html>
	<html>
	<head></head>
	<body>
	  <p>Counter A2: <span id="countA2">0</span></p>
	  <button id="incrementA2">Increment Counter A2</button>
	  <script>
		let countA2 = 0;
		document.getElementById('incrementA2').addEventListener('click', () => {
		  countA2++;
		  document.getElementById('countA2').textContent = countA2;
		});
	  </script>
	</body>
	</html>`

	// Server for origin C
	muxC := http.NewServeMux()
	muxC.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := w.Write([]byte(originCHTML))
		require.NoError(t, err)
	})
	muxC.HandleFunc("/innerC", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := w.Write([]byte(innerCHTML))
		require.NoError(t, err)
	})
	srvC := httptest.NewServer(muxC)
	t.Cleanup(func() {
		srvC.Close()
	})

	// Origin B: intermediate frame embedding origin C + own counter (with dynamic C URL)
	originBHTML := fmt.Sprintf(`<!DOCTYPE html>
	<html>
	<head></head>
	<body>
	  <p>Counter B: <span id="countB">0</span></p>
	  <button id="incrementB">Increment Counter B</button>
	  <iframe id="frameC" src="%s" width="400" height="200"></iframe>
	  <script>
		let countB = 0;
		document.getElementById('incrementB').addEventListener('click', () => {
		  countB++;
		  document.getElementById('countB').textContent = countB;
		});
	  </script>
	</body>
	</html>`, srvC.URL)

	// Server for origin B
	muxB := http.NewServeMux()
	muxB.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := w.Write([]byte(originBHTML))
		require.NoError(t, err)
	})
	srvB := httptest.NewServer(muxB)
	t.Cleanup(func() {
		srvB.Close()
	})

	// Origin A: main page embedding origin B and same-origin frame A (with dynamic B URL)
	originAHTML := fmt.Sprintf(`<!DOCTYPE html>
	<html>
	<head></head>
	<body>
	  <p>Counter A: <span id="countA">0</span></p>
	  <button id="incrementA">Increment Counter A</button>
	  <iframe id="frameA" src="/innerA" width="300" height="150" style="display: block; margin: 10px auto;"></iframe>
	  <iframe id="frameB" src="%s" width="450" height="300" style="display: block; margin: 10px auto;"></iframe>
	  <script>
		let countA = 0;
		document.getElementById('incrementA').addEventListener('click', () => {
		  countA++;
		  document.getElementById('countA').textContent = countA;
		});
	  </script>
	</body>
	</html>`, srvB.URL)

	// Server for origin A
	muxA := http.NewServeMux()
	muxA.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := w.Write([]byte(originAHTML))
		require.NoError(t, err)
	})
	muxA.HandleFunc("/innerA", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := w.Write([]byte(innerAHTML))
		require.NoError(t, err)
	})
	srvA := httptest.NewServer(muxA)
	t.Cleanup(func() {
		srvA.Close()
	})

	t.Run("ok/click_in_nested_frames", func(t *testing.T) {
		t.Parallel()

		// Use srvA.URL as the entry point in the rest of the test (navigate, click, etc.).
		page := newTestBrowser(t).NewPage(nil)

		// Navigate to the page that srvA is serving.
		opts := &common.FrameGotoOptions{
			Timeout: common.DefaultTimeout,
		}
		_, err := page.Goto(srvA.URL, opts)
		require.NoError(t, err)

		var (
			clickOpts     = common.NewFrameClickOptions(page.Timeout())
			expectedCount = "1"
		)

		// First click on the main frame.
		err = page.Locator("#incrementA", nil).Click(clickOpts)
		require.NoError(t, err)

		la := page.Locator("#countA", nil)
		countA, ok, err := la.TextContent(common.NewFrameTextContentOptions(la.Timeout()))
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, expectedCount, countA)

		// Now get the first nested frame.
		frameA, err := page.Query("#frameA")
		require.NoError(t, err)

		frameAContent, err := frameA.ContentFrame()
		require.NoError(t, err)

		// Click on the second nested frame.
		err = frameAContent.Locator("#incrementA2", nil).Click(clickOpts)
		require.NoError(t, err)

		la2 := frameAContent.Locator("#countA2", nil)
		countA2, ok, err := la2.TextContent(common.NewFrameTextContentOptions(la2.Timeout()))
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, expectedCount, countA2)

		// Now get the second nested frame.
		frameB, err := page.Query("#frameB")
		require.NoError(t, err)

		frameBContent, err := frameB.ContentFrame()
		require.NoError(t, err)

		// Click on the third nested frame.
		err = frameBContent.Locator("#incrementB", nil).Click(clickOpts)
		require.NoError(t, err)

		lb := frameBContent.Locator("#countB", nil)
		countB, ok, err := lb.TextContent(common.NewFrameTextContentOptions(lb.Timeout()))
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, expectedCount, countB)

		// Now get the third nested frame.
		frameC, err := frameBContent.Query("#frameC", false)
		require.NoError(t, err)

		frameCContent, err := frameC.ContentFrame()
		require.NoError(t, err)

		// Click on the fourth nested frame.
		err = frameCContent.Locator("#increment", nil).Click(clickOpts)
		require.NoError(t, err)

		lc := frameCContent.Locator("#count", nil)
		count, ok, err := lc.TextContent(common.NewFrameTextContentOptions(lc.Timeout()))
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, expectedCount, count)

		// Now get the fourth nested frame.
		frameD, err := frameCContent.Query("#frameD", false)
		require.NoError(t, err)

		frameDContent, err := frameD.ContentFrame()
		require.NoError(t, err)

		// Click on the fifth nested frame.
		err = frameDContent.Locator("#incrementD", nil).Click(clickOpts)
		require.NoError(t, err)

		ld := frameDContent.Locator("#countD", nil)
		countD, ok, err := ld.TextContent(common.NewFrameTextContentOptions(ld.Timeout()))
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, expectedCount, countD)
	})

	// This test is the same as the previous one, but uses the Locator API
	// instead of the Frame APIs.
	t.Run("ok/click_in_nested_frames_with_locator", func(t *testing.T) {
		t.Parallel()

		// Use srvA.URL as the entry point in the rest of the test (navigate, click, etc.).
		page := newTestBrowser(t).NewPage(nil)

		// Navigate to the page that srvA is serving.
		opts := &common.FrameGotoOptions{
			Timeout: common.DefaultTimeout,
		}
		_, err := page.Goto(srvA.URL, opts)
		require.NoError(t, err)

		var (
			clickOpts     = common.NewFrameClickOptions(page.Timeout())
			expectedCount = "1"
		)

		// First click on the main frame.
		err = page.Locator("#incrementA", nil).Click(clickOpts)
		require.NoError(t, err)

		la := page.Locator("#countA", nil)
		countA, ok, err := la.TextContent(common.NewFrameTextContentOptions(la.Timeout()))
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, expectedCount, countA)

		// Now get the first nested frame.
		frameAContent := page.Locator("#frameA", nil).ContentFrame()

		// Click on the second nested frame.
		err = frameAContent.Locator("#incrementA2", nil).Click(clickOpts)
		require.NoError(t, err)

		la2 := frameAContent.Locator("#countA2", nil)
		countA2, ok, err := la2.TextContent(common.NewFrameTextContentOptions(la2.Timeout()))
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, expectedCount, countA2)

		// Now get the second nested frame.
		frameBContent := page.Locator("#frameB", nil).ContentFrame()

		// Click on the third nested frame.
		err = frameBContent.Locator("#incrementB", nil).Click(clickOpts)
		require.NoError(t, err)

		lb := frameBContent.Locator("#countB", nil)
		countB, ok, err := lb.TextContent(common.NewFrameTextContentOptions(lb.Timeout()))
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, expectedCount, countB)

		// Now get the third nested frame.
		frameCContent := frameBContent.Locator("#frameC", nil).ContentFrame()

		// Click on the fourth nested frame.
		err = frameCContent.Locator("#increment", nil).Click(clickOpts)
		require.NoError(t, err)

		lc := frameCContent.Locator("#count", nil)
		count, ok, err := lc.TextContent(common.NewFrameTextContentOptions(lc.Timeout()))
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, expectedCount, count)

		// Now get the fourth nested frame.
		frameDContent := frameCContent.Locator("#frameD", nil).ContentFrame()

		// Click on the fifth nested frame.
		err = frameDContent.Locator("#incrementD", nil).Click(clickOpts)
		require.NoError(t, err)

		ld := frameDContent.Locator("#countD", nil)
		countD, ok, err := ld.TextContent(common.NewFrameTextContentOptions(ld.Timeout()))
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, expectedCount, countD)
	})
}

func TestPageUnroute(t *testing.T) {
	t.Parallel()

	jsRegexCheckerMock := func(pattern, url string) (bool, error) {
		matched, err := regexp.MatchString(fmt.Sprintf("http://[^/]*%s", pattern), url)
		if err != nil {
			return false, fmt.Errorf("error matching regex: %w", err)
		}
		return matched, nil
	}

	t.Run("unroute_single_route", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withHTTPServer())
		p := tb.NewPage(nil)

		routeHandlerCalls := 0
		routeHandler := func(route *common.Route) error {
			routeHandlerCalls++
			return route.Continue(common.ContinueOptions{})
		}

		tb.withHandler("/test", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, err := fmt.Fprintf(w, `
			<html>
				<body>
					<script>
						fetch('/api/data');
					</script>
				</body>
			</html>
			`)
			require.NoError(t, err)
		})

		tb.withHandler("/api/data", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := fmt.Fprint(w, `{"data": "test"}`)
			require.NoError(t, err)
		})

		// Add route
		err := p.Route("/api/data", routeHandler, jsRegexCheckerMock)
		require.NoError(t, err)

		opts := &common.FrameGotoOptions{
			WaitUntil: common.LifecycleEventNetworkIdle,
			Timeout:   common.DefaultTimeout,
		}
		_, err = p.Goto(tb.url("/test"), opts)
		require.NoError(t, err)

		assert.Equal(t, 1, routeHandlerCalls)

		// Remove the route
		routeHandlerCalls = 0
		err = p.Unroute("/api/data")
		require.NoError(t, err)

		_, err = p.Goto(tb.url("/test"), opts)
		require.NoError(t, err)

		assert.Equal(t, 0, routeHandlerCalls, "Route handler should not be called after unroute")
	})

	t.Run("unroute_multiple_matching_routes", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withHTTPServer())
		p := tb.NewPage(nil)

		handler1Calls := 0
		handler2Calls := 0

		routeHandler1 := func(route *common.Route) error {
			handler1Calls++
			return route.Continue(common.ContinueOptions{})
		}

		routeHandler2 := func(route *common.Route) error {
			handler2Calls++
			return route.Continue(common.ContinueOptions{})
		}

		tb.withHandler("/test", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, err := fmt.Fprintf(w, `
			<html>
				<body>
					<script>
						fetch('/api/data');
					</script>
				</body>
			</html>
			`)
			require.NoError(t, err)
		})

		tb.withHandler("/api/data", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := fmt.Fprint(w, `{"data": "test"}`)
			require.NoError(t, err)
		})

		// Add multiple routes for the same path
		err := p.Route("/api/data", routeHandler1, jsRegexCheckerMock)
		require.NoError(t, err)
		err = p.Route("/api/data", routeHandler2, jsRegexCheckerMock)
		require.NoError(t, err)

		opts := &common.FrameGotoOptions{
			WaitUntil: common.LifecycleEventNetworkIdle,
			Timeout:   common.DefaultTimeout,
		}
		_, err = p.Goto(tb.url("/test"), opts)
		require.NoError(t, err)

		// Only the most recently added handler should be called
		assert.Equal(t, 0, handler1Calls, "First handler should not be called when second handler is present")
		assert.Equal(t, 1, handler2Calls, "Second handler should be called")

		// Remove all routes for this path
		handler1Calls = 0
		handler2Calls = 0

		err = p.Unroute("/api/data")
		require.NoError(t, err)

		// Second navigation should not trigger any route handlers
		_, err = p.Goto(tb.url("/test"), opts)
		require.NoError(t, err)

		assert.Equal(t, 0, handler1Calls, "First handler should not be called after unroute")
		assert.Equal(t, 0, handler2Calls, "Second handler should not be called after unroute")
	})

	t.Run("unroute_nonexistent_route", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withHTTPServer())
		p := tb.NewPage(nil)

		routeHandlerCalls := 0
		routeHandler := func(route *common.Route) error {
			routeHandlerCalls++
			return route.Continue(common.ContinueOptions{})
		}

		tb.withHandler("/test", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, err := fmt.Fprintf(w, `
			<html>
				<body>
					<script>
						fetch('/api/data');
					</script>
				</body>
			</html>
			`)
			require.NoError(t, err)
		})

		tb.withHandler("/api/data", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := fmt.Fprint(w, `{"data": "data"}`)
			require.NoError(t, err)
		})

		err := p.Route("/api/data", routeHandler, jsRegexCheckerMock)
		require.NoError(t, err)

		opts := &common.FrameGotoOptions{
			WaitUntil: common.LifecycleEventNetworkIdle,
			Timeout:   common.DefaultTimeout,
		}
		_, err = p.Goto(tb.url("/test"), opts)
		require.NoError(t, err)

		assert.Equal(t, 1, routeHandlerCalls)

		// Remove a non-existent route - this should be a no-op and not affect existing route
		routeHandlerCalls = 0
		err = p.Unroute("/unknown")
		require.NoError(t, err)

		_, err = p.Goto(tb.url("/test"), opts)
		require.NoError(t, err)

		assert.Equal(t, 1, routeHandlerCalls, "Route handler should still be active")
	})
}

func TestPageUnrouteAll(t *testing.T) {
	t.Parallel()

	jsRegexCheckerMock := func(pattern, url string) (bool, error) {
		matched, err := regexp.MatchString(fmt.Sprintf("http://[^/]*%s", pattern), url)
		if err != nil {
			return false, fmt.Errorf("error matching regex: %w", err)
		}
		return matched, nil
	}

	tb := newTestBrowser(t, withHTTPServer())
	p := tb.NewPage(nil)

	route1Calls := 0
	route2Calls := 0

	routeHandler1 := func(route *common.Route) error {
		route1Calls++
		return route.Continue(common.ContinueOptions{})
	}

	routeHandler2 := func(route *common.Route) error {
		route2Calls++
		return route.Continue(common.ContinueOptions{})
	}

	tb.withHandler("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, err := fmt.Fprintf(w, `
			<html>
				<body>
					<script>
						fetch('/api/first');
						fetch('/api/second');
					</script>
				</body>
			</html>
			`)
		require.NoError(t, err)
	})

	tb.withHandler("/api/first", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := fmt.Fprint(w, `{"data": "first"}`)
		require.NoError(t, err)
	})

	tb.withHandler("/api/second", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := fmt.Fprint(w, `{"data": "second"}`)
		require.NoError(t, err)
	})

	// Add multiple routes
	err := p.Route("/api/first", routeHandler1, jsRegexCheckerMock)
	require.NoError(t, err)
	err = p.Route("/api/second", routeHandler2, jsRegexCheckerMock)
	require.NoError(t, err)

	opts := &common.FrameGotoOptions{
		WaitUntil: common.LifecycleEventNetworkIdle,
		Timeout:   common.DefaultTimeout,
	}
	_, err = p.Goto(tb.url("/test"), opts)
	require.NoError(t, err)

	assert.Equal(t, 1, route1Calls)
	assert.Equal(t, 1, route2Calls)

	// Remove all routes - no route handler should be triggered
	route1Calls = 0
	route2Calls = 0
	err = p.UnrouteAll()
	require.NoError(t, err)

	_, err = p.Goto(tb.url("/test"), opts)
	require.NoError(t, err)

	assert.Equal(t, 0, route1Calls, "First route should be removed")
	assert.Equal(t, 0, route2Calls, "Second route should be removed")
}

func TestPageWaitForEvent(t *testing.T) {
	t.Parallel()

	t.Run("ok/waits_for_console_event", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withHTTPServer())
		tb.withHandler("/page", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprintf(w, `
				<!doctype html>
				<html><body><script>
					setTimeout(() => console.log("hello world"), 50);
				</script></body></html>
			`)
		})

		p := tb.NewPage(nil)

		gotoPage := func() error {
			_, err := p.Goto(tb.url("/page"), &common.FrameGotoOptions{
				WaitUntil: common.LifecycleEventDOMContentLoad,
				Timeout:   common.DefaultTimeout,
			})
			return err
		}

		var ev common.PageEvent
		waitForConsole := func() error {
			var err error
			ev, err = p.WaitForEvent(
				common.PageEventConsole,
				&common.PageWaitForEventOptions{Timeout: p.Timeout()},
				nil,
			)
			return err
		}

		err := tb.run(tb.context(), gotoPage, waitForConsole)
		require.NoError(t, err)
		require.NotNil(t, ev.ConsoleMessage)
		require.Equal(t, "hello world", ev.ConsoleMessage.Text)
	})

	t.Run("ok/waits_for_response_event", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withHTTPServer())
		tb.withHandler("/api/data", func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprintf(w, `{"data": "test"}`)
		})
		tb.withHandler("/page", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprintf(w, `
				<!doctype html>
				<html><body><script>
					setTimeout(() => fetch('/api/data'), 50);
				</script></body></html>
			`)
		})

		p := tb.NewPage(nil)

		gotoPage := func() error {
			_, err := p.Goto(tb.url("/page"), &common.FrameGotoOptions{
				WaitUntil: common.LifecycleEventDOMContentLoad,
				Timeout:   common.DefaultTimeout,
			})
			return err
		}

		var ev common.PageEvent
		waitForResponse := func() error {
			var err error
			ev, err = p.WaitForEvent(
				common.PageEventResponse,
				&common.PageWaitForEventOptions{Timeout: p.Timeout()},
				func(pe common.PageEvent) (bool, error) {
					return strings.Contains(pe.Response.URL(), "/api/data"), nil
				},
			)
			return err
		}

		err := tb.run(tb.context(), gotoPage, waitForResponse)
		require.NoError(t, err)
		require.NotNil(t, ev.Response)
		require.Contains(t, ev.Response.URL(), "/api/data")
	})

	t.Run("err/canceled", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)

		go tb.cancelContext()
		<-tb.context().Done()

		_, err := p.WaitForEvent(
			common.PageEventConsole,
			&common.PageWaitForEventOptions{Timeout: p.Timeout()},
			nil,
		)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("err/timeout", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)

		err := tb.run(tb.context(), func() error {
			_, werr := p.WaitForEvent(
				common.PageEventConsole,
				&common.PageWaitForEventOptions{Timeout: 500 * time.Millisecond},
				nil,
			)
			return werr
		})
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}

func TestPageGoBackForward(t *testing.T) {
	t.Parallel()

	t.Run("go_back_and_forward", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())
		p := tb.NewPage(nil)

		url1 := tb.staticURL("page1.html")
		tb.GotoPageAndAssertURL(p, url1)

		url2 := tb.staticURL("page2.html")
		tb.GotoPageAndAssertURL(p, url2)

		opts := common.NewPageGoBackForwardOptions(common.LifecycleEventLoad, common.DefaultTimeout)
		_, err := p.GoBackForward(-1, opts)
		require.NoError(t, err)
		tb.AssertURL(p, url1, "expected to be back on first page")

		_, err = p.GoBackForward(+1, opts)
		require.NoError(t, err)
		tb.AssertURL(p, url2, "expected to be forward on second page")
	})

	t.Run("go_back_to_about_blank", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())
		p := tb.NewPage(nil)

		url1 := tb.staticURL("page1.html")
		tb.GotoPage(p, url1)

		opts := common.NewPageGoBackForwardOptions(common.LifecycleEventLoad, common.DefaultTimeout)
		_, err := p.GoBackForward(-1, opts)
		require.NoError(t, err)
		tb.AssertURL(p, common.BlankPage, "expected to be back on about:blank")

		resp, err := p.GoBackForward(-1, opts)
		require.NoError(t, err)
		assert.Nil(t, resp, "expected nil response when can't go back")
	})

	t.Run("go_forward_returns_nil_at_boundary", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())
		p := tb.NewPage(nil)

		url1 := tb.staticURL("page1.html")
		tb.GotoPage(p, url1)

		opts := common.NewPageGoBackForwardOptions(common.LifecycleEventLoad, common.DefaultTimeout)
		resp, err := p.GoBackForward(+1, opts)
		require.NoError(t, err)
		assert.Nil(t, resp, "expected nil response when can't go forward")
		tb.AssertURL(p, url1, "URL should not change when can't go forward")
	})

	t.Run("go_back_and_forward_with_iframe_page", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())
		p := tb.NewPage(nil)
		opts := common.NewPageGoBackForwardOptions(common.LifecycleEventLoad, common.DefaultTimeout)

		url1 := tb.staticURL("page1.html")
		url2 := tb.staticURL("page_with_iframe.html")
		url3 := tb.staticURL("page2.html")

		tb.GotoPage(p, url1)
		tb.GotoPage(p, url2)
		tb.GotoPage(p, url3)
		tb.GotoPageAndAssertURL(p, url1)

		_, err := p.GoBackForward(-1, opts)
		require.NoError(t, err)
		tb.AssertURL(p, url3, "first goBack should land on page2")

		_, err = p.GoBackForward(-1, opts)
		require.NoError(t, err)
		tb.AssertURL(p, url2, "second goBack should land on iframe page")

		_, err = p.GoBackForward(-1, opts)
		require.NoError(t, err)
		tb.AssertURL(p, url1, "third goBack should land on page1")

		_, err = p.GoBackForward(+1, opts)
		require.NoError(t, err)
		tb.AssertURL(p, url2, "first goForward should land on iframe page")

		_, err = p.GoBackForward(+1, opts)
		require.NoError(t, err)
		tb.AssertURL(p, url3, "second goForward should land on page2")

		for range 3 {
			_, err = p.GoBackForward(-1, opts)
			require.NoError(t, err)
			_, err = p.GoBackForward(+1, opts)
			require.NoError(t, err)
		}
		tb.AssertURL(p, url3, "after rapid navigation should still be on page2")
	})
}

// The race occurs when browser goroutines (FrameSession event loop,
// NetworkManager fire-and-forget goroutines) call PushIfNotDone on the
// k6 samples channel while close(samples) runs during engine shutdown.
// Reproduces the issue 5341 when run with the -race flag.
func TestPageCloseMetricEmissionRaceCondition(t *testing.T) {
	t.Parallel()

	samples := make(chan k6metrics.SampleContainer, 100)
	tb := newTestBrowser(t, withSamples(samples), withSkipClose())
	tb.vu.StartIteration(t)

	page := tb.NewPage(nil)
	_, err := page.Evaluate(`() => {
		window.k6browserSendWebVitalMetric(JSON.stringify({
			id: "v1-5341-1",
			name: "CLS",
			value: 0.01,
			rating: "good",
			delta: 0.01,
			numEntries: 1,
			navigationType: "navigate",
			url: window.location.href,
			spanID: ""
		}));
	}`)
	require.NoError(t, err)

	// Dispatches a pagehide event to trigger Web Vital metric emission.
	// At the same time, simulate the engine is shutting down.
	require.NoError(t, page.Close())
	close(samples)
}
