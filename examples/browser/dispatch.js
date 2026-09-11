import { browser } from 'k6/browser';
import { fail } from 'k6';
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
    await page.goto('https://quickpizza.grafana.com/test.k6.io/', { waitUntil: 'networkidle' });

    const contacts = page.getByRole('link', { name: '/contacts.php' });
    await contacts.dispatchEvent("click");

    await check(page.locator('h3'), {
      'header': async lo => {
        const text = await lo.textContent();
        return text == 'Contact us';
      }
    });
  } catch (error) {
    fail(`Browser iteration failed: ${error.message}`);
  } finally {
    await page.close();
  }
}
