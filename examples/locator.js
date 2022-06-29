import launcher from "k6/x/browser";

export default function () {
  const browser = launcher.launch('chromium', {
    headless: __ENV.XK6_HEADLESS ? true : false,
  });
  const context = browser.newContext();
  const page = context.newPage();
  page.goto("https://test.k6.io/flip_coin.php", {
    waitUntil: "networkidle",
  });

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

  // the tails locator clicks on the tails button by using the
  // locator's selector.
  tails.click();
  // Since clicking on each button causes page navigation,
  // waitForNavigation is needed. It's because the page
  // won't be ready until the navigation completes.
  page.waitForNavigation();
  console.log(currentBet.innerText());

  // the heads locator clicks on the heads button by using the
  // locator's selector.
  heads.click();
  page.waitForNavigation();
  console.log(currentBet.innerText());

  tails.click();
  page.waitForNavigation();
  console.log(currentBet.innerText());

  page.close();
  browser.close();
}
