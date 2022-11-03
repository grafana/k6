import { check } from 'k6';
import { chromium } from 'k6/x/browser';

export const options = {
  thresholds: {
    checks: ["rate==1.0"]
  }
}

export default function() {
  const browser = chromium.launch({
    headless: __ENV.XK6_HEADLESS ? true : false,
    args: ['host-resolver-rules=MAP test.k6.io 127.0.0.1'],
  });
  const context = browser.newContext();
  const page = context.newPage();

  page.goto('http://test.k6.io/', { waitUntil: 'load' }).then((res) => {
    check(res, {
      'null response': r => r === null,
    });
  }).finally(() => {
    page.close();
    browser.close();
  });
}
