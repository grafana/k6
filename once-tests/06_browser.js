import { browser } from 'k6/browser';
export const options = {
  scenarios: {
    ui: {
      executor: 'constant-vus',
      vus: 1,
      duration: '10s',
      exec: 'uiTest',
      options: { browser: { type: 'chromium' } },
    },
  },
};
export async function uiTest() {
  console.log('RUNNING UI_TEST');
  const page = await browser.newPage();
  await page.goto('https://test.k6.io/');
  await page.close();
}
