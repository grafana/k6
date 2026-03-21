import { browser } from 'k6/browser';
export const options = {
  scenarios: {
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
