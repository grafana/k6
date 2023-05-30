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
    await page.goto('https://test.k6.io/', { waitUntil: 'load' });
    
    const result = page.evaluate(([x, y]) => {
      return Promise.resolve(x * y);
    }, [5, 5]);
    console.log(result); // tests #120
    
    check(result, {
      'result is 25': (result) => result == 25,
    });
  } finally {
    page.close();
  }
}
