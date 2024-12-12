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
  // grant camera and microphone permissions to the
  // new browser context.
  const context = await browser.newContext({
    permissions: ["camera", "microphone"],
  });

  const page = await context.newPage();

  try {
    await page.goto('https://test.k6.io/');
    await page.screenshot({ path: `example-chromium.png` });
    await context.clearPermissions();
  } finally {
    await page.close();
  }
}
