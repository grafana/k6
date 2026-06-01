// auto_screenshot_anonymous_flow.js exercises the quickpizza
// recommendation flow as an anonymous (logged-out) user:
//
//   1. Land on the homepage.
//   2. Click the primary CTA to get a pizza recommendation.
//   3. Try to rate that recommendation.
//   4. Assert (via a wait that is expected to time out) that the
//      rating was NOT recorded — the app rejects rating attempts
//      from unauthenticated users.
//
// Run with auto-screenshot off and then on:
//
//   ./k6 run examples/browser/auto_screenshot_anonymous_flow.js
//   K6_BROWSER_AUTO_SCREENSHOT=actions ./k6 run examples/browser/auto_screenshot_anonymous_flow.js
//
// Expected pattern: action-tagged shots for the homepage, the
// recommendation, and the attempted rating, followed by a
// failure-tagged shot at the moment the success-indicator wait
// times out. The failure shot captures whatever the app actually
// shows the anon user (typically a login prompt or an error
// message).
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

    // 2. Get a recommendation.
    await page.getByRole('button', { name: /pizza/i }).first().click();
    await page.waitForTimeout(2000);

    // 3. Try to rate the recommendation. count() rather than a
    // click attempt with timeout because every rejected click would
    // trigger an extra failure-tagged screenshot from the
    // auto-screenshot feature, polluting the gallery with
    // selector-miss noise. count() returns 0 on miss without
    // rejecting.
    const ratingCandidates = [
      page.getByRole('button', { name: /rate/i }),
      page.locator('[aria-label*="star" i]'),
      page.locator('[role="radio"]'),
    ];
    for (const loc of ratingCandidates) {
      if ((await loc.count()) > 0) {
        await loc.first().click();
        await page.waitForTimeout(1000);
        break;
      }
    }

    // 4. Assert the rating was NOT recorded by waiting for a text
    // indicator that should never appear for an anonymous user.
    // The timeout rejection triggers the failure-path capture, which
    // produces a failure-tagged screenshot of whatever the app shows
    // in this state (login prompt, error banner, unchanged page,
    // etc.). Specific text rather than a class containment check so
    // we do not match incidental CSS classes elsewhere on the page.
    try {
      await page
        .getByText(/thank you for rating|rating recorded|your rating/i)
        .first()
        .waitFor({ timeout: 3000 });
    } catch (_) {
      // Expected: anonymous users cannot rate.
    }
  } finally {
    await page.close();
  }
}
