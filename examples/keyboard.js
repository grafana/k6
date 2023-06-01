import { browser } from 'k6/x/browser';

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
  const page = browser.newPage();

  await page.goto('https://test.k6.io/my_messages.php', { waitUntil: 'networkidle' });
    
  const userInput = page.locator('input[name="login"]');
  await userInput.click();
  page.keyboard.type('admin');
    
  const pwdInput = page.locator('input[name="password"]');
  await pwdInput.click();
  page.keyboard.type('123');

  page.keyboard.press('Enter'); // submit
    
  await page.close();
}
