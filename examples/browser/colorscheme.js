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
  const context = await browser.newContext({
    // valid values are "light", "dark" or "no-preference"
    colorScheme: 'dark',
  });
  const page = await context.newPage();

  try {
    await page.goto(
      'https://test.k6.io',
      { waitUntil: 'load' },
    )
    await check(page, {
      'isDarkColorScheme':
        p => p.evaluate(() => window.matchMedia('(prefers-color-scheme: dark)').matches)
    });
  } finally {
    await page.close();
  }
}
