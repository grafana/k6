import { chromium } from 'k6/x/browser';

export const options = {
  thresholds: {
    checks: ["rate==1.0"]
  }
}

export default function() {
  const browser = chromium.launch({
    headless: __ENV.XK6_HEADLESS ? true : false,
  });
  const context = browser.newContext();
  const page = context.newPage();
  page.goto("https://test.k6.io/flip_coin.php", {
    waitUntil: "networkidle",
  }).then(() => {
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
    Promise.all([
      page.waitForNavigation(),
      tails.click(),
    ]).then(() => {
      console.log(currentBet.innerText());
      // the heads locator clicks on the heads button
      // by using the locator's selector.
      return Promise.all([
        page.waitForNavigation(),
        heads.click(),
      ]);
    }).then(() => {
      console.log(currentBet.innerText());
      return Promise.all([
        page.waitForNavigation(),
        tails.click(),
      ]);
    }).finally(() => {
      console.log(currentBet.innerText());
      page.close();
      browser.close();
    })
  });
}
