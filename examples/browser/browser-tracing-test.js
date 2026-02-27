import { browser } from 'k6/browser';
import { instrumentBrowser, uninstrumentBrowser } from './browser-tracing.js';

export const options = {
  scenarios: {
    user: {
      exec: 'userLogin',
      executor: 'shared-iterations',
      options: {
        browser: {
          type: 'chromium',
        },
      },
    },
  },
};

const QUICKPIZZA_URL = __ENV.QUICKPIZZA_URL || 'http://localhost:3333';

export async function userLogin() {
  const page = await browser.newPage();

  try {
    await instrumentBrowser(page, {
      propagator: 'w3c',
      sampling: 1.0,
    });

    await page.goto(QUICKPIZZA_URL + '/login', {
      waitUntil: 'networkidle',
    });

    await page.getByLabel('username').pressSequentially('default');
    await page.getByLabel('password').fill('12345678');
    await page.getByText('Sign in').click();

    await page.getByRole('button', { name: 'Logout' }).waitFor();

    await Promise.all([
      page.waitForNavigation(),
      page.getByRole("link", { name: "Back to main page", exact: true }).click(),
    ]);

    await page
      .getByRole("button", { name: "Pizza, Please!", exact: true })
      .click();

    const ratingsResponse = page.waitForResponse(/\/api\/ratings/);
    await page.getByRole("button", { name: "Love it!", exact: true }).click();

    await ratingsResponse;

    await uninstrumentBrowser(page);
  } finally {
    await page.close();
  }
}
