package tests

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClickModifiers(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		action string
		double bool
	}{
		{"page", `page.click('#target', options)`, false},
		{"locator", `page.locator('#target').click(options)`, false},
		{"element", `(await page.$('#target')).click(options)`, false},
		{"page_double", `page.dblclick('#target', options)`, true},
		{"locator_double", `page.locator('#target').dblclick(options)`, true},
		{"element_double", `(await page.$('#target')).dblclick(options)`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t)
			tb.vu.ActivateVU()
			tb.vu.StartIteration(t)
			defer tb.vu.EndIteration(t)

			result := tb.vu.RunPromise(t, `
				const page = await browser.newPage();
				try {
					await page.setContent('<button id="target">Click</button>');
					await page.evaluate(() => {
						window.events = [];
						document.querySelector('#target').addEventListener('mousedown', e => {
							window.events.push([e.altKey, e.ctrlKey, e.metaKey, e.shiftKey]);
						});
					});
					await page.keyboard.down('Shift');
					const options = { modifiers: ['ControlOrMeta', 'Alt'] };
					await %s;
					await page.click('#target');
					await page.click('#target', { modifiers: [] });
					await page.click('#target');
					await page.keyboard.up('Shift');
					return JSON.stringify(await page.evaluate(() => window.events));
				} finally {
					await page.close();
				}
			`, tc.action)

			modified := fmt.Sprintf(`[true,%t,%t,false]`, runtime.GOOS != "darwin", runtime.GOOS == "darwin")
			want := modified
			if tc.double {
				want += "," + modified
			}
			want = "[" + want + ",[false,false,false,true],[false,false,false,false],[false,false,false,true]]"
			assert.JSONEq(t, want, result.Result().String())
		})
	}
}
