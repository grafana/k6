import { browser } from 'k6/browser';
import { check } from 'https://jslib.k6.io/k6-utils/1.5.0/index.js';

export const options = {
  scenarios:  { ui: { executor: 'shared-iterations', options: { browser: { type: 'chromium' }} } },
  thresholds: { checks: ["rate==1.0"] }
}

export default async function() {
  const page = await browser.newPage();

  await page.setContent(`
    <ul>
      <li>
        <h3>Product 1</h3>
        <button onclick="console.log('clicked 1')">Add to cart 1</button>
      </li>
      <li>
        <h3>Product 2</h3>
        <button onclick="console.log('clicked 2')">Add to cart 2</button>
      </li>
    </ul>`);

  // const el = await page.locator('internal:role=listitem', { hasText: 'Product 2' });
  // const el = await page.locator('internal:role=listitem', { hasText: /Produc. 2/ });
  const el = await page
    // .getByRole('listitem')
    // .getByRole("listitem >> internal:has-text='Product 2'")
    // .getByRole("listitem >> internal:has-text=/Produc. 2/")
    // .getByRole("listitem >> internal:has-not-text=Product 1")
    // .first()
    .filter({ hasText: 'Product 2' })
  // .filter({ hasText: 'Product 2' })
  // .filter({ hasText: /Product 2/ })

  // TODO: Also test filter chainability

  await check(el, { 'textContent': async e => {
    const txt = await e.textContent();
    return txt.includes(`Add to cart 2`);
  }});

  console.log(await el.textContent());

  await page.close();
}
