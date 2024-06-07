package tests

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/common"
)

type emulateMediaOpts struct {
	Media         string `js:"media"`
	ColorScheme   string `js:"colorScheme"`
	ReducedMotion string `js:"reducedMotion"`
}

type jsFrameBaseOpts struct {
	Timeout string
	Strict  bool
}

const sampleHTML = `<div><b>Test</b><ol><li><i>One</i></li></ol></div>`

func TestNestedFrames(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t,
		withFileServer(),
	)
	defer tb.Browser.Close()

	page := tb.NewPage(nil)
	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	_, err := page.Goto(
		tb.staticURL("iframe_test_main.html"),
		opts,
	)
	require.NoError(t, err)

	frame1Handle, err := page.WaitForSelector("iframe[id='iframe1']", nil)
	assert.Nil(t, err)
	assert.NotNil(t, frame1Handle)

	frame1, err := frame1Handle.ContentFrame()
	assert.Nil(t, err)
	assert.NotNil(t, frame1)

	frame2Handle, err := frame1.WaitForSelector("iframe[id='iframe2']", nil)
	assert.Nil(t, err)
	assert.NotNil(t, frame2Handle)

	frame2, err := frame2Handle.ContentFrame()
	assert.Nil(t, err)
	assert.NotNil(t, frame2)

	button1Handle, err := frame2.WaitForSelector("button[id='button1']", nil)
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

	err := p.EmulateMedia(tb.toSobekValue(emulateMediaOpts{
		Media:         "print",
		ColorScheme:   "dark",
		ReducedMotion: "reduce",
	}))
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
			tc := tc
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu, _, _, cleanUp := startIteration(t)
			defer cleanUp()

			// Test script as non string input
			vu.SetVar(t, "p", &sobek.Object{})
			got := vu.RunPromise(t, `
				p = await browser.newPage()
				return await p.evaluate(%s)
			`, tt.script)
			assert.Equal(t, vu.ToSobekValue(tt.want), got.Result())

			// Test script as string input
			got = vu.RunPromise(t,
				`return await p.evaluate("%s")`,
				tt.script,
			)
			assert.Equal(t, vu.ToSobekValue(tt.want), got.Result())
		})
	}
}

