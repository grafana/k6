import { browser } from 'k6/browser';
import { expect } from "https://jslib.k6.io/k6-testing/0.5.0/index.js";

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
    const response = await page.goto('https://quickpizza.grafana.com/test.k6.io/');
    expect(response.status()).toBe(200);
    await page.screenshot({ path: `example-chromium.png` });
    await context.clearPermissions();
  } finally {
    await page.close();
  }
}
