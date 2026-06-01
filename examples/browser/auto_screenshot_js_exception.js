// auto_screenshot_js_exception.js triggers a JavaScript exception
// inside a page.evaluate() call to exercise the auto-screenshot
// failure path for a DIFFERENT class of error than selector timeouts
// (auto_screenshot_failure.js) and navigation failures
// (auto_screenshot_dns_failure.js). The exception is raised by the
// user-supplied callback running inside Chrome, not by the browser
// module itself.
//
// Run with auto-screenshot off and then on:
//
//   ./k6 run examples/browser/auto_screenshot_js_exception.js
//   K6_BROWSER_AUTO_SCREENSHOT=actions ./k6 run examples/browser/auto_screenshot_js_exception.js
//
// Expected pattern: action-tagged shots from the setup calls, plus a
// failure-tagged shot at the moment evaluate() rejects with the
// exception thrown by the user's callback.
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

const fixtureHTML = `<!DOCTYPE html>
<html>
  <body>
    <h1>JS exception fixture</h1>
    <p>The next evaluate() will throw.</p>
  </body>
</html>`;

export default async function () {
  const context = await browser.newContext();
  const page = await context.newPage();

  try {
    await page.setContent(fixtureHTML);
    await page.locator('h1').textContent();

    try {
      await page.evaluate(() => {
        throw new Error('intentional test exception from page.evaluate');
      });
    } catch (err) {
      // Expected.
    }
  } finally {
    await page.close();
  }
}
