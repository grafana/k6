// practically none of this work on windows
//go:build !windows

package tests

import (
	"encoding/json"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestQueryNestedShadowDOM checks which elements each selector engine finds
// across nested shadow roots, and in which order.
func TestQueryNestedShadowDOM(t *testing.T) {
	t.Parallel()

	tests := []struct {
		selector string
		want     []string // text of the matches, in order
	}{
		{selector: `button`, want: []string{"Light", "Outer", "Inner"}},
		{selector: `button >> nth=1`, want: []string{"Outer"}},
		{selector: `button >> nth=2`, want: []string{"Inner"}},
		{selector: `#inner-host >> button`, want: []string{"Inner"}},
		{selector: `#outer-host >> internal:role=button`, want: []string{"Outer", "Inner"}},
		{selector: `internal:role=button`, want: []string{"Light", "Outer", "Inner"}},
		{selector: `internal:role=button[name="Inner"i]`, want: []string{"Inner"}},
		{selector: `internal:role=button >> nth=1`, want: []string{"Outer"}},
		{selector: `internal:text="Inner text"i`, want: []string{"Inner text"}},
		{selector: `internal:attr=[title="Tip"s]`, want: []string{"Outer text"}},
		{selector: `internal:attr=[placeholder="Inner field"i]`, want: []string{"Inner field"}},
		{selector: `internal:label="Name"i`, want: []string{"INPUT"}},
		{selector: `div >> internal:has-text="Inner text"i`, want: []string{"DIV", "DIV"}},
		{selector: `div >> internal:has-not-text="Inner text"i`, want: []string{}},
	}

	tb := newTestBrowser(t, withFileServer())
	tb.vu.ActivateVU()
	tb.vu.StartIteration(t)

	selectors := make([]string, 0, len(tests))
	for _, tt := range tests {
		selectors = append(selectors, tt.selector)
	}
	selectorsJSON, err := json.Marshal(selectors)
	require.NoError(t, err)

	got := tb.vu.RunPromise(t, `
		const page = await browser.newPage();
		await page.goto('%s');
		page.setDefaultTimeout(1000);
		const result = { selectors: {} };
		for (const selector of %s) {
			const texts = [];
			for (const handle of await page.$$(selector))
				texts.push((await handle.evaluate((e) => e.textContent || e.placeholder || e.tagName)).trim());
			result.selectors[selector] = texts;
		}
		try {
			result.locatorNth = await page.locator('button').nth(2).textContent();
		} catch (e) {
			result.locatorNth = String(e);
		}
		return JSON.stringify(result);
	`, tb.staticURL("shadow_dom_nested.html"), string(selectorsJSON))
	require.Equal(t, sobek.PromiseStateFulfilled, got.State(), "%v", got.Result())

	var result struct {
		Selectors  map[string][]string `json:"selectors"`
		LocatorNth string              `json:"locatorNth"`
	}
	require.NoError(t, json.Unmarshal([]byte(got.Result().String()), &result))
	for _, tt := range tests {
		assert.Equal(t, tt.want, result.Selectors[tt.selector], tt.selector)
	}
	assert.Equal(t, "Inner", result.LocatorNth, "nth() should find a button in a nested shadow root")
}
