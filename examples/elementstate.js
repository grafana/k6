import { browser } from 'k6/x/browser/async';
import { check } from 'https://jslib.k6.io/k6-utils/1.5.0/index.js';

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

  // Inject page content
  await page.setContent(`
    <div class="visible">Hello world</div>
    <div style="display:none" class="hidden"></div>
    <div class="editable" editable>Edit me</div>
    <input type="checkbox" enabled class="enabled">
    <input type="checkbox" disabled class="disabled">
    <input type="checkbox" checked class="checked">
    <input type="checkbox" class="unchecked">
  `);

  // Check state
  await check(page, {
    'is visible':
      async p => p.$('.visible').then(e => e.isVisible()),
    'is hidden':
      async p => p.$('.hidden').then(e => e.isHidden()),
    'is editable':
      async p => p.$('.editable').then(e => e.isEditable()),
    'is enabled':
      async p => p.$('.enabled').then(e => e.isEnabled()),
    'is disabled':
      async p => p.$('.disabled').then(e => e.isDisabled()),
    'is checked':
      async p => p.$('.checked').then(e => e.isChecked()),
    'is unchecked':
      async p => p.$('.unchecked')
        .then(async e => await e.isChecked() === false),
  });

  // Change state and check again
  await check(page, {
    'is unchecked checked':
      async p => p.$(".unchecked")
        .then(e => e.setChecked(true))
        .then(() => p.$(".unchecked"))
        .then(e => e.isChecked()),
    'is checked unchecked':
      async p => p.$(".checked")
        .then(e => e.setChecked(false))
        .then(() => p.$(".checked"))
        .then(e => e.isChecked())
        .then(checked => !checked),
  });

  await page.close();
}
