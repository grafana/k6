import http from 'k6/http';
import exec from 'k6/http';
import { browser } from 'k6/browser';
import { sleep, check, fail } from 'k6';

const BASE_URL = 'file:///mnt/c/Users/Jan/Desktop/repos/k6/internal/js/modules/k6/browser/tests/static/hide_unhide.html';

export const options = {
  scenarios: {
    ui: {
      executor: 'shared-iterations',
      vus: 1,
      iterations: 1000,
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
  await page.goto(BASE_URL);
  
  const btn = await page.locator('#incBtn');
  const box = await btn.boundingBox();
  console.log(box.x);

  await page.waitForTimeout(500);
  await page.close();
}