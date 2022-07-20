import { check } from 'k6';
import { chromium } from 'k6/x/browser';

export default function() {
  const browser = chromium.launch({
    headless: __ENV.XK6_HEADLESS ? true : false,
  });
  const context = browser.newContext();
  const page = context.newPage();

  // Goto front page, find login link and click it
  page.goto('https://test.k6.io/', { waitUntil: 'networkidle' });
  const elem = page.$('a[href="/my_messages.php"]');
  elem.click().then(() => {
    // Enter login credentials and login
    page.$('input[name="login"]').type('admin');
    page.$('input[name="password"]').type('123');
    return page.$('input[type="submit"]').click();
  }).then(() => {
    // We expect the above form submission to trigger a navigation, so wait for it
    // and the page to be loaded.
    page.waitForNavigation();

    check(page, {
      'header': page.$('h2').textContent() == 'Welcome, admin!',
    });
  }).finally(() => {
    page.close();
    browser.close();
  });
}
