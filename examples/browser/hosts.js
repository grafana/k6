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
  hosts: { 'test.k6.io': '127.0.0.254' },
  thresholds: {
    checks: ["rate==1.0"]
  }
};

export default async function() {
  const context = await browser.newContext();
  const page = await context.newPage();

  try {
    const res = await page.goto('http://test.k6.io/', {
      waitUntil: 'load'
    });
    await check(res, {
      'null response': r => r === null,
    });
  } finally {
    await page.close();
  }
}
