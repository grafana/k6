// auto_screenshot_dns_failure.js triggers a network-level navigation
// failure (DNS resolution against an RFC-2606 reserved name that is
// guaranteed not to resolve). Complements auto_screenshot_failure.js
// (which exercises selector-timeout rejections) by exercising the
// failure path for a DIFFERENT class of error: the browser never
// reaches the target URL at all.
//
// Run with auto-screenshot off and then on:
//
//   ./k6 run examples/browser/auto_screenshot_dns_failure.js
//   K6_BROWSER_AUTO_SCREENSHOT=actions ./k6 run examples/browser/auto_screenshot_dns_failure.js
//
// Expected pattern: action-tagged shots from the initial setContent
// and inspection, plus a failure-tagged shot at the moment the bad
// goto rejects. The failure shot captures whatever page the browser
// was on before the failed navigation (the fixture HTML), not an
// error page.
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
    <h1>Pre-navigation state</h1>
    <p>The next navigation will fail with a DNS error.</p>
  </body>
</html>`;

export default async function () {
  const context = await browser.newContext();
  const page = await context.newPage();

  try {
    await page.setContent(fixtureHTML);
    await page.locator('h1').textContent();

    try {
      await page.goto('http://this-host-does-not-exist-12345.invalid', {
        timeout: 5000,
      });
    } catch (err) {
      // Expected: DNS lookup fails.
    }
  } finally {
    await page.close();
  }
}
