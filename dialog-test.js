import { browser } from 'k6/browser';
import { check } from 'k6';

export const options = {
  scenarios: {
    browser: {
      executor: 'shared-iterations',
      options: { browser: { type: 'chromium' } },
    },
  },
};

const pageURL = `file://${__ENV.HTML_PATH}`;

export default async function () {
  const page = await browser.newPage();

  // A mutable ref that the dialog handler reads at call time.
  let dialogAction = 'accept';

  let dialogPage = null;
  page.on('dialog', async (dialog) => {
    console.log(`[dialog] action=${dialogAction}`);
    dialogPage = dialog.page();
    if (dialogAction === 'accept') {
      await dialog.accept();
    } else {
      await dialog.dismiss();
    }
  });

  try {
    await page.goto(pageURL);

    // ── Test 1: alert – accept ────────────────────────────────────────────
    dialogAction = 'accept';
    await page.locator('#alert-btn').click();
    await page.waitForFunction(() => document.getElementById('result').textContent !== 'waiting...');
    const r1 = await page.locator('#result').textContent();
    check(null, { 'T1 alert accept → "alert:accepted"': () => r1 === 'alert:accepted' });
    console.log(`T1: ${r1}`);

    // Reset
    await page.evaluate(() => { document.getElementById('result').textContent = 'waiting...'; });

    // ── Test 2: confirm – dismiss ─────────────────────────────────────────
    dialogAction = 'dismiss';
    await page.locator('#confirm-btn').click();
    await page.waitForFunction(() => document.getElementById('result').textContent !== 'waiting...');
    const r2 = await page.locator('#result').textContent();
    check(null, { 'T2 confirm dismiss → "confirm:dismissed"': () => r2 === 'confirm:dismissed' });
    console.log(`T2: ${r2}`);

    await page.evaluate(() => { document.getElementById('result').textContent = 'waiting...'; });

    // ── Test 3: confirm – accept ──────────────────────────────────────────
    dialogAction = 'accept';
    await page.locator('#confirm-btn').click();
    await page.waitForFunction(() => document.getElementById('result').textContent !== 'waiting...');
    const r3 = await page.locator('#result').textContent();
    check(null, { 'T3 confirm accept → "confirm:accepted"': () => r3 === 'confirm:accepted' });
    console.log(`T3: ${r3}`);

    // ── Test 4: dialog.page() returns the owning page ─────────────────────
    check(null, { 'T4 dialog.page() is not null': () => dialogPage !== null });
    console.log(`T4: dialogPage=${dialogPage !== null ? 'ok' : 'null'}`);

  } finally {
    await page.close();
  }
}
