// practically none of this work on windows
//go:build !windows

package tests

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/env"
)

func TestFramePress(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)

	p := tb.NewPage(nil)

	err := p.SetContent(`<input id="text1">`, nil)
	require.NoError(t, err)

	f := p.Frames()[0]

	opts := common.NewFramePressOptions(f.Timeout())
	require.NoError(t, f.Press("#text1", "Shift+KeyA", opts))
	require.NoError(t, f.Press("#text1", "KeyB", opts))
	require.NoError(t, f.Press("#text1", "Shift+KeyC", opts))

	inputValue, err := f.InputValue("#text1", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
	require.NoError(t, err)
	require.Equal(t, "AbC", inputValue)
}

func TestFrameDismissDialogBox(t *testing.T) {
	t.Parallel()

	for _, tt := range []string{
		"alert",
		"confirm",
		"prompt",
		"beforeunload",
	} {
		t.Run(tt, func(t *testing.T) {
			t.Parallel()

			var (
				tb = newTestBrowser(t, withFileServer())
				p  = tb.NewPage(nil)
			)

			opts := &common.FrameGotoOptions{
				WaitUntil: common.LifecycleEventNetworkIdle,
				Timeout:   common.DefaultTimeout,
			}
			_, err := p.Goto(tb.staticURL("dialog.html?dialogType="+tt), opts)
			require.NoError(t, err)

			if tt == "beforeunload" {
				err = p.Click("#clickHere", common.NewFrameClickOptions(p.Timeout()))
				require.NoError(t, err)
			}

			result, ok, err := p.TextContent("#textField", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
			require.NoError(t, err)
			require.True(t, ok)
			assert.EqualValues(t, tt+" dismissed", result)
		})
	}
}

func TestFrameNoPanicWithEmbeddedIFrame(t *testing.T) {
	t.Parallel()

	// We're skipping this when running in headless
	// environments since the bug that the test fixes
	// only surfaces when in headfull mode.
	// Remove this skip once we have headfull mode in
	// CI: https://go.k6.io/k6/js/modules/k6/browser/issues/678
	if env.IsBrowserHeadless() {
		t.Skip("skipped when in headless mode")
	}

	// run the browser in headfull mode.
	tb := newTestBrowser(
		t,
		withFileServer(),
		withEnvLookup(env.ConstLookup(env.BrowserHeadless, "0")),
	)

	p := tb.NewPage(nil)
	opts := &common.FrameGotoOptions{
		WaitUntil: common.LifecycleEventDOMContentLoad,
		Timeout:   common.DefaultTimeout,
	}
	_, err := p.Goto(tb.staticURL("embedded_iframe.html"), opts)
	require.NoError(t, err)

	result, ok, err := p.TextContent("#doneDiv", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
	require.NoError(t, err)
	require.True(t, ok)
	assert.EqualValues(t, "Done!", result)
}

// Without the fix in https://go.k6.io/k6/js/modules/k6/browser/pull/942
// this test would hang on the "sign in" link click.
func TestFrameNoPanicNavigateAndClickOnPageWithIFrames(t *testing.T) {
	t.Parallel()

	// We're skipping this when running in headless
	// environments since the bug that the test fixes
	// only surfaces when in headfull mode.
	// Remove this skip once we have headfull mode in
	// CI: https://go.k6.io/k6/js/modules/k6/browser/issues/678
	if env.IsBrowserHeadless() {
		t.Skip("skipped when in headless mode")
	}

	tb := newTestBrowser(
		t,
		withFileServer(),
		withEnvLookup(env.ConstLookup(env.BrowserHeadless, "0")),
	)
	p := tb.NewPage(nil)
	tb.withHandler("/iframeSignIn", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, tb.staticURL("iframe_signin.html"), http.StatusMovedPermanently)
	})

	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	_, err := p.Goto(
		tb.staticURL("iframe_home.html"),
		opts,
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(tb.context(), 5*time.Second)
	defer cancel()

	err = tb.run(
		ctx,
		func() error { return p.Click(`a[href="/iframeSignIn"]`, common.NewFrameClickOptions(p.Timeout())) },
		func() error {
			_, err := p.WaitForNavigation(
				common.NewFrameWaitForNavigationOptions(p.Timeout()), nil,
			)
			return err
		},
	)
	require.NoError(t, err)

	result, ok, err := p.TextContent("#doneDiv", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
	require.NoError(t, err)
	require.True(t, ok)
	assert.EqualValues(t, "Sign In Page", result)
}

func TestFrameTitle(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(
		`<html><head><title>Some title</title></head></html>`,
		nil,
	)
	require.NoError(t, err)

	title, err := p.MainFrame().Title()
	assert.NoError(t, err)
	assert.Equal(t, "Some title", title)
}

func TestFrameGetAttribute(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`<a id="el" href="null">Something</a>`, nil)
	require.NoError(t, err)

	got, ok, err := p.Frames()[0].GetAttribute("#el", "href", common.NewFrameBaseOptions(p.MainFrame().Timeout()))
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "null", got)
}

func TestFrameGetAttributeMissing(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`<a id="el">Something</a>`, nil)
	require.NoError(t, err)

	got, ok, err := p.Frames()[0].GetAttribute("#el", "missing", common.NewFrameBaseOptions(p.MainFrame().Timeout()))
	require.NoError(t, err)
	require.False(t, ok)
	assert.Equal(t, "", got)
}

