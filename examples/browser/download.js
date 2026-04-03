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
  // Enable downloads and optionally set a custom download path.
  // If downloadsPath is not set, a temporary directory will be used.
  const context = await browser.newContext({
    acceptDownloads: true,
    downloadsPath: '/tmp/k6-downloads',
  });

  const page = await context.newPage();

  try {
    // Navigate to a page and interact with download links.
    await page.goto('https://quickpizza.grafana.com/test.k6.io/');
  } finally {
    await page.close();
  }
}
