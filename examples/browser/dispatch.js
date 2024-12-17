import { browser } from 'k6/browser';
import { check } from 'https://jslib.k6.io/k6-utils/1.5.0/index.js';

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
    await page.goto('https://test.k6.io/', { waitUntil: 'networkidle' });

    const contacts = page.locator('a[href="/contacts.php"]');
    await contacts.dispatchEvent("click");

    await check(page.locator('h3'), {
      'header': async lo => {
        const text = await lo.textContent();
        return text == 'Contact us';
      }
    });
  } finally {
    await page.close();
  }
}
