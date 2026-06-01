// auto_screenshot_grafana.js exercises a real, content-heavy marketing
// site (grafana.com): real CSS, real fonts, real analytics, real
// lazy-loaded sections, real animations. Useful to characterise the
// auto-screenshot feature under realistic page complexity rather than
// the small inline fixtures used by the other scripts.
//
// Run with auto-screenshot off and then on:
//
//   ./k6 run examples/browser/auto_screenshot_grafana.js
//   K6_BROWSER_AUTO_SCREENSHOT=actions ./k6 run examples/browser/auto_screenshot_grafana.js
//
// Expected pattern: a screenshot per browser API call, dedup-aware,
// covering the initial load and each scroll step.
//
// Because this script depends on a public network endpoint, treat its
// numbers as illustrative rather than reproducible across runs.
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
    // 'load' rather than 'networkidle': real marketing sites typically
    // never reach network idle because of continuous analytics and
    // tracking requests. Using 'networkidle' here would let the goto
    // run until the navigation timeout (~30s) and the rest of the
    // script would never execute.
    await page.goto('https://grafana.com', { waitUntil: 'load' });

    // Scroll the viewport in three steps so any lazy-loaded sections
    // get revealed and the MutationObserver path has multiple distinct
    // settle points to react to.
    await page.evaluate(() => window.scrollBy(0, 600));
    await page.waitForTimeout(500);

    await page.evaluate(() => window.scrollBy(0, 1200));
    await page.waitForTimeout(500);

    await page.evaluate(() => window.scrollBy(0, 2000));
    await page.waitForTimeout(500);

    // Read something from the page so Mode A logs at least one
    // post-scroll inspection call.
    await page.locator('h1').first().textContent();
  } finally {
    await page.close();
  }
}
