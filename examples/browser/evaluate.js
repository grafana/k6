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
    await page.goto("https://test.k6.io/", { waitUntil: "load" });

    // calling evaluate without arguments
    await check(page, {
      'result should be 210': async p => {
        const n = await p.evaluate(() => 5 * 42);
        return n == 210;
      }
    });

    // calling evaluate with arguments
    await check(page, {
      'result should be 25': async p => {
        const n = await p.evaluate((x, y) => x * y, 5, 5);
        return n == 25;
      }
    });
  } finally {
    await page.close();
  }
}
