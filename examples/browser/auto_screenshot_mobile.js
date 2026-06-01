// auto_screenshot_mobile.js exercises the auto-screenshot feature
// against a mobile-emulated viewport so the produced PNGs reflect a
// phone-sized layout rather than the default desktop viewport.
// Useful to verify that viewport screenshots adapt to the context's
// configured device dimensions.
//
// Run with auto-screenshot off and then on:
//
//   ./k6 run examples/browser/auto_screenshot_mobile.js
//   K6_BROWSER_AUTO_SCREENSHOT=actions ./k6 run examples/browser/auto_screenshot_mobile.js
//
// Expected pattern: action-tagged shots whose dimensions match the
// iPhone X viewport (approximately 375x812 logical pixels, doubled
// by the device's deviceScaleFactor).
import { browser, devices } from 'k6/browser';

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
  // Object.assign rather than spread because k6's Babel does not yet
  // support spread in object literals.
  const ctxOpts = Object.assign({}, devices['iPhone X']);
  const context = await browser.newContext(ctxOpts);
  const page = await context.newPage();

  try {
    await page.goto('https://quickpizza.grafana.com/test.k6.io/', {
      waitUntil: 'load',
    });
    await page.locator('h2, h1').first().textContent();
  } finally {
    await page.close();
  }
}
