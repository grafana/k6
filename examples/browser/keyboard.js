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
  }
}

export default async function () {
  const page = await browser.newPage();

  try {
    const response = await page.goto('https://quickpizza.grafana.com/my_messages.php', { waitUntil: 'networkidle' });
    expect(response.status()).toBe(200);

    const userInput = page.locator('input[name="login"]');
    await userInput.click();
    await page.keyboard.type("admin");

    const pwdInput = page.locator('input[name="password"]');
    await pwdInput.click();
    await page.keyboard.type("123");

    await page.keyboard.press('Enter'); // submit
    await page.waitForNavigation();
  } finally {
    await page.close();
  }
}
