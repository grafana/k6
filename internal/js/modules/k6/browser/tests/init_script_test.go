package tests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBrowserContextAddInitScript(t *testing.T) {
	t.Parallel()

	for name, script := range map[string]string{
		"string":         `"Math.random = () => 0.42"`,
		"function":       `function () { Math.random = () => 0.42; }`,
		"named_function": `function initialize() { Math.random = () => 0.42; }`,
		"arrow_function": `() => { Math.random = () => 0.42; }`,
		"async_function": `async function () { Math.random = () => 0.42; }`,
		"no_arguments":   `function () { if (arguments.length !== 0) throw new Error('unexpected arguments'); Math.random = () => 0.42; }`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t)
			tb.vu.StartIteration(t)
			defer tb.vu.EndIteration(t)

			_, err := tb.vu.RunAsync(t, `
				const context = await browser.newContext();
				await context.addInitScript(%s);
				const page = await context.newPage();
				for (let i = 0; i < 2; i++) {
					await page.goto('about:blank');
					const value = await page.evaluate(() => Math.random());
					if (value !== 0.42) {
						throw new Error('init script did not run: ' + value);
					}
				}
				await context.close();
			`, script)
			require.NoError(t, err)
		})
	}
}
