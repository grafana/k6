import { chromium } from 'k6/browser';

// Connect to an external browser via K6_BROWSER_WS_URL.
// No options.browser — k6 won't launch or manage a browser.
//
// Usage:
//   K6_BROWSER_WS_URL=ws://localhost:9222 k6 run examples/browser/remote.js

export default async function () {
  const browser = await chromium.connectOverCDP(__ENV.K6_BROWSER_WS_URL);
  const page = await browser.newPage();

  try {
    await page.goto('https://quickpizza.grafana.com/', { waitUntil: 'networkidle' });
    console.log(`title: ${await page.title()}`);
  } finally {
    await page.close();
    await browser.close();
  }
}
