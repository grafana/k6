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
    await page.goto('https://googlechromelabs.github.io/dark-mode-toggle/demo/', {
      waitUntil: 'load',
    });
    await check(page, {
      "GetAttribute('mode')": async p => {
        const e = await p.$('#dark-mode-toggle-3');
        return await e.getAttribute('mode') === 'light';
      }
    });
  } finally {
    await page.close();
  }
}
