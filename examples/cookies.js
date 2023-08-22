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
    context.addCookies([{name: 'testcookie', value: '1', sameSite: 'Strict', domain: '127.0.0.1', path: '/'}]);
    context.addCookies([{name: 'testcookie2', value: '2', sameSite: 'Lax', domain: '127.0.0.1', path: '/'}]);

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