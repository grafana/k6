import { check } from 'k6';
import { browser } from 'k6/x/browser/async';

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
  const preferredColorScheme = 'dark';

  const context = await browser.newContext({
    // valid values are "light", "dark" or "no-preference"
    colorScheme: preferredColorScheme,
  });
  const page = await context.newPage();

  try {
    await page.goto(
      'https://googlechromelabs.github.io/dark-mode-toggle/demo/',
      { waitUntil: 'load' },
    )
    const colorScheme = await page.evaluate(() => {
      return {
        isDarkColorScheme: window.matchMedia('(prefers-color-scheme: dark)').matches
      };
    });
    check(colorScheme, {
      'isDarkColorScheme': cs => cs.isDarkColorScheme
    });
  } finally {
    await page.close();
  }
}
