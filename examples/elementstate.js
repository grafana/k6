import { check } from 'k6';
import { browser } from 'k6/x/browser/async';

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
  let isVisible = await page.$('.visible').then(e => e.isVisible());
  let isHidden = await page.$('.hidden').then(e => e.isHidden());
  let isEditable = await page.$('.editable').then(e => e.isEditable());
  let isEnabled = await page.$('.enabled').then(e => e.isEnabled());
  let isDisabled = await page.$('.disabled').then(e => e.isDisabled());
  let isChecked = await page.$('.checked').then(e => e.isChecked());
  let isUnchecked = !await page.$('.unchecked').then(e => e.isChecked());

  check(page, {
    'visible': isVisible,
    'hidden': isHidden,
    'editable': isEditable,
    'enabled': isEnabled,
    'disabled': isDisabled,
    'checked': isChecked,
    'unchecked': isUnchecked,
  });

  await page.close();
}
