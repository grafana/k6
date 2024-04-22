import { browser } from 'k6/x/browser';

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
  }
}

export default async function () {
  const page = browser.newPage();

  await page.goto('https://test.k6.io/', { waitUntil: 'networkidle' });

  // Obtain ElementHandle for news link and navigate to it
  // by tapping in the 'a' element's bounding box
  const newsLinkBox = page.$('a[href="/news.php"]').boundingBox();
  await page.touchscreen.tap(newsLinkBox.x + newsLinkBox.width / 2, newsLinkBox.y);

  // Wait until the navigation is done before closing the page.
  // Otherwise, there will be a race condition between the page closing
  // and the navigation.
  await page.waitForNavigation({ waitUntil: 'networkidle' });

  page.close();
}
