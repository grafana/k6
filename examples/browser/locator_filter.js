import { browser } from 'k6/browser';
import { check } from 'https://jslib.k6.io/k6-utils/1.5.0/index.js';

export const options = {
  scenarios:  {
    ui: {
      executor: 'shared-iterations',
      options: {
        browser: {
          type: 'chromium'
        }
      }
    }
  },
  thresholds: {
    checks: ["rate==1.0"]
  }
}

export default async function() {
  const page = await browser.newPage();

  await page.setContent(`
    <ul>
      <li>
        <h3>Product 1</h3>
        <button>Add to cart 1</button>
      </li>
      <li>
        <h3>Product 2</h3>
        <button>Add to cart 2</button>
      </li>
    </ul>
  `);

  // Filter the 2nd item by filtering items with the "Product 2" text.
  await check(page, {
    'Filter the Add to cart 2 button': async p => {
      const txt = await p
        .getByRole('listitem')
        .filter({ hasText: 'Product 2' })
        .first()
        .textContent();
      return txt.includes(`Add to cart 2`);
    }
  });

  // Using a regex, filter the 1st item by filtering items without the "Product 2" text.
  await check(page, {
    'Filter the Add to cart 1 button': async p => {
      const txt = await p
        .getByRole('listitem')
        .filter({ hasNotText: /Product 2/ })
        .first()
        .textContent();
      return txt.includes(`Add to cart 1`);
    }
  });

  // Filtering with the locator.locator options instead of the locator.filter method.
  await check(page, {
    'Filter the Add to cart 1 button with locator options': async p => {
      const txt = await p
        .getByRole('list')
        .locator('li', { hasText: 'Product' })
        .locator('button', { hasText: /Add to cart 1/ })
        .first()
        .textContent();
      return txt.includes(`Add to cart 1`);
    }
  });

  await page.close();
}
