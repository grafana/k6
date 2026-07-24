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
    checks: ['rate==1.0'],
  },
};

export default async function () {
  const page = await browser.newPage();

  try {
    await page.setContent(`
      <html>
        <body>
          <button id="alert-btn" onclick="alert('hello from k6 browser')">Show alert</button>
        </body>
      </html>
    `);

    page.on('dialog', async (dialog) => {
      check(dialog, {
        'dialog type is alert': (d) => d.type() === 'alert',
        'dialog message is correct': (d) => d.message() === 'hello from k6 browser',
      });
      await dialog.accept();
    });

    await page.locator('#alert-btn').click();
  } finally {
    await page.close();
  }
}
