import { browser } from 'k6/browser';
import { instrumentBrowser, uninstrumentBrowser } from './browser-tracing.js';

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
    await instrumentBrowser(page, {
      propagator: 'w3c',
      sampling: 1.0,
    });

    await page.goto(QUICKPIZZA_URL);
    await page.getByRole('button', { name: 'Pizza, Please!' }).click();
    await page.waitForTimeout(2000);

    await uninstrumentBrowser(page);
  } finally {
    await page.close();
  }
}
