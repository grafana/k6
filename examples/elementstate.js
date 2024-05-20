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
  page.setContent(`
    <div class="visible">Hello world</div>
    <div style="display:none" class="hidden"></div>
    <div class="editable" editable>Edit me</div>
    <input type="checkbox" enabled class="enabled">
    <input type="checkbox" disabled class="disabled">
    <input type="checkbox" checked class="checked">
    <input type="checkbox" class="unchecked">
  `);

  // Check state
  const isVisible = await page.$('.visible').isVisible();
  const isHidden = await page.$('.hidden').isHidden();
  const isEditable = await page.$('.editable').isEditable();
  const isEnabled = await page.$('.enabled').isEnabled();
  const isDisabled = await page.$('.disabled').isDisabled();
  const isChecked = await page.$('.checked').isChecked();
  const isUnchecked = await page.$('.unchecked').isChecked() === false;
  check(page, {
    'visible': isVisible,
    'hidden': isHidden,
    'editable': isEditable,
    'enabled': isEnabled,
    'disabled': isDisabled,
    'checked': isChecked,
    'unchecked': isUnchecked,
  });

  page.close();
}
