import { browser } from 'k6/browser';
import { check } from 'https://jslib.k6.io/k6-utils/1.5.0/index.js';

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
  const page = await browser.newPage();
  
  try {
    await page.goto('https://test.k6.io/');

    page.on('console', async msg => check(msg, {
      'assert console message type': msg =>
        msg.type() == 'log',
      'assert console message text': msg =>
        msg.text() == 'this is a console.log message 42',
      'assert console message first argument': async msg => {
        const arg1 = await msg.args()[0].jsonValue();
        return arg1 == 'this is a console.log message';
      },
      'assert console message second argument': async msg => {
        const arg2 = await msg.args()[1].jsonValue();
        return arg2 == 42;
      }
    }));

    await page.evaluate(() => console.log('this is a console.log message', 42));
  } finally {
    await page.close();
  }
}
