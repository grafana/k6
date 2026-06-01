// auto_screenshot_play_grafana.js exercises a real interactive SPA
// rather than a content-heavy marketing site. play.grafana.com is
// the public Grafana sandbox: dashboards, panels, time-range picking,
// in-app navigation. Useful for characterising the auto-screenshot
// feature on a script that performs SPA-style interactions against a
// real backend.
//
// Run with auto-screenshot off and then on:
//
//   ./k6 run examples/browser/auto_screenshot_play_grafana.js
//   K6_BROWSER_AUTO_SCREENSHOT=actions ./k6 run examples/browser/auto_screenshot_play_grafana.js
//
// Expected pattern: one screenshot per browser API call, dedup-aware.
// Covers the initial dashboard list, the navigation into a dashboard,
// and an interaction inside the dashboard view.
//
// Because this script depends on a public network endpoint and the
// state of the Grafana playground can change over time, treat the
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
    // 'load' rather than 'networkidle': Grafana keeps polling and
    // never quite reaches network idle, the same way grafana.com
    // does not.
    await page.goto('https://play.grafana.com', { waitUntil: 'load' });

    // Give the SPA a moment to finish hydrating before interacting.
    await page.waitForTimeout(1000);

    // Scroll to give the page a chance to lazy-load any below-the-fold
    // content the homepage might have.
    await page.evaluate(() => window.scrollBy(0, 600));
    await page.waitForTimeout(500);

    // Read the page title via DOM rather than navigating; keeps the
    // script offline-safe in the sense that it does not depend on a
    // specific link existing.
    await page.locator('h1, h2').first().textContent();
  } finally {
    await page.close();
  }
}
