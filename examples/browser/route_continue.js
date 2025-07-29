import { check, sleep } from 'k6';
import { browser } from 'k6/browser';

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
};

export default async function () {
  const page = await browser.newPage();

  try {
    await page.route(/.*\/api\/pizza/, function (route) {
      route.continue({
        postData: JSON.stringify({
          customName: 'My Pizza',
        }),
      });
    });

    await page.goto('https://quickpizza.grafana.com/');

    await page.getByRole('button', { name: 'pizza, please' }).click();
    sleep(1);

    const checkData = await page
      .locator('//p[text()[contains(., "Name:")]]')
      .innerText();
    check(page, {
      pizzaName: checkData === 'Name: My Pizza',
    });
  } finally {
    await page.close();
  }
}
