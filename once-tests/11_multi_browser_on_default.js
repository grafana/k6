import http from 'k6/http';
import { browser } from 'k6/browser';
export const options = {
  scenarios: {
    api: {
      executor: 'constant-vus',
      vus: 10,
      duration: '30s',
      exec: 'apiTest',
    },
    visual: {
      executor: 'constant-vus',
      vus: 1,
      duration: '10s',
      options: { browser: { type: 'chromium' } },
    },
  },
};
export function apiTest() {
  console.log('RUNNING API_TEST');
  http.get('https://test.k6.io/');
}
export default async function () {
  console.log('RUNNING DEFAULT');
  const page = await browser.newPage();
  await page.goto('https://test.k6.io/');
  await page.close();
}
