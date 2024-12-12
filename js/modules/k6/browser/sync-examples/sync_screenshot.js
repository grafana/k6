import { browser } from 'k6/x/browser';

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
  const context = browser.newContext();
  const page = context.newPage();
  
  try {
    await page.goto('https://test.k6.io/');
    page.screenshot({ path: 'screenshot.png' });
    // TODO: Assert this somehow. Upload as CI artifact or just an external `ls`?
    // Maybe even do a fuzzy image comparison against a preset known good screenshot?
  } finally {
    page.close();
  }
}
