import { browser } from 'k6/browser';
import { expect } from "https://jslib.k6.io/k6-testing/0.5.0/index.js";

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
    this.headsButton = page.getByRole("button", { name: "Bet on heads!" });
    this.tailsButton = page.getByRole("button", { name: "Bet on tails!" });
    this.currentBet = page.getByText(/Your bet\: .*/);
  }

  async goto() {
    const response = await this.page.goto("https://quickpizza.grafana.com/flip_coin.php", { waitUntil: "networkidle" });
    expect(response.status()).toBe(200);
    return response;
  }

  heads() {
    return Promise.all([
      this.page.waitForNavigation(),
      this.headsButton.click(),
    ]);
  }

  tails() {
    return Promise.all([
      this.page.waitForNavigation(),
      this.tailsButton.click(),
    ]);
  }

  current() {
    return this.currentBet.innerText();
  }
}

export default async function() {
  const context = await browser.newContext();
  const page = await context.newPage();

  const bet = new Bet(page);
  try {
    await bet.goto()
    await bet.tails();
    console.log("Current bet:", await bet.current());
    await bet.heads();
    console.log("Current bet:", await bet.current());
    await bet.tails();
    console.log("Current bet:", await bet.current());
    await bet.heads();
    console.log("Current bet:", await bet.current());
  } finally {
    await page.close();
  }
}
