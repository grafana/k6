import { chromium } from 'k6/browser';

// Connect to an external CDP browser with a URL computed at runtime.
//
// For instance, you could fetch that URL from your browser provider's API,
// in setup(), and pass it to the iterations:
//
//   import http from 'k6/http';
//
//   export function setup() {
//     const res = http.post('https://provider.example/v1/sessions', /* ... */);
//     return { wsURL: res.json().connectUrl };
//   }
//
//   export default async function (data) {
//     const browser = await chromium.connectOverCDP(data.wsURL);
//     // ...
//   }
//
// For a quick local run, start Chrome with --remote-debugging-port=9222 and pass
// its ws URL via an env var — e.g., CDP_WS_URL, simulating a URL computed at runtime:
//   CDP_WS_URL=ws://localhost:9222/devtools/browser/<id> k6 run examples/browser/remote.js
//
// Note this is different from K6_BROWSER_WS_URL, which covers a different path where
// k6 users can provide the ws URL via an env var, not computed at runtime.

export default async function () {
  const browser = await chromium.connectOverCDP(__ENV.CDP_WS_URL);
  const page = await browser.newPage();

  try {
    await page.goto('https://quickpizza.grafana.com/', { waitUntil: 'networkidle' });
    console.log(`title: ${await page.title()}`);
  } finally {
    await page.close();
    // For user-managed browsers (those created via chromium.connectOverCDP),
    // close() is exposed so you can release the connection early.
    // It's optional, k6 auto-closes it at iteration end, just like k6-managed browsers.
    await browser.close();
  }
}
