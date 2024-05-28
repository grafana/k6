import { check } from 'k6';
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
  let el = await page.$('.visible');
  const isVisible = await el.isVisible();

  el = await page.$('.hidden');
  const isHidden = await el.isHidden();

  el = await page.$('.editable');
  const isEditable = await el.isEditable();

  el = await page.$('.enabled');
  const isEnabled = await el.isEnabled();

  el = await page.$('.disabled');
  const isDisabled = await el.isDisabled();

  el = await page.$('.checked');
  const isChecked = await el.isChecked();

  el = await page.$('.unchecked');
  const isUnchecked = await el.isChecked() === false;

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
