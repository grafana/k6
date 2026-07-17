import { chromium } from 'k6/browser';

// Connect to an external CDP browser with a URL computed at runtime.
//
// Usage:
//   K6_BROWSER_WS_URL=ws://localhost:9222 k6 run examples/browser/remote.js
//
// In real usage the URL is typically fetched from a provider API (e.g. in
// setup()) rather than read from an env var.

export default async function () {
  const browser = await chromium.connectOverCDP(__ENV.K6_BROWSER_WS_URL);
  const page = await browser.newPage();

  try {
    await page.goto('https://quickpizza.grafana.com/', { waitUntil: 'networkidle' });
    console.log(`title: ${await page.title()}`);
  } finally {
    // close() is optional — k6 auto-closes the connection at iteration end.
    await page.close();
    await browser.close();
  }
}
