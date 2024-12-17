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
  const page = await browser.newPage();
  
  await page.setContent("<html><head><style></style></head><body>hello!</body></html>")

  await page.evaluate(() => {
    const shadowRoot = document.createElement('div');
    shadowRoot.id = 'shadow-root';
    shadowRoot.attachShadow({mode: 'open'});
    shadowRoot.shadowRoot.innerHTML = '<p id="shadow-dom">Shadow DOM</p>';
    document.body.appendChild(shadowRoot);
  });

  await check(page.locator('#shadow-dom'), {
    'shadow element exists': e => e !== null,
    'shadow element text is correct': async e => {
      return await e.innerText() === 'Shadow DOM';
    }
  });

  await page.close();
}
