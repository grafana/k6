import { chromium } from 'k6/x/browser';

export default async function () {
  const browser = chromium.launch({
    headless: __ENV.XK6_HEADLESS ? true : false,
  });
  const page = browser.newPage();

  await page.goto('https://test.k6.io/', { waitUntil: 'networkidle' });
  
  // Obtain ElementHandle for news link and navigate to it
  // by tapping in the 'a' element's bounding box
  const newsLinkBox = page.$('a[href="/news.php"]').boundingBox();
  page.touchscreen.tap(newsLinkBox.x + newsLinkBox.width / 2, newsLinkBox.y);
    
  await page.close();
  await browser.close();
}