func TestPageEvaluateMappingError(t *testing.T) {
	t.Parallel()

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
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu, _, _, cleanUp := startIteration(t)
			defer cleanUp()

			// Test script as non string input
			vu.SetVar(t, "p", &sobek.Object{})
			_, err := vu.RunAsync(t, `
				p = await browser.newPage()
				await p.evaluate(%s)
			`, tt.script)
			assert.ErrorContains(t, err, tt.wantErr)

			// Test script as string input
			_, err = vu.RunAsync(t, `
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
		innerHTML, err := p.InnerHTML("div", nil)
		require.NoError(t, err)
		assert.Equal(t, `<b>Test</b><ol><li><i>One</i></li></ol>`, innerHTML)
	})

	t.Run("err_empty_selector", func(t *testing.T) {
		t.Parallel()

		defer func() {
		}()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		_, err := p.InnerHTML("", nil)
		require.ErrorContains(t, err, "The provided selector is empty")
	})

	t.Run("err_wrong_selector", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		err := p.SetContent(sampleHTML, nil)
		require.NoError(t, err)
		_, err = p.InnerHTML("p", tb.toSobekValue(jsFrameBaseOpts{Timeout: "100"}))
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
		innerText, err := p.InnerText("div", nil)
		require.NoError(t, err)
		assert.Equal(t, "Test\nOne", innerText)
	})

	t.Run("err_empty_selector", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		_, err := p.InnerText("", nil)
		require.ErrorContains(t, err, "The provided selector is empty")
	})

	t.Run("err_wrong_selector", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		err := p.SetContent(sampleHTML, nil)
		require.NoError(t, err)
		_, err = p.InnerText("p", tb.toSobekValue(jsFrameBaseOpts{Timeout: "100"}))
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
		textContent, ok, err := p.TextContent("div", nil)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, "TestOne", textContent)
	})

	t.Run("err_empty_selector", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		_, _, err := p.TextContent("", nil)
		require.ErrorContains(t, err, "The provided selector is empty")
	})

	t.Run("err_wrong_selector", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		err := p.SetContent(sampleHTML, nil)
		require.NoError(t, err)
		_, _, err = p.TextContent("p", tb.toSobekValue(jsFrameBaseOpts{
			Timeout: "100",
		}))
		require.Error(t, err)
	})

	t.Run("err_wrong_selector", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		err := p.SetContent(sampleHTML, nil)
		require.NoError(t, err)
		_, _, err = p.TextContent("p", tb.toSobekValue(jsFrameBaseOpts{
			Timeout: "100",
		}))
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

	inputValue, err := p.InputValue("input", nil)
	require.NoError(t, err)
	got, want := inputValue, "hello1"
	assert.Equal(t, got, want)

	inputValue, err = p.InputValue("select", nil)
	require.NoError(t, err)
	got, want = inputValue, "hello2"
	assert.Equal(t, got, want)

	inputValue, err = p.InputValue("textarea", nil)
	require.NoError(t, err)
	got, want = inputValue, "hello3"
	assert.Equal(t, got, want)
}

// test for: https://github.com/grafana/xk6-browser/issues/132
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
		require.NoError(t, el.Fill("", nil))
		require.NoError(t, el.Type(want, nil))

		got, err := el.InputValue(nil)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
}

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
			err := p.Fill(tt.selector, tt.value, nil)
			require.NoError(t, err)
			inputValue, err := p.InputValue(tt.selector, nil)
			require.NoError(t, err)
			require.Equal(t, tt.value, inputValue)
		})
	}
	for _, tt := range sad {
		t.Run("sad/"+tt.name, func(t *testing.T) {
			err := p.Fill(tt.selector, tt.value, nil)
			require.Error(t, err)
		})
	}
}

func TestPageIsChecked(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)

	err := p.SetContent(`<input type="checkbox" checked>`, nil)
	require.NoError(t, err)
	checked, err := p.IsChecked("input", nil)
	require.NoError(t, err)
	assert.True(t, checked, "expected checkbox to be checked")

	err = p.SetContent(`<input type="checkbox">`, nil)
	require.NoError(t, err)
	checked, err = p.IsChecked("input", nil)
	require.NoError(t, err)
	assert.False(t, checked, "expected checkbox to be unchecked")
}

func TestPageScreenshotFullpage(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	err := p.SetViewportSize(tb.toSobekValue(struct {
		Width  float64 `js:"width"`
		Height float64 `js:"height"`
	}{
		Width: 1280, Height: 800,
	}))
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
		div.style.background = 'linear-gradient(red, blue)';

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

	assert.Equal(t, 1280, img.Bounds().Max.X, "screenshot width is not 1280px as expected, but %dpx", img.Bounds().Max.X)
	assert.Equal(t, 800, img.Bounds().Max.Y, "screenshot height is not 800px as expected, but %dpx", img.Bounds().Max.Y)

	r, _, b, _ := img.At(0, 0).RGBA()
	assert.Greater(t, r, uint32(128))
	assert.Less(t, b, uint32(128))
	r, _, b, _ = img.At(0, 799).RGBA()
	assert.Less(t, r, uint32(128))
	assert.Greater(t, b, uint32(128))
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

	t.Run("ok_func_raf_default", func(t *testing.T) {
		t.Parallel()

		vu, _, log, cleanUp := startIteration(t)
		defer cleanUp()

		vu.SetVar(t, "page", &sobek.Object{})
		_, err := vu.RunOnEventLoop(t, `fn = () => {
			if (typeof window._cnt == 'undefined') window._cnt = 0;
			if (window._cnt >= 50) return true;
			window._cnt++;
			return false;
		}`)
		require.NoError(t, err)

		_, err = vu.RunAsync(t, script, "fn", "{}", "null")
		require.NoError(t, err)
		assert.Contains(t, *log, "ok: null")
	})

	t.Run("ok_func_raf_default_arg", func(t *testing.T) {
		t.Parallel()

		vu, _, log, cleanUp := startIteration(t)
		defer cleanUp()

		_, err := vu.RunOnEventLoop(t, `fn = arg => {
			window._arg = arg;
			return true;
		}`)
		require.NoError(t, err)

		_, err = vu.RunAsync(t, script, "fn", "{}", `"raf_arg"`)
		require.NoError(t, err)
		assert.Contains(t, *log, "ok: null")

		p := vu.RunPromise(t, `return await page.evaluate(() => window._arg);`)
		require.Equal(t, p.State(), sobek.PromiseStateFulfilled)
		assert.Equal(t, "raf_arg", p.Result().String())
	})

	t.Run("ok_func_raf_default_args", func(t *testing.T) {
		t.Parallel()

		vu, rt, log, cleanUp := startIteration(t)
		defer cleanUp()

		_, err := vu.RunOnEventLoop(t, `fn = (...args) => {
			window._args = args;
			return true;
		}`)
		require.NoError(t, err)

		args := []int{1, 2, 3}
		argsJS, err := json.Marshal(args)
		require.NoError(t, err)

		_, err = vu.RunAsync(t, script, "fn", "{}", "..."+string(argsJS))
		require.NoError(t, err)
		assert.Contains(t, *log, "ok: null")

		p := vu.RunPromise(t, `return await page.evaluate(() => window._args);`)
		require.Equal(t, p.State(), sobek.PromiseStateFulfilled)
		var gotArgs []int
		_ = rt.ExportTo(p.Result(), &gotArgs)
		assert.Equal(t, args, gotArgs)
	})

	t.Run("err_expr_raf_timeout", func(t *testing.T) {
		t.Parallel()

		vu, _, _, cleanUp := startIteration(t)
		defer cleanUp()

		_, err := vu.RunAsync(t, script, "false", "{ polling: 'raf', timeout: 500 }", "null")
		require.ErrorContains(t, err, "timed out after 500ms")
	})

	t.Run("err_wrong_polling", func(t *testing.T) {
		t.Parallel()

		vu, _, _, cleanUp := startIteration(t)
		defer cleanUp()

		_, err := vu.RunAsync(t, script, "false", "{ polling: 'blah' }", "null")
		require.Error(t, err)
		assert.Contains(t, err.Error(),
			`parsing waitForFunction options: wrong polling option value:`,
			`"blah"; possible values: "raf", "mutation" or number`)
	})

	t.Run("ok_expr_poll_interval", func(t *testing.T) {
		t.Parallel()

		vu, _, log, cleanUp := startIteration(t)
		defer cleanUp()

		vu.SetVar(t, "page", &sobek.Object{})
		_, err := vu.RunAsync(t, `
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
		_, err = vu.RunAsync(t, script, `"document.querySelector('h1')"`, "{ polling: 100, timeout: 2000, }", "null")
		require.NoError(t, err)
		assert.Contains(t, *log, "ok: Hello")
	})

	t.Run("ok_func_poll_mutation", func(t *testing.T) {
		t.Parallel()

		vu, _, log, cleanUp := startIteration(t)
		defer cleanUp()

		vu.SetVar(t, "page", &sobek.Object{})
		_, err := vu.RunAsync(t, `
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

		_, err = vu.RunAsync(t, script, "fn", "{ polling: 'mutation', timeout: 2000, }", "null")
		require.NoError(t, err)
		assert.Contains(t, *log, "ok: null")
	})
}

func TestPageWaitForLoadState(t *testing.T) {
	t.Parallel()

	t.Run("err_wrong_event", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		err := p.WaitForLoadState("none", nil)
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
		common.NewFrameWaitForNavigationOptions(p.Timeout()),
	)
	require.ErrorContains(t, err, "canceled")
}

func TestPagePress(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)

	p := tb.NewPage(nil)

	err := p.SetContent(`<input id="text1">`, nil)
	require.NoError(t, err)

	require.NoError(t, p.Press("#text1", "Shift+KeyA", nil))
	require.NoError(t, p.Press("#text1", "KeyB", nil))
	require.NoError(t, p.Press("#text1", "Shift+KeyC", nil))

	inputValue, err := p.InputValue("#text1", nil)
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

		err := p.Close(nil)
		assert.NoError(t, err)
	})

	t.Run("page_from_browserContext", func(t *testing.T) {
		t.Parallel()

		b := newTestBrowser(t, withHTTPServer())

		c, err := b.NewContext(nil)
		require.NoError(t, err)
		p, err := c.NewPage()
		require.NoError(t, err)

		err = p.Close(nil)
		assert.NoError(t, err)
	})
}

func TestPageOn(t *testing.T) {
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
		tc := tc
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
			eventHandlerOne := func(cm *common.ConsoleMessage) {
				defer close(done1)
				tc.assertFn(t, cm)
			}

			eventHandlerTwo := func(cm *common.ConsoleMessage) {
				defer close(done2)
				tc.assertFn(t, cm)
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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withHTTPServer())

			tb.withHandler("/slow", func(w http.ResponseWriter, _ *http.Request) {
				time.Sleep(100 * time.Millisecond)
				fmt.Fprintf(w, `sorry for being so slow`)
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

	testCases := []struct {
		name      string
		url       string
		opts      map[string]any
		selector  string
		errAssert func(*testing.T, error)
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
			opts: map[string]any{
				// set a timeout smaller than the time
				// it takes the element to show up
				"timeout": "50",
			},
			selector: "#my-div",
			errAssert: func(t *testing.T, e error) {
				t.Helper()
				assert.ErrorContains(t, e, "timed out after")
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
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

			_, err = page.WaitForSelector(tc.selector, tb.toSobekValue(tc.opts))
			tc.errAssert(t, err)
		})
	}
}

func TestPageThrottleNetwork(t *testing.T) {
	t.Parallel()

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
		tc := tc
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

				fmt.Fprint(w, string(bb))
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
			_, err = page.WaitForSelector(selector, nil)
			require.NoError(t, err)

			resp, err := page.InnerText(selector, nil)
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

	tb := newTestBrowser(t, withFileServer())

	tb.withHandler("/ping", func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			err := req.Body.Close()
			require.NoError(t, err)
		}()
		bb, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		fmt.Fprint(w, string(bb))
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
	for i := 0; i < iterations; i++ {
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
		_, err = page.WaitForSelector(selector, nil)
		require.NoError(t, err)

		ms += time.Since(start).Abs().Milliseconds()
	}

	return ms / int64(iterations)
}

func TestPageIsVisible(t *testing.T) {
	t.Parallel()

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
		tc := tc
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

			got, err := page.IsVisible(tc.selector, tb.toSobekValue(tc.options))

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

	testCases := []struct {
		name     string
		selector string
		options  common.FrameIsVisibleOptions
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
			options: common.FrameIsVisibleOptions{
				Strict: true,
			},
			wantErr: "error:strictmodeviolation",
		},
	}

	for _, tc := range testCases {
		tc := tc
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

			got, err := page.IsHidden(tc.selector, tb.toSobekValue(tc.options))

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

	// Start a server that will return static html files.
	mux := http.NewServeMux()
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)

	const (
		slash = string(os.PathSeparator)
		path  = slash + testBrowserStaticDir + slash
	)
	fs := http.FileServer(http.Dir(testBrowserStaticDir))
	mux.Handle(path, http.StripPrefix(path, fs))

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu, _, _, cleanUp := startIteration(t)
			defer cleanUp()

			got := vu.RunPromise(t, `
				const p = await browser.newPage()
				await p.goto("%s/%s/shadow_and_doc_frag.html")

				const s = p.locator('%s')
				await s.waitFor({
					timeout: 1000,
					state: 'attached',
				});

				const text = await s.innerText();
				return text;
 			`, s.URL, testBrowserStaticDir, tt.selector)
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

	err = p2.WaitForLoadState(common.LifecycleEventLoad.String(), nil)
	require.NoError(t, err)

	// Now there should be 2 pages.
	pp = p.Context().Pages()
	assert.Equal(t, 2, len(pp))

	// Make sure the new page contains the correct page.
	got, err := p2.InnerHTML("h1", nil)
	require.NoError(t, err)
	assert.Equal(t, "you clicked!", got)
}

func TestPageGetAttribute(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`<a id="el" href="null">Something</a>`, nil)
	require.NoError(t, err)

	got, ok, err := p.GetAttribute("#el", "href", nil)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "null", got)
}

func TestPageGetAttributeMissing(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`<a id="el">Something</a>`, nil)
	require.NoError(t, err)

	got, ok, err := p.GetAttribute("#el", "missing", nil)
	require.NoError(t, err)
	require.False(t, ok)
	assert.Equal(t, "", got)
}

func TestPageGetAttributeEmpty(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`<a id="el" empty>Something</a>`, nil)
	require.NoError(t, err)

	got, ok, err := p.GetAttribute("#el", "empty", nil)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "", got)
}
