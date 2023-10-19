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
  // grant camera and microphone permissions to the
  // new browser context.
  const context = browser.newContext({
    permissions: ["camera", "microphone"],
  });

  const page = context.newPage();

  try {
    await page.goto('https://test.k6.io/');
    page.screenshot({ path: `example-chromium.png` });
    context.clearPermissions();
  } finally {
    page.close();
  }
}
