import launcher from "k6/x/browser";

/*
Page Object Model is a well-known pattern to abstract a web page.

The Locator API enables using the Page Object Model pattern to organize
and simplify test code.

Note: For comparison, you can see another example that does not use
the Page Object Model pattern in locator.js.
*/
export class Bet {
  constructor(page) {
    this.page = page;
    this.headsButton = page.locator("input[value='Bet on heads!']");
    this.tailsButton = page.locator("input[value='Bet on tails!']");
    this.currentBet = page.locator("//p[starts-with(text(),'Your bet: ')]");
  }

  goto() {
    this.page.goto("https://test.k6.io/flip_coin.php", { waitUntil: "networkidle" });
  }

  heads() {
    this.headsButton.click();
    this.page.waitForNavigation();
  }

  tails() {
    this.tailsButton.click();
    this.page.waitForNavigation();
  }

  current() {
    return this.currentBet.innerText();
  }
}

export default function () {
  const browser = launcher.launch('chromium', { headless: __ENV.XK6_HEADLESS ? true : false });
  const context = browser.newContext();
  const page = context.newPage();

  const bet = new Bet(page);
  bet.goto();

  bet.tails();
  console.log("Current bet:", bet.current());
  
  bet.heads();
  console.log("Current bet:", bet.current());

  bet.tails();
  console.log("Current bet:", bet.current());

  bet.heads();
  console.log("Current bet:", bet.current());

  page.close();
  browser.close();
}
