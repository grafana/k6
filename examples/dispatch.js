import { check } from 'k6';
import { browser } from 'k6/x/browser/async';

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

    const h3 = page.locator("h3");
    const ok = await h3.textContent() == "Contact us";
    check(ok, { "header": ok });
  } finally {
    await page.close();
  }
}
