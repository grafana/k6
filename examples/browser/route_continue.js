import { sleep } from 'k6';
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

    const e = page.getByText('Name:')
    check(null, {
      pizzaName: await e.innerText() === 'Name: My Pizza',
    });
  } finally {
    await page.close();
  }
}
