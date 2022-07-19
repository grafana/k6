import { check } from 'k6';
import { chromium } from 'k6/x/browser';

export default function() {
  const browser = chromium.launch({
    headless: __ENV.XK6_HEADLESS ? true : false,
    args: ['host-resolver-rules=MAP test.k6.io 127.0.0.254'],
  });
  const context = browser.newContext();
  const page = context.newPage();

  const res = page.goto('http://test.k6.io/', { waitUntil: 'load' });

  check(res, {
    'null response': r => r === null,
  });

  page.close();
  browser.close();
}
