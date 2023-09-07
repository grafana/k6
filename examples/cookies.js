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
};

export default async function () {
  const page = browser.newPage();
  const context = page.context();

  try {
    // get cookies from the browser context
    check(context.cookies().length, {
        'initial number of cookies should be zero': n => n === 0,
    });

    // add some cookies to the browser context
    const unixTimeSinceEpoch = Math.round(new Date() / 1000);
    const day = 60*60*24;
    const dayAfter = unixTimeSinceEpoch+day;
    const dayBefore = unixTimeSinceEpoch-day;
    context.addCookies([
      // this cookie expires at the end of the session
      {
        name: 'testcookie',
        value: '1',
        sameSite: 'Strict',
        domain: '127.0.0.1',
        path: '/',
      },
      // this cookie expires in a day
      {
        name: 'testcookie2', 
        value: '2', 
        sameSite: 'Lax', 
        domain: '127.0.0.1', 
        path: '/', 
        expires: dayAfter,
      },
      // this cookie expires in the past, so it will be removed.
      {
        name: 'testcookie3',
        value: '3',
        sameSite: 'Lax',
        domain: '127.0.0.1',
        path: '/',
        expires: dayBefore
      }
    ]);

    check(context.cookies().length, {
      'number of cookies should be 2': n => n === 2,
    });

    const cookies = context.cookies();
    check(cookies[0], {
      'cookie 1 name should be testcookie': c => c.name === 'testcookie',
      'cookie 1 value should be 1': c => c.value === '1',
    });
    check(cookies[1], {
      'cookie 2 name should be testcookie2': c => c.name === 'testcookie2',
      'cookie 2 value should be 2': c => c.value === '2',
    });
  } finally {
    page.close();
  }
}
