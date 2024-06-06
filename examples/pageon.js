import { browser } from 'k6/x/browser/async';
import { check } from 'k6';

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

    page.on('console', async(msg) => {
        const jsonValue1 = await msg.args()[0].jsonValue();
        const jsonValue2 = await msg.args()[1].jsonValue();
        check(msg, {
            'assertConsoleMessageType': msg => msg.type() == 'log',
            'assertConsoleMessageText': msg => msg.text() == 'this is a console.log message 42',
            'assertConsoleMessageArgs0': msg => jsonValue1 == 'this is a console.log message',
            'assertConsoleMessageArgs1': msg => jsonValue2 == 42,
        });
    });

    await page.evaluate(() => console.log('this is a console.log message', 42));
  } finally {
    await page.close();
  }
}
