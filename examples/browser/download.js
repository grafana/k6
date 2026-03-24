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
    await page.goto('https://example.com/download-page');

    // Start waiting for the download before clicking.
    const downloadPromise = page.waitForEvent('download');
    await page.click('#download-link');
    const download = await downloadPromise;

    console.log(`Download started: ${download.url()}`);
    console.log(`Suggested filename: ${download.suggestedFilename()}`);

    // Wait for the download to complete and get the path.
    const path = await download.path();
    console.log(`Downloaded to: ${path}`);

    // Alternatively, save the file to a specific location.
    await download.saveAs('/tmp/my-file.txt');

    // Check if the download failed.
    const failure = await download.failure();
    if (failure) {
      console.log(`Download failed: ${failure}`);
    }
  } finally {
    await page.close();
  }
}
