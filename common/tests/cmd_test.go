package tests

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.k6.io/k6/cmd"
	k6tests "go.k6.io/k6/cmd/tests"
)

func TestCmd(t *testing.T) {
	t.Skip("data race")

	t.Parallel()

	ts := k6tests.NewGlobalTestState(t)

	ts.CmdArgs = []string{
		"k6", "run", "-",
	}
	ts.Stdin = bytes.NewBufferString(`
		import { browser } from 'k6/browser';

		export const options = {
		scenarios: {
			browser: {
				executor: 'shared-iterations',
				options: {
				browser: {
					type: 'chromium',
				},
				},
			},
		},
		};

		export default async function () {
		const page = await browser.newPage();
		await page.goto('https://test.k6.io/browser.php');
		const options = page.locator('#numbers-options');
		await options.selectOption({label:'Five'});
		await options.selectOption({index:5});
		await options.selectOption({value:'five'});
		await options.selectOption({label:'Five'});
		await options.selectOption('Five'); // Value or label
		
		await options.selectOption([{label:'Five'}]);
		await options.selectOption(['Five']); // Value or label
		}
	`)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.Empty(t, ts.Stderr.String())
}
