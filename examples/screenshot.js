import launcher from 'k6/x/browser';

export default function() {
  const browser = launcher.launch('chromium', {
    headless: __ENV.XK6_HEADLESS ? true : false,
  });
  const context = browser.newContext();
  const page = context.newPage();
  page.goto('https://test.k6.io/');
  page.screenshot({ path: 'screenshot.png' });
  // TODO: Assert this somehow. Upload as CI artifact or just an external `ls`?
  // Maybe even do a fuzzy image comparison against a preset known good screenshot?
  page.close();
  browser.close();
}
