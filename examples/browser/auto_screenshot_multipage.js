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
    await page.goto('https://quickpizza.grafana.com/test.k6.io/', {
      waitUntil: 'networkidle',
    });

    // Navigate to the messages page.
    await Promise.all([
      page.waitForNavigation(),
      page.getByRole('link', { name: '/my_messages.php' }).click(),
    ]);

    // Fill the login form and submit.
    await page.locator('input[name="login"]').type('admin');
    await page.locator('input[name="password"]').type('123');
    await Promise.all([
      page.waitForNavigation(),
      page.getByText('Go!').click(),
    ]);

    // Read post-login content to give Mode B a quiet period to settle.
    await page.locator('h2').textContent();
  } finally {
    await page.close();
  }
}
