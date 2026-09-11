import { browser } from 'k6/browser';
import { fail } from 'k6';

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
}

export default async function() {
  const page = await browser.newPage();

  page.on('metric', (metric) => {
    metric.tag({
      name:'test',
      matches: [
        {url: /^https:\/\/test\.k6\.io\/\?q=[0-9a-z]+$/, method: 'GET'},
      ]
    });
  });

  try {
    await page.goto('https://quickpizza.grafana.com/test.k6.io/?q=abc123');
    await page.goto('https://quickpizza.grafana.com/test.k6.io/?q=def456');
  } catch (error) {
    fail(`Browser iteration failed: ${error.message}`);
  } finally {
    await page.close();
  }
}
