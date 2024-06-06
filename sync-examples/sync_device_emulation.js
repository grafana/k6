import { check, sleep } from 'k6';
import { browser, devices } from 'k6/x/browser';

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
  const device = devices['iPhone X'];
  // The spread operator is currently unsupported by k6's Babel, so use
  // Object.assign instead to merge browser context and device options.
  // See https://github.com/grafana/k6/issues/2296
  const options = Object.assign({ locale: 'es-ES' }, device);
  const context = browser.newContext(options);
  const page = context.newPage();

  try {
    await page.goto('https://k6.io/', { waitUntil: 'networkidle' });
    const dimensions = page.evaluate(() => {
      return {
        width: document.documentElement.clientWidth,
        height: document.documentElement.clientHeight,
        deviceScaleFactor: window.devicePixelRatio
      };
    });

    check(dimensions, {
      'width': d => d.width === device.viewport.width,
      'height': d => d.height === device.viewport.height,
      'scale': d => d.deviceScaleFactor === device.deviceScaleFactor,
    });

    if (!__ENV.K6_BROWSER_HEADLESS) {
      sleep(10);
    }
  } finally {
    page.close();
  }
}
