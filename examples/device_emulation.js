import { check, sleep } from 'k6';
import { chromium, devices } from 'k6/x/browser';

export default function() {
  const browser = chromium.launch({
    headless: __ENV.XK6_HEADLESS ? true : false,
  });

  const device = devices['iPhone X'];
  // The spread operator is currently unsupported by k6's Babel, so use
  // Object.assign instead to merge browser context and device options.
  // See https://github.com/grafana/k6/issues/2296
  const options = Object.assign({ locale: 'es-ES' }, device);
  const context = browser.newContext(options);
  const page = context.newPage();

  page.goto('https://k6.io/', { waitUntil: 'networkidle' });

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

  if (!__ENV.XK6_HEADLESS) {
    sleep(10);
  }

  page.close();
  browser.close();
}
