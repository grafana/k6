import { browser } from 'k6/x/browser';
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
  const page = browser.newPage();
  
  try {
    await page.goto('https://test.k6.io/');

    page.on('console', msg => {
        check(msg, {
            'assertConsoleMessageType': msg => msg.type() == 'log',
            'assertConsoleMessageText': msg => msg.text() == 'this is a console.log message 42',
            'assertConsoleMessageArgs0': msg => msg.args()[0].jsonValue() == 'this is a console.log message',
            'assertConsoleMessageArgs1': msg => msg.args()[1].jsonValue() == 42,
        });
    });

    page.evaluate(() => console.log('this is a console.log message', 42));
  } finally {
    page.close();
  }
}
