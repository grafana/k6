import { browser } from 'k6/browser';

export const options = {
  scenarios: {
    browser: {
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

  try {
    // Wait for console event with predicate function shorthand
    const consolePromise = page.waitForEvent('console', (msg) => msg.text().includes('hello'));

    // Evaluate triggers console.log which will emit console events
    await page.evaluate(() => {
      console.log('some other message');
      console.log('hello from page');
    });

    // Only matches the message containing 'hello'
    const msg = await consolePromise;
    console.log(`Console message received: ${msg.text()}`);

    // Wait for response event with options object
    const responsePromise = page.waitForEvent('response', {
      predicate: (res) => res.url().includes('example.com'),
      timeout: 5000,
    });

    await page.goto('https://example.com');

    const response = await responsePromise;
    console.log(`Response received: ${response.url()}`);
  } finally {
    await page.close();
  }
}
