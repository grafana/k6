import { check } from 'k6';
import { chromium } from 'k6/x/browser';

export default function() {
  const browser = chromium.launch({
    headless: true,
  });
  const context = browser.newContext();
  const page = context.newPage();

  page.evaluate(() => {
    setTimeout(() => {
      const el = document.createElement('h1');
      el.innerHTML = 'Hello';
      document.body.appendChild(el);
    }, 1000);
  });

  page.waitForFunction("document.querySelector('h1')", {
    polling: 'mutation',
    timeout: 2000,
  }).then(ok => {
    check(ok, { 'waitForFunction successfully resolved': ok.innerHTML() == 'Hello' });
    page.close();
    browser.close();
  });
}
