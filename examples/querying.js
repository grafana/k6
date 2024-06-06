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
    await page.goto('https://test.k6.io/');

    const titleWithCSS = await page.$('header h1.title').then(e => e.textContent());
    const titleWithXPath = await page.$(`//header//h1[@class="title"]`).then(e => e.textContent());

    check(page, {
      'Title with CSS selector': titleWithCSS == 'test.k6.io',
      'Title with XPath selector': titleWithXPath == 'test.k6.io',
    });
  } finally {
    await page.close();
  }
}