func TestFrameGetAttributeEmpty(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`<a id="el" empty>Something</a>`, nil)
	require.NoError(t, err)

	got, ok, err := p.Frames()[0].GetAttribute("#el", "empty", common.NewFrameBaseOptions(p.MainFrame().Timeout()))
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "", got)
}

func TestFrameSetChecked(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`<input id="el" type="checkbox">`, nil)
	require.NoError(t, err)

	isopts := common.NewFrameIsCheckedOptions(p.MainFrame().Timeout())
	checked, err := p.Frames()[0].IsChecked("#el", isopts)
	require.NoError(t, err)
	assert.False(t, checked)

	err = p.Frames()[0].SetChecked("#el", true, common.NewFrameCheckOptions(p.Frames()[0].Timeout()))
	require.NoError(t, err)
	isopts = common.NewFrameIsCheckedOptions(p.MainFrame().Timeout())
	checked, err = p.Frames()[0].IsChecked("#el", isopts)
	require.NoError(t, err)
	assert.True(t, checked)

	err = p.Frames()[0].SetChecked("#el", false, common.NewFrameCheckOptions(p.Frames()[0].Timeout()))
	require.NoError(t, err)
	isopts = common.NewFrameIsCheckedOptions(p.MainFrame().Timeout())
	checked, err = p.Frames()[0].IsChecked("#el", isopts)
	require.NoError(t, err)
	assert.False(t, checked)
}

func TestFrameWaitForURLSuccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name:     "when_already_at_matching_url",
			code:     `await frame.waitForURL(/.*waitfornavigation_test\.html$/);`,
			expected: []string{"waitfornavigation_test.html"},
		},
		{
			name: "exact_url_match",
			code: `
				await Promise.all([
					frame.waitForURL(page1URL),
					frame.locator('#page1').click()
				]);
			`,
			expected: []string{"page1.html"},
		},
		{
			name: "regex_pattern_match",
			code: `
				await Promise.all([
					frame.waitForURL(/.*2\.html$/),
					frame.locator('#page2').click()
				]);
			`,
			expected: []string{"page2.html"},
		},
		{
			name: "empty_pattern_match",
			code: `
				await Promise.all([
					frame.waitForURL(''),
					frame.locator('#page2').click()
				]);
			`,
			expected: []string{"page2.html", "waitfornavigation_test.html"},
		},
		{
			name: "waitUntil_domcontentloaded",
			code: `
				await Promise.all([
					frame.waitForURL(/.*page1\.html$/, { waitUntil: 'domcontentloaded' }),
					frame.locator('#page1').click()
				]);
			`,
			expected: []string{"page1.html"},
		},
		{
			name: "already_at_url_with_regex_pattern",
			code: `
				await frame.waitForURL(/.*\/waitfornavigation_test\.html$/);
			`,
			expected: []string{"waitfornavigation_test.html"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup
			tb := newTestBrowser(t, withFileServer())
			tb.vu.ActivateVU()
			tb.vu.StartIteration(t)

			tb.vu.SetVar(t, "frame", &sobek.Object{})
			tb.vu.SetVar(t, "testURL", tb.staticURL("waitfornavigation_test.html"))
			tb.vu.SetVar(t, "page1URL", tb.staticURL("page1.html"))
			_, err := tb.vu.RunAsync(t, `
					const page = await browser.newPage();
		
					await page.setContent('<iframe></iframe>');
		
					const iframeElement = await page.$('iframe');
					frame = await iframeElement.contentFrame();
				`)
			require.NoError(t, err)

			// Test logic
			code := fmt.Sprintf(`
			await frame.goto(testURL);

			%s
			
			return frame.url();`, tt.code)

			result := tb.vu.RunPromise(t, "%s", code)
			got := strings.ReplaceAll(result.Result().String(), tb.staticURL(""), "")
			assert.Contains(t, tt.expected, got)
		})
	}
}

func TestFrameWaitForURLFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name: "timeout_on_mismatched_url",
			code: `
				await Promise.all([
					frame.waitForURL(/.*nonexistent\.html$/, { timeout: 500 }),
					frame.locator('#page1').click()  // This goes to page1.html, not nonexistent.html
				]);
			`,
			expected: "timed out after 500ms",
		},
		{
			name: "missing_required_argument",
			code: `
				await frame.waitForURL();
			`,
			expected: "missing required argument 'url'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup
			tb := newTestBrowser(t, withFileServer())
			tb.vu.ActivateVU()
			tb.vu.StartIteration(t)

			tb.vu.SetVar(t, "frame", &sobek.Object{})
			tb.vu.SetVar(t, "testURL", tb.staticURL("waitfornavigation_test.html"))
			_, err := tb.vu.RunAsync(t, `
					const page = await browser.newPage();
		
					await page.setContent('<iframe></iframe>');
		
					const iframeElement = await page.$('iframe');
					frame = await iframeElement.contentFrame();
				`)
			require.NoError(t, err)

			// Test logic
			code := fmt.Sprintf(`
			await frame.goto(testURL);

			%s`, tt.code)

			_, err = tb.vu.RunAsync(t, "%s", code)
			assert.ErrorContains(t, err, tt.expected)
		})
	}
}
