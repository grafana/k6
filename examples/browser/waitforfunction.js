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
    await page.evaluate(() => {
      setTimeout(() => {
        const el = document.createElement('h1');
        el.innerHTML = 'Hello';
        document.body.appendChild(el);
      }, 1000);
    });

    const e = await page.waitForFunction(
      "document.querySelector('h1')", {
        polling: 'mutation',
        timeout: 2000
      }
    );
    await check(e, {
      'waitForFunction successfully resolved': async e => {
        return await e.innerHTML() === 'Hello';
      }
    });
  } finally {
    await page.close();
  }
}
