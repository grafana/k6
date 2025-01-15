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
    await page.goto('https://test.k6.io/?q=abc123');
    await page.goto('https://test.k6.io/?q=def456');
  } finally {
    await page.close();
  }
}
