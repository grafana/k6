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
    await page.goto('https://test.k6.io/');
    check(page, {
      'Title with CSS selector':
        p => p.$('header h1.title').textContent() == 'test.k6.io',
      'Title with XPath selector':
        p => p.$(`//header//h1[@class="title"]`).textContent() == 'test.k6.io',
    });
  } finally {
    page.close();
  }
}
