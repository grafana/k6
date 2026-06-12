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
    await page.goto('https://www.selenium.dev/selenium/web/alerts.html');

    page.on('dialog', async (dialog) => {
      check(dialog, {
        'dialog type is alert': (d) => d.type() === 'alert',
        'dialog has message': (d) => d.message().length > 0,
      });
      await dialog.accept();
    });

    await page.locator('#alert').click();
  } finally {
    await page.close();
  }
}
