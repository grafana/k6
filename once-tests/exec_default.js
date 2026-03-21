import http from 'k6/http';
import { browser } from 'k6/browser';

export const options = {
  scenarios: {
    checkout: {
      executor: 'constant-vus',
      vus: 1,
      duration: '10s',
      exec: 'default',
      options: {
        browser: { type: 'chromium' },
      },
    },
  },
};

export default async function () {
  console.log('RUNNING DEFAULT');
  const page = await browser.newPage();
  await page.goto('https://test.k6.io/');
  await page.close();
}
