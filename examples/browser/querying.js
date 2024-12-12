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
    await page.goto('https://test.k6.io/');

    await check(page, {
      'Title with CSS selector': async p => {
        const e = await p.$('header h1.title');
        return await e.textContent() === 'test.k6.io';
      },
      'Title with XPath selector': async p => {
        const e = await p.$('//header//h1[@class="title"]');
        return await e.textContent() === 'test.k6.io';
      }
    });
  } finally {
    await page.close();
  }
}
