import launcher from 'k6/x/browser';
import { check } from 'k6';

export default function() {
  const browser = launcher.launch('chromium', {
    headless: __ENV.XK6_HEADLESS ? true : false,
  });
  const context = browser.newContext();
  const page = context.newPage();

  page.goto('https://test.k6.io/', { waitUntil: 'load' });
  const dimensions = page.evaluate(() => {
    return {
      width: document.documentElement.clientWidth,
      height: document.documentElement.clientHeight,
      deviceScaleFactor: window.devicePixelRatio
    };
  });

  check(dimensions, {
    'width': d => d.width === 1265,
    'height': d => d.height === 720,
    'scale': d => d.deviceScaleFactor === 1,
  });

  page.close();
  browser.close();
}
