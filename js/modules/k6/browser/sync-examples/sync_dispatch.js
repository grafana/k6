import { check } from 'k6';
import { browser } from 'k6/x/browser';

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
  const context = browser.newContext();
  const page = context.newPage();

  try {
    await page.goto('https://test.k6.io/', { waitUntil: 'networkidle' });

    page.locator('a[href="/contacts.php"]')
        .dispatchEvent("click");

    check(page, {
      header: (p) => p.locator("h3").textContent() == "Contact us",
    });
  } finally {
    page.close();
  }
}
