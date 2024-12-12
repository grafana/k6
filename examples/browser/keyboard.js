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

  await page.goto('https://test.k6.io/my_messages.php', { waitUntil: 'networkidle' });

  const userInput = page.locator('input[name="login"]');
  await userInput.click();
  await page.keyboard.type("admin");

  const pwdInput = page.locator('input[name="password"]');
  await pwdInput.click();
  await page.keyboard.type("123");

  await page.keyboard.press('Enter'); // submit
  await page.waitForNavigation();

  await page.close();
}
