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
    ui: {
      executor: 'constant-vus',
      vus: 1,
      duration: '10s',
      exec: 'uiTest',
      options: {
        browser: { type: 'chromium' },
      },
    },
  },
};
export function apiTest() {
  console.log('RUNNING API_TEST');
  http.get('https://test.k6.io/');
}
export async function uiTest() {
  const page = await browser.newPage();
  await page.goto('https://test.k6.io/');
  await page.close();
}
export default async function () {
  console.log('RUNNING DEFAULT');
  const page = await browser.newPage();
  await page.goto('https://test.k6.io/');
  await page.close();
}
