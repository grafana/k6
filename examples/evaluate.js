import { check } from 'k6';
import { chromium } from 'k6/x/browser';

export default function() {
  const browser = chromium.launch({
    headless: __ENV.XK6_HEADLESS ? true : false,
  });
  const context = browser.newContext();
  const page = context.newPage();

  page.goto('https://test.k6.io/', { waitUntil: 'load' });
  const dimensions = page.evaluate(() => {
    const obj = {
      width: document.documentElement.clientWidth,
      height: document.documentElement.clientHeight,
      deviceScaleFactor: window.devicePixelRatio
    };
    console.log(obj); // tests #120
    return obj;
  });

  check(dimensions, {
    'width': d => d.width === 1265,
    'height': d => d.height === 720,
    'scale': d => d.deviceScaleFactor === 1,
  });

  page.close();
  browser.close();
}
