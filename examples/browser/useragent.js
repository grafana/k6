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
  let context = await browser.newContext({
    userAgent: 'k6 test user agent',
  })
  let page = await context.newPage();
  await check(page, {
    'user agent is set': async p => {
        const userAgent = await p.evaluate(() => navigator.userAgent);
        return userAgent.includes('k6 test user agent');
    }
  });
  await page.close();
  await context.close();

  context = await browser.newContext();
  check(context.browser(), {
    'user agent does not contain headless': b => {
        return b.userAgent().includes('Headless') === false;
    }
  });

  page = await context.newPage();
  await check(page, {
    'chromium user agent does not contain headless': async p => {
        const userAgent = await p.evaluate(() => navigator.userAgent);
        return userAgent.includes('Headless') === false;
    }
  });
  await page.close();
}
