package tests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWaitForSelectorAbsent(t *testing.T) {
	t.Parallel()

	for _, api := range []string{"locator", "page", "frame", "element"} {
		for _, state := range []string{"hidden", "detached"} {
			t.Run(api+"/"+state, func(t *testing.T) {
				t.Parallel()
				tb := newTestBrowser(t)
				tb.vu.ActivateVU()
				tb.vu.StartIteration(t)
				defer tb.vu.EndIteration(t)

				_, err := tb.vu.RunAsync(t, `
                    const page = await browser.newPage();
                    try {
                        await page.setContent('<div>ready</div>');
                        const options = {state: %q, timeout: 500};
                        if (%q === 'locator') {
                            await page.locator('#absent').waitFor(options);
                        } else {
                            const target = %q === 'frame' ? page.mainFrame() : (%q === 'element' ? await page.$('body') : page);
                            const result = await target.waitForSelector('#absent', options);
                            if (result !== null) throw new Error('expected null for absent element');
                        }
                    } finally {
                        await page.close();
                    }
                `, state, api, api, api)
				require.NoError(t, err)
			})
		}
	}
}
