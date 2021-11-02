import launcher from 'k6/x/browser';
import { check } from 'k6';

export default function() {
  const browser = launcher.launch('chromium', {
    // FIXME: colorScheme doesn't actually work... The page loads in light mode
    // regardless of the colorScheme value.
    // See https://github.com/grafana/xk6-browser/issues/46
    colorScheme: 'dark', // Valid values are "light", "dark" or "no-preference"
    headless: __ENV.XK6_HEADLESS ? true : false,
  });
  const context = browser.newContext();
  const page = context.newPage();
  page.goto('https://googlechromelabs.github.io/dark-mode-toggle/demo/', { waitUntil: 'load' });
  const el = page.$('#dark-mode-toggle-3');

  // FIXME: getAttribute() fails with:
  // unable to get node ID of element handle *dom.RequestNodeParams
  // See https://github.com/grafana/xk6-browser/issues/47
  // check(el, {
  //   'color scheme': e => e.getAttribute('mode') == 'dark',
  // });

  page.close();
  browser.close();
}
