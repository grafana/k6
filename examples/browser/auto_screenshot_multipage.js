// auto_screenshot_multipage.js exercises a multi-page server-rendered
// navigation flow. Use it to characterise the auto-screenshot feature
// on a script that performs full page transitions.
//
// Run with auto-screenshot off and then on:
//
//   ./k6 run examples/browser/auto_screenshot_multipage.js
//   K6_BROWSER_AUTO_SCREENSHOT=actions ./k6 run examples/browser/auto_screenshot_multipage.js
//
// Expected pattern: ~one screenshot per browser API call, deduplicated
// by CRC32 so identical frames collapse. Failure-path captures fire
// additionally when a browser API call rejects (e.g. selector timeout).
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

export default async function () {
  const context = await browser.newContext();
  const page = await context.newPage();

  try {
    // 'load' rather than 'networkidle': quickpizza issues background
    // requests after first paint and effectively never reaches
    // network idle, so 'networkidle' would risk hitting the
    // navigation timeout.
    await page.goto('https://quickpizza.grafana.com', { waitUntil: 'load' });
    await page.waitForTimeout(500);

    // First interaction: click the primary CTA on the home page.
    // Defensive try/catch keeps the script robust if the page's
    // top-level layout changes; the auto-screenshot capture path
    // still fires for whatever did happen.
    try {
      await page
        .getByRole('button')
        .first()
        .click({ timeout: 3000 });
      await page.waitForTimeout(1500);
    } catch (_) {
      // Selector miss: fall back to a scroll so there is still a
      // meaningful state change for the capturer to observe.
      await page.evaluate(() => window.scrollBy(0, 400));
      await page.waitForTimeout(500);
    }

    // Second navigation: hit the admin route. quickpizza serves an
    // admin landing or login page here; either way it is a full page
    // transition distinct from the home page.
    await page.goto('https://quickpizza.grafana.com/admin', {
      waitUntil: 'load',
    });
    await page.waitForTimeout(500);

    // Read something visible so Mode A logs at least one inspection
    // call against the post-navigation page.
    await page.locator('h1, h2').first().textContent();
  } finally {
    await page.close();
  }
}
