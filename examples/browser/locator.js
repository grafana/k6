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
  thresholds: {
    checks: ["rate==1.0"]
  }
}

export default async function() {
  const context = await browser.newContext();
  const page = await context.newPage();
  
  try {
    await page.goto("https://test.k6.io/flip_coin.php", {
      waitUntil: "networkidle",
    })

    /*
    In this example, we will use two locators, matching a
    different betting button on the page. If you were to query
    the buttons once and save them as below, you would see an
    error after the initial navigation. Try it!
  
      const heads = page.$("input[value='Bet on heads!']");
      const tails = page.$("input[value='Bet on tails!']");
  
    The Locator API allows you to get a fresh element handle each
    time you use one of the locator methods. And, you can carry a
    locator across frame navigations. Let's create two locators;
    each locates a button on the page.
    */
    const heads = page.locator("input[value='Bet on heads!']");
    const tails = page.locator("input[value='Bet on tails!']");

    const currentBet = page.locator("//p[starts-with(text(),'Your bet: ')]");

    // In the following Promise.all the tails locator clicks
    // on the tails button by using the locator's selector.
    // Since clicking on each button causes page navigation,
    // waitForNavigation is needed -- this is because the page
    // won't be ready until the navigation completes.
    // Setting up the waitForNavigation first before the click
    // is important to avoid race conditions.
    await Promise.all([
      page.waitForNavigation(),
      tails.click(),
    ]);
    console.log(await currentBet.innerText());
    // the heads locator clicks on the heads button
    // by using the locator's selector.
    await Promise.all([
      page.waitForNavigation(),
      heads.click(),
    ]);
    console.log(await currentBet.innerText());
    await Promise.all([
      page.waitForNavigation(),
      tails.click(),
    ]);
    console.log(await currentBet.innerText());
  } finally {
    await page.close();
  }
}
