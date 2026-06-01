// auto_screenshot_queryheavy.js issues many read-only browser API calls
// (predicates and getters) against an otherwise stable page. Use it to
// characterise how aggressively the auto-screenshot feature produces
// redundant capture requests, and how effective the CRC32-based dedup
// in the capturer is at suppressing them.
//
// Run with auto-screenshot off and then on:
//
//   ./k6 run examples/browser/auto_screenshot_queryheavy.js
//   K6_BROWSER_AUTO_SCREENSHOT=actions ./k6 run examples/browser/auto_screenshot_queryheavy.js
//
// Expected pattern: many capture requests (~one per API call) but the
// dedup path collapses them to ~one persisted file because the page
// never changes.
import { browser } from 'k6/browser';

export const options = {
  scenarios: {
    ui: {
      executor: 'shared-iterations',
      options: {
        browser: {
          type: 'chromium',
        },
      },
    },
  },
};

const stableHTML = `<!DOCTYPE html>
<html>
  <body>
    <h1 id="greeting">Hello</h1>
    <ul>
      <li id="a">one</li>
      <li id="b">two</li>
      <li id="c">three</li>
    </ul>
  </body>
</html>`;

export default async function () {
  const context = await browser.newContext();
  const page = await context.newPage();

  try {
    await page.setContent(stableHTML);

    // Twenty inspection-style calls against a page that does not
    // change. Every successful call still triggers the auto-screenshot
    // capture path in Mode A.
    for (let i = 0; i < 20; i++) {
      await page.locator('#greeting').textContent();
      await page.locator('#a').isVisible();
    }
  } finally {
    await page.close();
  }
}
