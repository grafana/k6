import { browser } from 'k6/browser';
import { check } from 'k6';

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
    await page.route(
      'https://quickpizza.grafana.com/api/tools',
      function (route) {
        route.abort();
      }
    );

    await page.route(/(\.png$)|(\.jpg$)/, function (route) {
      route.abort();
    });

    page.on('request', function (request) {
      console.log('on request', 'url', request.url());
    });

    page.on('response', function (response) {
      console.log('on response', 'url', response.url());
    });

    // Test waitForResponse alongside page.on('response')
    const responsePromise = page.waitForResponse('https://quickpizza.grafana.com/');

    await page.goto('https://quickpizza.grafana.com/', {
      waitUntil: 'networkidle',
    });

    const response = await responsePromise;

    // Check that the main page loaded successfully
    check(response, {
      'main page status is 200': (r) => r.status() === 200,
      'main page URL is correct': (r) => r.url() === 'https://quickpizza.grafana.com/',
    });

    // Test waitForResponse with user interaction
    const pizzaResponsePromise = page.waitForResponse('https://quickpizza.grafana.com/api/pizza');

    await page.getByRole('button', { name: /pizza/i }).click();

    const pizzaResponse = await pizzaResponsePromise;

    // Check that the pizza API call was successful
    check(pizzaResponse, {
      'pizza API status is 200': (r) => r.status() === 200,
      'pizza API URL is correct': (r) => r.url() === 'https://quickpizza.grafana.com/api/pizza',
    });
  } finally {
    await page.close();
  }
}
