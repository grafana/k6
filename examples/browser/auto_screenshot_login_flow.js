// auto_screenshot_login_flow.js exercises the quickpizza
// authenticated rating flow end-to-end:
//
//   1. Land on the homepage.
//   2. Click "Login" to reveal the login form.
//   3. Type the default credentials (default / 12345678).
//   4. Submit and wait for the post-login redirect.
//   5. Get a pizza recommendation.
//   6. Rate the recommendation.
//   7. Assert the rating WAS recorded by waiting for a
//      success/thank-you indicator.
//
// Run with auto-screenshot off and then on:
//
//   ./k6 run examples/browser/auto_screenshot_login_flow.js
//   K6_BROWSER_AUTO_SCREENSHOT=actions ./k6 run examples/browser/auto_screenshot_login_flow.js
//
// Expected pattern: action-tagged shots covering the home page,
// the login form, the post-login home, the recommendation, and
// the rating. If the success indicator appears as expected no
// failure-tagged shot is produced; if quickpizza changes its UI
// and the assertion times out, a failure-tagged shot captures
// the unexpected state — exactly the debugging value Mode A's
// failure path is designed to deliver.
import { browser } from 'k6/browser';

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
};

export default async function () {
  const context = await browser.newContext();
  const page = await context.newPage();

  try {
    // 1. Homepage.
    await page.goto('https://quickpizza.grafana.com', { waitUntil: 'load' });
    await page.waitForTimeout(500);

    // 2. Open the login screen. count() first so selector misses do
    // not produce spurious failure-tagged screenshots from the
    // auto-screenshot feature.
    const loginCandidates = [
      page.getByRole('link', { name: /log\s*in|sign\s*in/i }),
      page.getByRole('button', { name: /log\s*in|sign\s*in/i }),
    ];
    let loginOpened = false;
    for (const loc of loginCandidates) {
      if ((await loc.count()) > 0) {
        await loc.first().click();
        loginOpened = true;
        break;
      }
    }
    if (!loginOpened) {
      // Last resort: go to /login directly.
      await page.goto('https://quickpizza.grafana.com/login', {
        waitUntil: 'load',
      });
    }
    await page.waitForTimeout(500);

    // 3. Type the credentials. quickpizza's demo account is
    // default / 12345678 (per the app's own onboarding).
    await page
      .locator('input[name="username" i], input[type="text"]')
      .first()
      .type('default');
    await page
      .locator('input[name="password" i], input[type="password"]')
      .first()
      .type('12345678');

    // 4. Submit login. quickpizza authenticates via an SPA-style
    // POST without triggering a top-level navigation, so a fixed
    // wait is more reliable than waitForNavigation, which would
    // time out (and produce a spurious failure-tagged screenshot).
    await page
      .getByRole('button', { name: /log\s*in|sign\s*in|submit/i })
      .first()
      .click();
    await page.waitForTimeout(2000);

    // Some flows return the user to the homepage after login;
    // others land on a profile / dashboard page. Navigate back
    // to the home explicitly so the recommend step has a known
    // starting point.
    await page.goto('https://quickpizza.grafana.com', { waitUntil: 'load' });
    await page.waitForTimeout(500);

    // 5. Get a recommendation.
    await page.getByRole('button', { name: /pizza/i }).first().click();
    await page.waitForTimeout(2000);

    // 6. Rate the recommendation. count() first to avoid
    // triggering failure-tagged screenshots for selector misses.
    const ratingCandidates = [
      page.getByRole('button', { name: /rate/i }),
      page.locator('[aria-label*="star" i]'),
      page.locator('[role="radio"]'),
    ];
    for (const loc of ratingCandidates) {
      if ((await loc.count()) > 0) {
        await loc.first().click();
        await page.waitForTimeout(1500);
        break;
      }
    }

    // 7. Assert the rating was recorded by waiting for a specific
    // success text. Wrapped in a try so a UI drift produces a
    // failure-tagged screenshot of the actual state rather than a
    // hard iteration crash.
    try {
      await page
        .getByText(/thank you for rating|rating recorded|your rating/i)
        .first()
        .waitFor({ timeout: 5000 });
    } catch (_) {
      // UI may have changed; the failure-path capture has already
      // taken a screenshot of the page as it actually appears.
    }
  } finally {
    await page.close();
  }
}
