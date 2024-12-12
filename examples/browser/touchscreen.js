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
  }
}

export default async function () {
  const page = await browser.newPage();

  await page.goto("https://test.k6.io/", { waitUntil: "networkidle" });

  // Obtain ElementHandle for news link and navigate to it
  // by tapping in the 'a' element's bounding box
  const newsLinkBox = await page.$('a[href="/news.php"]').then((e) => e.boundingBox());

  // Wait until the navigation is done before closing the page.
  // Otherwise, there will be a race condition between the page closing
  // and the navigation.
  await Promise.all([
    page.waitForNavigation(),
    page.touchscreen.tap(newsLinkBox.x + newsLinkBox.width / 2, newsLinkBox.y),
  ]);

  await page.close();
}
