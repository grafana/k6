import { browser } from 'k6/browser';

export const options = {
  scenarios: {
    ui: {
      executor: 'shared-iterations',
      iterations: 5,
      options: {
        browser: {
          type: 'chromium',
        },
      },
    },
  },
};

const QUICKPIZZA_URL = __ENV.QUICKPIZZA_URL || 'http://localhost:3333';

export default async function () {
  const page = await browser.newPage();

  try {
    await page.goto(QUICKPIZZA_URL);
    await page.getByRole('button', { name: 'Pizza, Please!' }).click();
    await page.waitForTimeout(2000);
  } finally {
    await page.close();
  }
}
