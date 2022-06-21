import { check } from 'k6';
import launcher from 'k6/x/browser';

export default function() {
  const browser = launcher.launch('chromium', {
    headless: __ENV.XK6_HEADLESS ? true : false,
  });
  const context = browser.newContext();
  const page = context.newPage();

  // Goto front page, find login link and click it
  page.goto('https://test.k6.io/', { waitUntil: 'networkidle' });
  const elem = page.$('a[href="/my_messages.php"]');
  elem.click();

  // Enter login credentials and login
  page.$('input[name="login"]').type('admin');
  page.$('input[name="password"]').type('123');
  page.$('input[type="submit"]').click();

  // We expect the above form submission to trigger a navigation, so wait for it
  // and the page to be loaded.
  page.waitForNavigation();

  check(page, {
    'header': page.$('h2').textContent() == 'Welcome, admin!',
  });

  page.close();
  browser.close();
}
