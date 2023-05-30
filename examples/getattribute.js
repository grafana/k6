import { check } from 'k6';
import { browser } from 'k6/x/browser';

export const options = {
  scenarios: {
    ui: {
      executor: 'shared-iterations',
      options: {
        browser: {
            type: 'chromium',
        },
      },
    },
  },
  thresholds: {
    checks: ["rate==1.0"]
  }
}

export default async function() {
  const context = browser.newContext();
  const page = context.newPage();

  try {
    await page.goto('https://googlechromelabs.github.io/dark-mode-toggle/demo/', {
      waitUntil: 'load',
    });
    let el = page.$('#dark-mode-toggle-3')
    check(el, {
      "GetAttribute('mode')": e => e.getAttribute('mode') == 'light',
    });
  } finally {
    page.close();
  }
}
