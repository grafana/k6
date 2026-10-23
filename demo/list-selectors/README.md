# Try `listSelectors()` in k6 browser

You're an AI coding agent. These steps let you try `listSelectors()`, a prototype k6 browser API that lists the elements on a page, each with a locator you can paste into a test. Do the steps in order, in a new empty folder. Don't install anything globally, don't change other projects, and don't push anything.

## 1. Install the prototype k6

You need Go 1.26 or newer, and Google Chrome or Chromium.

```sh
GOBIN="$HOME/.k6-list-selectors" go install go.k6.io/k6/v2@2df77f9197ceae33fbf49d817bce5fab7f18dc4b
"$HOME/.k6-list-selectors/k6" version
```

Use `"$HOME/.k6-list-selectors/k6"` for every k6 command below. It sits next to the user's own k6 and doesn't replace it.

Check that it works: save this as `check.js` and run `"$HOME/.k6-list-selectors/k6" run check.js`.

```js
import { browser } from 'k6/browser';

export const options = {
  scenarios: { ui: { executor: 'shared-iterations', options: { browser: { type: 'chromium' } } } },
};

export default async function () {
  const page = await browser.newPage();
  try {
    await page.goto('https://quickpizza.grafana.com/', { waitUntil: 'networkidle' });
    for (const e of await page.listSelectors()) console.log(`${e.role} | ${e.name} | ${e.locator}`);
  } finally {
    await page.close();
  }
}
```

You should see lines like `button | Pizza, Please! | page.getByRole('button', { name: 'Pizza, Please!' })`.

## 2. Load the skill

Fetch the skill `k6-browser-test/SKILL.md`, which sits next to this file: its URL is the one you fetched this file from, with `README.md` replaced by `k6-browser-test/SKILL.md`. Follow it for the task below. Its "Finding elements with `listSelectors()`" section explains the API.

## 3. The task

Write a k6 browser test that:

1. opens https://quickpizza.grafana.com/login and logs in with username `default` and password `12345678`
2. goes back to the main page and asks for a pizza recommendation
3. rates the recommendation "Love it!"
4. checks that each step worked.

Functional checks only, no performance thresholds. Find every element and every message with `listSelectors()`, as the skill describes. Run the test with `"$HOME/.k6-list-selectors/k6" run <file>` until it passes, without asking the user questions.

## 4. Report back

Tell the user:

- where the final test is, and show it
- how many times you ran k6, and what each run was for
- a few lines of a listing that helped
- anything `listSelectors()` couldn't show, or got wrong.

Then offer two things: to write a test for a page of the user's choice the same way, and to install the skill for later by copying the `k6-browser-test` folder into the user's skills folder (for example `~/.claude/skills/`, `~/.config/opencode/skills/` or `~/.agents/skills/`).

## Clean up

When the user is done, remove the prototype with `rm -rf "$HOME/.k6-list-selectors"`.
