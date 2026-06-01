// auto_screenshot_spa.js exercises a single-page-app pattern: a static
// page whose visible state is driven entirely by client-side JavaScript
// (button clicks toggle modals, tabs, validation state). Use it to
// characterise auto-screenshot on SPA changes that do not fire any
// navigation or load lifecycle event.
//
// Run with auto-screenshot off and then on:
//
//   ./k6 run examples/browser/auto_screenshot_spa.js
//   K6_BROWSER_AUTO_SCREENSHOT=actions ./k6 run examples/browser/auto_screenshot_spa.js
//
// Expected pattern: one screenshot per click (each click is an API
// call). The CRC32 dedup ignores identical adjacent frames so visually
// stable interactions collapse.
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

const spaHTML = `<!DOCTYPE html>
<html>
  <head><title>SPA fixture</title></head>
  <body>
    <h1>SPA Fixture</h1>
    <button id="open-modal">Open modal</button>
    <button id="switch-tab">Switch tab</button>
    <button id="validate">Validate</button>
    <div id="modal" style="display:none; padding:1em; background:#eef;">Modal!</div>
    <div id="tab-content">Tab A content</div>
    <div id="validation"></div>
    <script>
      let tab = 'A';
      document.getElementById('open-modal').addEventListener('click', () => {
        const m = document.getElementById('modal');
        m.style.display = m.style.display === 'none' ? 'block' : 'none';
      });
      document.getElementById('switch-tab').addEventListener('click', () => {
        tab = tab === 'A' ? 'B' : 'A';
        document.getElementById('tab-content').textContent = 'Tab ' + tab + ' content';
      });
      document.getElementById('validate').addEventListener('click', () => {
        document.getElementById('validation').textContent = 'Please enter a value.';
        document.getElementById('validation').style.color = 'red';
      });
    </script>
  </body>
</html>`;

export default async function () {
  const context = await browser.newContext();
  const page = await context.newPage();

  try {
    await page.setContent(spaHTML);

    // Cycle through several visible-state changes that never trigger
    // a navigation or lifecycle event.
    await page.locator('#open-modal').click();
    await page.waitForTimeout(400);

    await page.locator('#switch-tab').click();
    await page.waitForTimeout(400);

    await page.locator('#validate').click();
    await page.waitForTimeout(400);

    await page.locator('#open-modal').click();
    await page.waitForTimeout(400);
  } finally {
    await page.close();
  }
}
