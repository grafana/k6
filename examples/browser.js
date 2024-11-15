import { check } from 'k6';
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
  thresholds: {
    checks: ["rate==1.0"]
  }
}

export default async function() {
  const context = await browser.newContext();
  const page = await context.newPage();

  try {
    // Goto front page, find login link and click it
    await page.goto('https://test.k6.io/', { waitUntil: 'networkidle' });
    await Promise.all([
      page.waitForNavigation(),
      page.locator('a[href="/my_messages.php"]').click(),
    ]);
    // Enter login credentials and login
    await page.locator('input[name="login"]').type("admin");
    await page.locator('input[name="password"]').type("123");
    // We expect the form submission to trigger a navigation, so to prevent a
    // race condition, setup a waiter concurrently while waiting for the click
    // to resolve.
    await Promise.all([
      page.waitForNavigation(),
      page.locator('input[type="submit"]').click(),
    ]);
    const content = await page.locator("h2").textContent();
    check(content, {
      'header': content => content == 'Welcome, admin!',
    });
  } finally {
    await page.close();
  }
}
