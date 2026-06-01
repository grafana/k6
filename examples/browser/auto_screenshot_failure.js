// auto_screenshot_failure.js triggers a browser API rejection partway
// through a script to exercise the auto-screenshot failure path added
// in the same feature. The script first loads a stable page, then
// attempts to click a selector that does not exist; the click rejects
// with a timeout and the failure-path capture should fire regardless
// of which mode is active.
//
// Run with auto-screenshot off, then with each mode:
//
//   ./k6 run examples/browser/auto_screenshot_failure.js
//   K6_BROWSER_AUTO_SCREENSHOT=actions ./k6 run examples/browser/auto_screenshot_failure.js
//   K6_BROWSER_AUTO_SCREENSHOT=changes ./k6 run examples/browser/auto_screenshot_failure.js
//
// Expected pattern:
//   - off:     0 screenshots
//   - actions: action-tagged shots from the successful calls, plus a
//              failure-tagged shot at the moment the bad click rejects.
//   - changes: change-tagged shots from the initial load, plus a
//              failure-tagged shot at the moment the bad click rejects.
//
// The script catches the rejection so the iteration completes cleanly
// and the harness can measure it; the capture has already been
// scheduled before the reject is forwarded to the JS runtime.
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

const fixtureHTML = `<!DOCTYPE html>
<html>
  <body>
    <h1>Failure fixture</h1>
    <button id="show-banner">Show banner</button>
    <div id="banner" style="display:none; padding:1em; background:#fee;">
      Working...
    </div>
    <script>
      document.getElementById('show-banner').addEventListener('click', () => {
        document.getElementById('banner').style.display = 'block';
      });
    </script>
  </body>
</html>`;

export default async function () {
  const context = await browser.newContext();
  const page = await context.newPage();

  try {
    await page.setContent(fixtureHTML);

    // First, perform a successful interaction so the page reaches a
    // distinct state. Without this the failure-path screenshot would
    // be byte-identical to the last successful action's screenshot
    // and the capturer's CRC32 dedup would silently skip it.
    await page.locator('#show-banner').click();

    // Now attempt a click on a selector that does not exist. The
    // promise rejects after the supplied timeout; the failure-path
    // capture fires before the rejection is forwarded to the runtime.
    try {
      await page.locator('#nonexistent-button').click({ timeout: 500 });
    } catch (err) {
      // Swallow so the iteration completes; the failure-path capture
      // has already been scheduled by the time we reach this point.
    }
  } finally {
    await page.close();
  }
}
