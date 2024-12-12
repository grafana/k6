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
  thresholds: {
    checks: ["rate==1.0"]
  }
}

export default async function() {
  const context = await browser.newContext();
  const page = await context.newPage();
  
  try {
    await page.goto('https://test.k6.io/');
    await page.screenshot({ path: 'screenshot.png' });
    // TODO: Assert this somehow. Upload as CI artifact or just an external `ls`?
    // Maybe even do a fuzzy image comparison against a preset known good screenshot?
  } finally {
    await page.close();
  }
}
